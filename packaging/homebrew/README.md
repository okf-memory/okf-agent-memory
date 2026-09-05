# Homebrew Tap for OKF Agent Memory

This directory contains the official Homebrew Formula for **OKF Agent Memory** (`okf`).

---

## 🚀 Setting up `okf-memory/homebrew-tap`

To allow users worldwide to install via `brew install okf-memory/tap/okf`, create a dedicated GitHub repository named **`homebrew-tap`** under your GitHub organization (`okf-memory/homebrew-tap`):

```bash
# 1. Create a new empty repository on GitHub: okf-memory/homebrew-tap
# 2. Clone it locally or initialize it:
mkdir -p homebrew-tap/Formula
cp packaging/homebrew/Formula/okf.rb homebrew-tap/Formula/okf.rb

cd homebrew-tap
git init
git add Formula/okf.rb
git commit -m "feat: add okf formula v0.1.0"
git remote add origin git@github.com:okf-memory/homebrew-tap.git
git branch -M main
git push -u origin main
```

---

## 📦 How Users Install It

Once `okf-memory/homebrew-tap` is pushed, any user on macOS (Apple Silicon or Intel) or Linux can install with a single command:

```bash
brew install okf-memory/tap/okf
```

Or by tapping first:

```bash
brew tap okf-memory/tap
brew install okf
```

---

## 🔄 Automatic Formula Updates on Release

The `.github/workflows/release.yml` workflow automatically computes the exact SHA256 hashes for all platform binaries and generates the updated `Formula/okf.rb` on each new release tag (`v*`).
