---
type: Process
title: Automated Security Auditing & Jules Remediation Workflow
description: Proactive continuous security auditing with Google Jules and the isolated worktree review and merge workflow.
generated: { by: agent/mcp, at: 2026-09-08T10:27:02Z }
---

# Automated Security Auditing & Jules Remediation Workflow

## Context & Objectives

To maintain the highest level of defensive assurance for `okf-agent-memory`, automated security auditing is integrated as a proactive, continuous discovery loop. **Google Jules** runs as an autonomous adversarial background agent performing daily repository audits, complemented by deterministic local review and integration workflows.

---

## 1. Continuous Audit Protocol (Google Jules)

Google Jules acts as an Adversarial Security Specialist targeting `pkg/okf` and `cmd/okf`:

* **Target Branch**: Jules must branch off and open Pull Requests strictly against `develop`.
* **Focus Areas**:
  - Path traversal and directory boundary containment (CWE-22) in `bundle.go` and `mutate.go`.
  - Symlink resolution and escape containment (CWE-59).
  - MCP tool call argument sanitization and server root confinement (`cmd/okf/mcp.go`).
  - Resource limits, parsing edge cases, and frontmatter smuggling.
* **Adversarial Regression Tests**: Every finding must include minimal reproducible test cases in `pkg/okf/mutate_security_test.go` or `cmd/okf/mcp_test.go`.

---

## 2. Daily Integration & Verification Pipeline

When Jules opens one or more remediation branches (`security-audit-remediation-<id>`):

### Step 1: Discover Pending Branches
```bash
make jules-list
```
Lists all open Jules remediation branches on `origin`.

### Step 2: Isolated Verification & Review
```bash
make jules-review
# or for a specific branch:
make jules-review BRANCH=<branch-name>
```
* **Git Worktree Isolation**: Spawns an isolated temporary Git worktree for the target branch without disrupting the developer's working directory.
* **Automated Pipeline Gate**: Runs `make check` (formatting, vet, lint, unit/integration tests, and strict OKF bundle validations).
* **Multi-Branch Awareness**: If multiple Jules branches are pending, reports all of them and selects the latest by default.

### Step 3: Adversarial Review & False-Positive Gating
Review the diff against `develop` for subtle agentic hallucinations or false positives:
* **False Positives on Concept Names**: Ensure root-level protections (e.g. `AGENTS.md`) do not inadvertently reject legitimate nested concepts in subdirectories (such as `architecture/agents.md`).
* **Clean Code & Permissions**: Verify permissions remain `0o644` for files and `0o755` for directories.

### Step 4: Deterministic Merge & Remote Cleanup
```bash
make jules-merge
# or for a specific branch:
make jules-merge BRANCH=<branch-name>

# Push to origin and remove remote branch:
git push origin develop
git push origin --delete <jules-branch-name>
```

---

# Related Concepts
- [Bundle Isolation and Mutation Security Boundaries](../architecture/security-boundaries.md): Core defensive security boundaries audited by Jules
- [MCP Tool Security & Untrusted Agent Input](mcp-agent-safety.md): Sanitization and confinement requirements for agent MCP interfaces
- [Contributor Guidelines & PR Standards](contributing.md): General quality gates and verification standards for pull requests
