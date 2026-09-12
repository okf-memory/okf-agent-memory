# OKF Agent Memory — Release Playbook

This playbook provides a step-by-step procedure for releasing new versions of **OKF Agent Memory** (`okf`).

---

## 1. Release Overview & Architecture

When a new version tag (e.g. `v0.1.0`) is published:
1. **GitHub Actions CI/CD** (`.github/workflows/release.yml`) automatically builds cross-platform binaries for macOS (ARM64 & x86_64), Linux (ARM64 & x86_64), and Windows (ARM64 & x86_64).
2. Binaries are compiled with injected build metadata (`Version`, `Commit`, `Date`).
3. Standalone starter pack archives (`.tar.gz` and `.zip`) are packaged.
4. SHA256 checksums and a generated Homebrew Formula (`Formula/okf.rb`) are attached to the GitHub Release.

---

## 2. Pre-Release Checklist

Before tagging a release, run through the following verification gates:

### Step 1: Quality & Conformance Gate
Run the full test and validation suite:

```bash
# 1. Format code according to Go best practices
make fmt

# 2. Synchronize active skills to embedded bootstrap assets (dogfooding)
make sync-assets

# 3. Run static analysis, unit tests (including drift checks), and strict OKF v0.2 bundle validation
make check

# 4. Run vulnerability scanner
make vuln
```

Ensure:
- ✅ All Go tests pass (`pkg/okf/...`), including `TestDogfoodingAssetDrift`
- ✅ Zero lint or vet errors
- ✅ 0 bundle validation errors, 0 broken links, 0 orphaned concepts across `knowledge/` and `examples/`

### Step 2: Security & Adversarial Audit Gate (MANDATORY)
Run automated security analysis and perform an adversarial review before every release:

```bash
# Run automated security linters and vulnerability checks
make audit-security
```

- **Conduct Security Specialist Agent Review**:
  - Extract the diff for the upcoming release against the previous tag:
    `git diff <previous-tag>..HEAD` (e.g. `git diff v0.1.2..HEAD`)
  - Instruct a Security Specialist Agent using [`docs/SECURITY_AUDIT.md`](./SECURITY_AUDIT.md) to audit the diff against all 4 audit areas.
  - Require a report status of **"Passed (0 High/Critical)"** before proceeding to Step 3.

### Step 3: Ensure Working Directory is Clean
```bash
git status
```
Ensure no uncommitted files or untracked scratch files remain.

---

## 3. Step-by-Step Release Process

> [!TIP]
> **Automatic Versioning in GitHub Actions**: When you push a Git tag (e.g. `git push origin v0.1.0`), GitHub Actions **automatically extracts the version** from the tag name and injects it into all release binaries, checksums, and the Homebrew formula. You do **not** need to configure or pass `VERSION=` to GitHub!
>
> `VERSION=...` is only needed if you want to test building release binaries **locally on your machine** before pushing the tag.

### Quick Release Summary
```bash
# 1. Author release notes and update changelog on develop and commit
git checkout develop
git pull origin develop
git add docs/releases/v0.2.0.md knowledge/log.md
git commit -m "chore(release): prepare release notes and changelog for v0.2.0"
git push origin develop

# 2. Merge develop into main
git checkout main
git pull origin main
git merge --ff-only develop
git push origin main

# 3. Tag and push on main (triggers automated build & GitHub release)
git tag -a v0.2.0 -m "Release v0.2.0"
git push origin v0.2.0
```

---

### Detailed Step-by-Step Guide

#### Step 1: Choose the Version Number
Follow [Semantic Versioning](https://semver.org/):
- **Patch (`v0.2.1`)**: Bug fixes, minor documentation updates, non-breaking improvements.
- **Minor (`v0.3.0`)**: New CLI commands, convention updates, new tool integrations.
- **Major (`v1.0.0`)**: Specification stabilization, breaking CLI/convention changes.

Export your target version (for changelog and optional local testing):
```bash
export RELEASE_VER="v0.2.0"
export CLEAN_VER="0.2.0"
```

#### Step 2: Prepare Release Notes & Update Changelog

1. **Author Release Notes in `docs/releases/${RELEASE_VER}.md`**:
   - Create `docs/releases/${RELEASE_VER}.md` containing the release highlights, detailed feature breakdown, security improvements, community acknowledgements, and upgrade instructions.
   - **Automated CI Fail-Fast Gate**: The GitHub Actions release workflow (`.github/workflows/release.yml`) strictly requires this file. If a release tag is pushed without `docs/releases/${RELEASE_VER}.md`, CI immediately fails at the start. When present, CI automatically injects its full markdown content into the GitHub Release body—no manual copy-pasting required!

2. **Add a dated entry in [`knowledge/log.md`](../knowledge/log.md)**:
   ```markdown
   ## YYYY-MM-DD
   * **Release**: Published version v0.2.0 with [Key Highlights].
   ```

3. **Validate knowledge bundle**:
   ```bash
   make validate
   ```

4. **Commit release notes and changelog**:
   ```bash
   git add docs/releases/${RELEASE_VER}.md knowledge/log.md
   git commit -m "chore(release): prepare release notes and changelog for ${RELEASE_VER}"
   ```

#### Step 3: (Optional) Test Local Release Build
If you want to verify that release artifacts build locally before tagging:

```bash
# Build cross-platform binaries locally
VERSION=${CLEAN_VER} make release

# Build starter pack archives locally
VERSION=${CLEAN_VER} make dist-bundle

# Verify local binary reports exact version
./bin/okf version
```

#### Step 4: Merge to Main and Push Git Tag (Triggers CI/CD)
Merge `develop` into `main`, then create an annotated tag and push it to GitHub. This triggers the GitHub Actions workflow, which automatically compiles all release binaries with the tag version:

```bash
# 1. Switch to main and merge develop
git checkout main
git pull origin main
git merge --ff-only develop
git push origin main

# 2. Create annotated tag on main
git tag -a "${RELEASE_VER}" -m "Release ${RELEASE_VER}"

# 3. Push tag to GitHub
git push origin "${RELEASE_VER}"
```

---

## 4. Post-Release & Distribution

### 1. Monitor GitHub Actions Release Workflow
1. Navigate to `https://github.com/okf-memory/okf-agent-memory/actions`.
2. Verify that the **Release** workflow completes with green checkmarks.
3. Verify the published assets on `https://github.com/okf-memory/okf-agent-memory/releases/tag/${RELEASE_VER}`:
   - `okf-darwin-arm64`, `okf-darwin-amd64`
   - `okf-linux-amd64`, `okf-linux-arm64`
   - `okf-windows-amd64.exe`, `okf-windows-arm64.exe`
   - `okf-starter-pack-${RELEASE_VER}.tar.gz` & `.zip`
   - `checksums.txt`
   - `Formula/okf.rb`

### 2. Verify Homebrew Tap Sync
The release workflow automatically updates `okf-memory/homebrew-tap` if `HOMEBREW_TAP_TOKEN` is configured:
1. Verify the automated commit in `https://github.com/okf-memory/homebrew-tap/commits/main`.
2. Test installation:
   ```bash
   brew update
   brew upgrade okf
   okf version
   ```
*(Fallback if token is absent: Manually copy the generated `Formula/okf.rb` from the release assets into `okf-memory/homebrew-tap`).*

### 3. Verify Documentation Website Sync
The release workflow automatically updates `okf-memory/okf-memory.github.io` if `WEBSITE_UPDATE_TOKEN` is configured:
1. Verify the automated commit in `https://github.com/okf-memory/okf-memory.github.io/commits/main`.
2. Verify that `version.json`, install scripts, and OpenGraph assets reflect the new version.
*(Fallback if token is absent: In `okf-memory.github.io`, run `./scripts/bump-version.sh ${RELEASE_VER} --push`).*

### 4. Verify Direct Go Install
Test direct global installation via Go toolchain:
```bash
go install github.com/okf-memory/okf-agent-memory/cmd/okf@${RELEASE_VER}
okf version
```

---

## 5. Hotfix Protocol

If an urgent bug is discovered after release:
1. Create a hotfix branch from `main`:
   ```bash
   git checkout -b hotfix/v0.2.1 main
   ```
2. Apply the fix and add a regression test in `pkg/okf/`.
3. Run `make check`.
4. Merge into `main` and release `v0.2.1` following this playbook.
5. Backport hotfix into `develop` to ensure changes are not lost:
   ```bash
   git checkout develop
   git merge main
   git push origin develop
   ```
