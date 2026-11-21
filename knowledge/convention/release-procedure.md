---
type: Process
title: "Release Procedure & Distribution Runbook"
description: "Canonical procedure for preparing releases, quality gates, file inventory, and the human boundary for signed commits, tags, and pushes."
tags: [release, playbook, runbook, distribution, workflow, tagging]
generated: { by: agent/mcp, at: "2026-10-12T10:04:02Z" }
status: stable
sources:
  - resource: https://github.com/okf-memory/okf-agent-memory/blob/main/docs/project/playbooks/RELEASE_PLAYBOOK.md
    id: release-playbook
    title: OKF Agent Memory Release Playbook
    last_modified: 2026-09-18
---

# Release Procedure & Distribution Runbook

This operational process governs the end-to-end workflow for preparing, verifying, and packaging releases of `okf-agent-memory`. It codifies `docs/project/playbooks/RELEASE_PLAYBOOK.md` directly into the agent's persistent memory.

---

## 1. Agent Invariant: Maintainer Authority Boundary

> [!IMPORTANT]
> **Human-in-the-Loop Constraint:**
> AI agents MUST prepare all release assets, run verification checks, and stage the changes with a prepared commit message.
> **AI agents MUST NEVER execute `git commit`, `git tag`, `git merge`, or `git push`**, and MUST NEVER disable or bypass the terminal sandbox to do so. Signing commits and tags with the maintainer's key, merging into `main`, tagging, and pushing (`origin develop`, `origin main`, `origin <tag>`) are strictly reserved for the maintainer.

---

## 2. Release Trigger: "Release" Action Pipeline

Whenever the user prompts with "release", "lass uns releasen", or specifies a target version `vX.Y.Z`, the agent must immediately execute the following steps in sequence:

### Phase 1: Quality & Verification Gates
1. Run formatting: `make fmt`
2. Sync embedded bootstrap assets (if `.agents/skills` changed): `make sync-assets`
3. Run comprehensive test suite: `make check` (vet, lint, unit tests, integration tests)
4. Validate knowledge bundle: `okf validate knowledge --strict --drift` (must report 0 errors, 0 warnings, 0 broken links)

### Phase 2: Master File Inventory & Release Documentation
Update or create all files in the canonical release inventory:
- [ ] `docs/releases/v<X.Y.Z>.md`: Comprehensive release notes with sections:
  - Overview & Highlights
  - Feature details & Breaking changes
  - Storage / Conformance hardening
  - Developer experience & CI updates
  - Community & Special Thanks (crediting external contributors and reporters by handle and issue/PR, whether or not an agent files their report; never credit the project's own agent or bot accounts)
  - Pre-built Binaries link
  - `### Full Changelog`: Link comparison `https://github.com/okf-memory/okf-agent-memory/compare/v<PREV>...v<CURR>`
- [ ] `docs/releases/README.md`: Add row with version, date, and core highlights
- [ ] `knowledge/log.md`: Record release entry under today's date
- [ ] `knowledge/roadmap/milestones.md`: Update milestone deliverables and current phase status
- [ ] `CONTRIBUTORS.md`: Credit contributors by handle (humans who author PRs using AI agents are credited normally); never credit the project's own bot/agent accounts like Jules or Claude Bot. Reporters whose operator is unknown are thanked in the release notes; adding them here is at the maintainer's discretion.
- [ ] `README.md`: Update supported version range (e.g. `(v0.1.0 – v0.4.3)`)

### Phase 3: Staging & Handover of Git Commands
The agent stages the release preparation files and stops. It does not commit:
```bash
git add docs/releases/ README.md knowledge/ CONTRIBUTORS.md
```
The agent supplies the prepared commit message, and the maintainer runs the signed commit and the remaining Git Flow steps:
```bash
git commit -S -m "chore(release): prepare release notes and knowledge for v<X.Y.Z>"
git checkout main
git merge --ff-only develop
git tag -s v<X.Y.Z> -m "Release v<X.Y.Z>: <Title>"
git checkout develop
```

### Phase 4: Handover to User
Report completion to the user, list what is staged, and supply the exact commit message and the final push command for the maintainer to execute:
```bash
git push origin develop && git push origin main && git push origin v<X.Y.Z>
```

---

# Related Concepts
- [Contributor Guidelines & PR Standards](contributing.md): GitFlow branch strategy and quality gates
- [Knowledge Lifecycle & Review Workflow](lifecycle.md): Read-Before-Write loop and audit logging
- [Project Roadmap & Development Milestones](../roadmap/milestones.md): Milestone tracking and phase progression
- [Retirement of Standalone Starter-Pack Distribution Archives](../architecture/starter-pack-retirement.md): Governs the exclusion of standalone starter-pack archives from release assets
