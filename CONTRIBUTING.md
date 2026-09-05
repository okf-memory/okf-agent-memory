# Contributing to OKF Agent Memory

Thank you for your interest in contributing to **OKF Agent Memory**! We welcome contributions from developers, researchers, and AI practitioners.

---

## 🧭 Core Principles

Before submitting code or documentation, please keep our core tenets in mind:

1. **Zero External Dependencies**: The Go core library and CLI (`pkg/okf`, `cmd/okf`) must remain 100% zero-dependency, relying strictly on the Go standard library.
2. **Strict Spec Conformance**: All knowledge structures must comply with the [OKF v0.2 Specification](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md).
3. **Walk the Talk (Memory First)**: Any pull request introducing architectural decisions, CLI commands, or workflow conventions must update `knowledge/` and pass validation.

---

## 🛠️ Development Setup

### Prerequisites
- **Go**: 1.24 or higher
- **Make**: Recommended for build and test targets
- **Git**

### Getting Started

1. **Fork and clone** the repository:
   ```bash
   git clone https://github.com/<your-username>/okf-agent-memory.git
   cd okf-agent-memory
   ```

2. **Build the binary**:
   ```bash
   make build
   # Binary will be placed in bin/okf
   ```

3. **Run tests**:
   ```bash
   make test
   ```

4. **Validate the repository's knowledge bundle**:
   ```bash
   make validate
   # or: ./bin/okf validate knowledge --strict --drift
   ```

---

## 📋 Development Workflow

### 1. Branching Strategy
- Base all feature and bugfix branches off `main`:
  ```bash
  git checkout -b feat/your-feature-name
  # or
  git checkout -b fix/issue-description
  ```

### 2. Commit Message Conventions
We adhere to [Conventional Commits](https://www.conventionalcommits.org/):
- `feat:` A new feature or capability (e.g. `feat: add bm25 scoring boost for tags`)
- `fix:` A bug fix (e.g. `fix: handle missing frontmatter delimiter safely`)
- `docs:` Documentation updates
- `refactor:` Code refactoring with no behavioral changes
- `test:` Adding or updating tests
- `chore:` Build scripts, CI workflow, or release updates

### 3. Knowledge & Memory Maintenance
If your PR modifies architecture, CLI behavior, conventions, or APIs:
- Use `./bin/okf` (or `okf`) to create or update concepts:
  ```bash
  ./bin/okf create <concept-id> knowledge --type <type> --title "<title>" --desc "<summary>"
  ./bin/okf update <concept-id> knowledge --desc "<new-summary>"
  ./bin/okf relate <src-id> <target-id> knowledge --desc "<relationship>"
  ```
- Verify that `knowledge/log.md` and parent indices are cleanly maintained.
- Ensure strict conformance and zero drift:
  ```bash
  make validate
  ```

---

## 🧪 Pre-Submission Checklist

Before submitting a Pull Request, verify that all of the following pass locally:

- [ ] `go fmt ./...` (or `make fmt`) has been run.
- [ ] `go vet ./...` (or `make vet`) passes without errors.
- [ ] `make test` runs with 100% passing tests.
- [ ] `make validate` passes with `0 errors, 0 warnings, 0 orphans, 0 broken links`.
- [ ] Relevant documentation (`README.md`, `docs/`, `SKILL.md`) is updated.

---

## 🚀 Submitting a Pull Request

1. Push your branch to your fork.
2. Open a Pull Request against `main`.
3. Provide a clear description explaining:
   - What problem does this PR solve?
   - What changes were made?
   - How did you verify the changes?
4. Ensure all GitHub Actions checks pass green.

Thank you for helping make AI agent memory reliable, persistent, and standardized!
