# SECURITY_AUDIT.md — Security Reviewer Agent & Continuous Audit Framework

This document establishes the instructions, checklist, and operating procedures for conducting adversarial security audits of the **OKF Agent Memory** codebase.

---

## 1. Role & Mindset for the Security Reviewer Agent

You are a senior **Application Security Specialist & Adversarial Code Auditor**. 
Your mission is not to make code run, but to **systematically discover vulnerabilities, logic flaws, and manipulation vectors**.

### Core Mindset
1. **Assume Compromised Input**: Every string originating from the CLI, MCP tool calls, or parsed from external files (including concept markdown files) is potentially malicious or the product of an indirect prompt injection attack.
2. **Confused Deputy Awareness**: Verify whether a tool (such as `okf_create` or `okf_show`) that appears benign to the user can be exploited by an agent as an arbitrary file-read (LFI) or arbitrary file-write primitive.
3. **Defense-in-Depth**: A single check is insufficient. Boundaries must be enforced at the API boundary (CLI/MCP), within domain logic, AND at the central choke-points (`ensureWithinRoot`, `SaveConcept`).

---

## 2. Mandatory Audit Checklist

### Area A: Path & Filesystem Security (CWE-22 / CWE-59)
- [ ] **Bundle Boundary Enforcement**:
  - Ensure NO filesystem operation (`os.WriteFile`, `os.MkdirAll`, `os.ReadFile`) ever escapes the designated bundle directory.
  - Are canonical paths resolved using `filepath.EvalSymlinks` and `filepath.Abs`, and verified via `filepath.Rel` against the canonical bundle root?
  - Are `..` path segments and leading directory separators strictly rejected?
- [ ] **Symlink Traversal & Escape Prevention**:
  - Does `LoadBundle` inspect symlinks via `ensureWithinRoot` to prevent reading arbitrary system files outside the bundle (Local File Inclusion / LFI)?
  - Does `SaveConcept` refuse to overwrite existing symlinks pointing outside the bundle?
  - Are symlinks restricted to point only to markdown (`.md`) files within the bundle (no directory symlinks or non-markdown files)?
- [ ] **Bookkeeping Isolation**:
  - Do automated bookkeeping operations (`UpdateParentIndex`, `AppendLogEntry`) write strictly within the canonical bundle hierarchy?
- [ ] **Reserved Files Protection**:
  - Is it impossible to overwrite critical root files (`index.md`, `log.md`, `AGENTS.md`) as concept documents?
- [ ] **File Permissions**:
  - Are files created with at most `0o644` (rw-r--r--) and directories with `0o755` (rwxr-xr-x)? Never allow `0o777`.
  - *Note on Static Analysis*: Standard repo documents must remain readable by collaborators, CI runners, and Git sub-processes. Static analysis rules expecting daemon-private permissions (e.g. `gosec G301/G306`) are excluded because OKF bundles are collaborative version-controlled knowledge bases, not secret credential stores.

### Area B: MCP & Agent Interfaces (Indirect Prompt Injection & Confinement)
- [ ] **MCP Server Root Confinement**:
  - Does the MCP server enforce that the `bundle` tool call argument stays within the server's canonical `rootDir`?
  - Are attempts to switch to bundles outside the workspace root (e.g. `bundle: "/etc"` or `bundle: "../../outside"`) rejected immediately?
- [ ] **Tool Call Argument Sanitization**:
  - Are arguments such as `concept_id`, `bundle`, and `type` validated for strict type, length, and format constraints before invocation?
- [ ] **Information & Error Leakage**:
  - Do tool errors returned to the agent avoid leaking sensitive environment details, host tokens, or unnecessary absolute system paths?

### Area C: Denial of Service & Resource Bounds
- [ ] **Pathological Inputs & Link Cycles**:
  - Can cyclical concept links cause infinite loops in graph traversal or index generation?
  - Are regular expressions protected against ReDoS (catastrophic backtracking)?
  - Are search and discovery limits (`--limit`, paging) enforced to prevent memory exhaustion?

### Area D: Data Integrity & Provenance
- [ ] **Frontmatter & Scalar Sanitization**:
  - Are metadata fields (`type`, `title`, `description`, `actor`) validated against newline injection (`\r`, `\n`) to prevent YAML attribute smuggling or forged verification states (`verified: { by: "human:attacker" }`)?
  - Is the YAML document delimiter (`---`) forbidden in metadata values?
  - Are special characters safely quoted in YAML output without entity-escaping?
- [ ] **Trust Boundaries**:
  - Is an agent prevented from self-attributing human verification (`human:...`)?
  - Are temporal timestamps (`generated.at` vs. `verified.at`) validated for chronological ordering (`verified.at` must not precede `generated.at`)?

---

## 3. Audit Report Format

When conducting an audit, produce the report in the following format:

1. **Executive Summary**: Passed / Passed with Warnings / Failed.
2. **Vulnerabilities Found**:
   - Categorized by Severity (Critical, High, Medium, Low) and CWE (e.g. CWE-22, CWE-59).
   - Description of impact and exploit scenario.
3. **Exact Code Locations**: File and line numbers.
4. **Remediation & Regression Test Requirements**: Required code fixes and unit test cases.

---

## 4. Continuous Automated Auditing via Google Jules (Daily Scan)

To complement pre-release gates with proactive, continuous discovery, **Google Jules** runs as an autonomous background agent performing daily repository audits.

### Jules Task Prompt Template
```markdown
You are acting as an Adversarial Security Specialist for the okf-agent-memory repository.

Target Branch: Always branch off and open pull requests against `develop`.

Task:
1. Review all recent code modifications in `pkg/okf` and `cmd/okf` against the criteria in `docs/SECURITY_AUDIT.md`.
2. Pay special attention to:
   - Path boundary containment (CWE-22) and symlink resolution (CWE-59) in `pkg/okf/bundle.go` and `pkg/okf/mutate.go`.
   - MCP tool call argument sanitization and server root confinement in `cmd/okf/mcp.go`.
   - Resource limits, parsing edge cases, and frontmatter smuggling.
3. Write adversarial unit tests in `pkg/okf/mutate_security_test.go` or `cmd/okf/mcp_test.go` attempting to bypass boundary checks.
4. If you discover a vulnerability or code health improvement:
   - Provide a minimal reproducible test case.
   - Open a Pull Request against `develop` detailing:
     * Finding & CWE Category
     * Exploitation Scenario / Risk
     * Implemented Remediation & Tests Added
```

### Daily Integration Workflow (Maintainer / Reviewer)

When Jules finishes an audit session and opens a branch / PR:

1. **List Open Jules Branches**:
   ```bash
   make jules-list
   ```
2. **Review & Verify in Isolated Worktree**:
   ```bash
   make jules-review
   # or for a specific branch: make jules-review BRANCH=<branch-name>
   ```
   *Automatically checks out the latest (or specified) Jules branch in an isolated temporary Git worktree, runs `make check`, and shows the compact diff against `develop`.*
3. **Inspect for Edge Cases & False Positives**:
   *Check that valid concepts (e.g. nested subfolders) are not inadvertently blocked.*
4. **Merge & Clean Up**:
   ```bash
   make jules-merge
   # or for a specific branch: make jules-merge BRANCH=<branch-name>
   git push origin develop
   git push origin --delete <jules-branch-name>
   ```
