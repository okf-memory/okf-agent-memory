# AGENTS.md - DMAA Protocol v0.1 - Instructions for AI Agents in `okf-agent-memory`

> Powered by [OKF Agent Memory](https://github.com/okf-memory/okf-agent-memory) -- Open Knowledge Format (OKF) v0.3 persistent project memory for AI agents.

## 0. Project & Domain Codex
- TONE: style == direct_concise, zero_pleasantries
- FORMAT: diagrams => ASSERT(syntax == mermaid, ELSE=STOP("All diagrams MUST use Mermaid syntax; ASCII/text art prohibited."))
- GOAL: maintain(high_factual_integrity, domain_neutrality, strict_determinism)
- COMMITS: maintainer signs + executes ALL commits. Agent => `git add` (stage) + PROPOSE commit message ONLY. NEVER run `git commit` (incl. `--amend`, `--no-verify`, `-S` overrides), NEVER push.
- COMMIT_FORMAT: Conventional Commits => `<type>(<scope>): <description>`
    ASSERT(type in [feat, fix, docs, style, refactor, test, chore], ELSE=STOP("Invalid commit type."))
    ASSERT(len(line[0]) <= 50, ELSE=STOP("Commit header exceeds 50 characters."))
    ASSERT(mood(description) == imperative, ELSE=STOP("Use imperative mood in description, e.g. 'add' not 'added'."))
    EXAMPLES: `feat(auth): add OAuth2 login support` | `fix(api): handle null response in user endpoint`

<!-- BEGIN OKF AGENT MEMORY -->
## 1. Behavioral Invariants & Constraints (RFC 2119)
- MUST execute `okf_search(query=keywords, limit=3)` before proposing architecture, dependencies, or changes.
- NEVER scan `knowledge/` via `list_dir`, `grep_search`, `find`, or raw readers.
- NEVER forge human verification (`verified:` is human-only; declare `generated: { by: "<actor>", at: "<iso-time>" }`).
- NEVER credit own agent/bot accounts in CONTRIBUTORS or release notes.
- PREFER native `okf_*` MCP tools OVER CLI fallback commands.

## 2. Guard Clauses & Scope Governance
- ON edit(@path/):
    IF first_visit(@path/) => okf_search(for_path=@path/)
    IF governance == "hold" => STOP("Subsystem frozen by governance. Request explicit human confirmation.")
    IF governance == "constraint" => MUST adhere to all listed invariants
    IF governance == "context" => proceed with awareness
- ON user_query(architecture | requirements | conventions | domain_facts):
    okf_search(query=keywords, limit=3) => evaluate summary description
    IF relevant => okf_show(concept_id) ONLY on demand
    IF updating_existing_concept => PREFER okf_update OVER okf_create

## 3. Completion Pipeline (Sequential Assertion Gates)
1. IF arch_decisions_made => MUST okf_create(architecture/*, type="decision", title=..., desc=...)
2. IF requirements_discovered => MUST okf_update(concept_id)
3. IF concepts_mutated => MUST sync(knowledge/log.md, knowledge/index.md)
4. IF skills_or_conventions_mutated => MUST run(`make sync-assets`)
5. ASSERT(okf_validate(strict=true, drift=true) == {errors: 0, warnings: 0}, ELSE=fix_before_exit)
<!-- END OKF AGENT MEMORY -->
