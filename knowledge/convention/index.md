# Convention

* [Core Memory Principles & Agent Contract](principles.md) - Foundational design principles and minimal behavioral guarantees for agents maintaining persistent project memory.
* [Knowledge Lifecycle & Review Workflow](lifecycle.md) - Operational lifecycle stages and the Read-Before-Write loop for discovering, persisting, and updating project knowledge.
* [Contributor Guidelines & PR Standards](contributing.md) - Engineering standards, zero third-party dependency policy, validation rules, and PR workflow for contributors.
* [MCP Tool Security & Untrusted Agent Input](mcp-agent-safety.md) - Conventions for agentic memory operations treating all tool arguments as untrusted input and enforcing boundary confinement.
* [Automated Security Auditing & Defensive Verification Workflow](security-audit.md) - Proactive continuous security auditing with static analyzers (gosec, govulncheck), adversarial agent reviews, and regression testing.
* [Engineering & Coding Best Practices (Clean Code, TDD, DRY)](coding-standards.md) - Core software engineering conventions covering Clean Code, TDD, DRY, idiomatic Go, and zero third-party dependency design.
* [Dual-Memory Agent Architecture & Agent Action Grammar](dual-memory-architecture.md) - Two-layer memory model separating normative working memory (Push/AAG) from semantic domain memory (Pull/OKF).
* [Release Procedure & Distribution Runbook](release-procedure.md) - Canonical procedure for preparing releases, quality gates, file inventory, and the human boundary for signed commits, tags, and pushes.
* [CLI and MCP Command Modification Checklist](command-mutation-checklist.md) - Comprehensive checklist and invariant gates required when adding or modifying CLI commands, flags, arguments, and MCP tools.
