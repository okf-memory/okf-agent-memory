package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

func runGitCLI(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	fullArgs := append([]string{"-c", "commit.gpgsign=false"}, args...)
	cmd := exec.Command("git", fullArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=CLI Test",
		"GIT_AUTHOR_EMAIL=cli@example.com",
		"GIT_COMMITTER_NAME=CLI Test",
		"GIT_COMMITTER_EMAIL=cli@example.com",
	)
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func TestCLI_SearchShowAndValidateGitFreshness(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}

	repoDir := t.TempDir()
	t.Setenv("OKF_USER_DIR", filepath.Join(repoDir, "no-user"))
	t.Setenv("OKF_SYSTEM_DIR", filepath.Join(repoDir, "no-system"))

	runGitCLI(t, repoDir, nil, "init")

	bundleDir := filepath.Join(repoDir, "knowledge")
	if err := os.MkdirAll(filepath.Join(bundleDir, "decisions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "pkg/pay"), 0o755); err != nil {
		t.Fatal(err)
	}

	write := func(rel, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repoDir, filepath.FromSlash(rel)), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write("pkg/pay/charge.go", "package pay\nfunc Charge() {}\n")
	write("knowledge/index.md", "---\nokf_version: \"0.3\"\n---\n# Index\n- [Payment Flow](decisions/payment.md) - Payment processing rule\n")
	write("knowledge/decisions/payment.md", `---
type: Decision
title: Payment Flow
description: Payment processing rule
code_refs:
  - pkg/pay/charge.go
generated:
  by: agent/cli
  at: "2026-05-01T10:00:00Z"
verified:
  - by: human:lead
    at: "2026-05-02T10:00:00Z"
---
Payment body
`)

	t1 := "2026-08-01T12:00:00+00:00"
	runGitCLI(t, repoDir, nil, "add", ".")
	runGitCLI(t, repoDir, []string{"GIT_AUTHOR_DATE=" + t1, "GIT_COMMITTER_DATE=" + t1}, "commit", "-m", "init payment")
	expectedHash := runGitCLI(t, repoDir, nil, "rev-parse", "--short", "HEAD")

	// 1. okf search without --with-git-freshness -> frontmatter provider
	outDefault := captureOutput(func() {
		cmdSearch([]string{"payment", bundleDir, "--scope", "project", "--json"})
	})
	var defaultResults []okf.SearchResult
	if err := json.Unmarshal([]byte(outDefault), &defaultResults); err != nil {
		t.Fatalf("failed to decode default search JSON: %v\n%s", err, outDefault)
	}
	if len(defaultResults) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(defaultResults))
	}
	if defaultResults[0].TrustTier != okf.TrustTierHumanReviewed {
		t.Errorf("expected trust_tier %q, got %q", okf.TrustTierHumanReviewed, defaultResults[0].TrustTier)
	}
	if defaultResults[0].Staleness == nil || defaultResults[0].Staleness.Provider != okf.FreshnessProviderFrontmatter {
		t.Errorf("expected frontmatter provider without flag, got %+v", defaultResults[0].Staleness)
	}

	// 2. okf search --with-git-freshness --json -> git provider + commit hash
	outGit := captureOutput(func() {
		cmdSearch([]string{"payment", bundleDir, "--scope", "project", "--with-git-freshness", "--json"})
	})
	var gitResults []okf.SearchResult
	if err := json.Unmarshal([]byte(outGit), &gitResults); err != nil {
		t.Fatalf("failed to decode git search JSON: %v\n%s", err, outGit)
	}
	if len(gitResults) != 1 || gitResults[0].Staleness == nil {
		t.Fatalf("expected 1 git search result with staleness, got %+v", gitResults)
	}
	if gitResults[0].Staleness.Provider != okf.FreshnessProviderGit || gitResults[0].Staleness.Commit != expectedHash {
		t.Errorf("expected git provider with commit %q, got %+v", expectedHash, gitResults[0].Staleness)
	}

	// 3. okf search text output surfaces Trust and Touched
	outText := captureOutput(func() {
		cmdSearch([]string{"payment", bundleDir, "--scope", "project", "--with-git-freshness"})
	})
	if !strings.Contains(outText, "Trust: human-reviewed") || !strings.Contains(outText, "git:"+expectedHash) {
		t.Errorf("expected text output to include trust_tier and git commit, got:\n%s", outText)
	}

	// 4. okf show --with-git-freshness --json & text
	outShowJSON := captureOutput(func() {
		cmdShow([]string{"decisions/payment", bundleDir, "--with-git-freshness", "--json"})
	})
	var showMap map[string]any
	if err := json.Unmarshal([]byte(outShowJSON), &showMap); err != nil {
		t.Fatalf("failed to decode show JSON: %v\n%s", err, outShowJSON)
	}
	if showMap["trust_tier"] != okf.TrustTierHumanReviewed {
		t.Errorf("expected show trust_tier human-reviewed, got %v", showMap["trust_tier"])
	}
	stalenessMap, _ := showMap["staleness"].(map[string]any)
	if stalenessMap["provider"] != okf.FreshnessProviderGit || stalenessMap["commit"] != expectedHash {
		t.Errorf("unexpected show staleness: %v", stalenessMap)
	}

	outShowText := captureOutput(func() {
		cmdShow([]string{"decisions/payment", bundleDir, "--with-git-freshness"})
	})
	if !strings.Contains(outShowText, "Trust Tier:  human-reviewed") || !strings.Contains(outShowText, "git:"+expectedHash) {
		t.Errorf("expected show text output to include Trust Tier and git commit, got:\n%s", outShowText)
	}

	// 5. Commit a newer code change and run okf validate --drift-git --json
	write("pkg/pay/charge.go", "package pay\nfunc Charge() { /* changed */ }\n")
	t2 := "2026-08-10T12:00:00+00:00"
	runGitCLI(t, repoDir, nil, "add", "pkg/pay/charge.go")
	runGitCLI(t, repoDir, []string{"GIT_AUTHOR_DATE=" + t2, "GIT_COMMITTER_DATE=" + t2}, "commit", "-m", "modify charge")

	outVal := captureOutput(func() {
		cmdValidate([]string{bundleDir, "--drift-git", "--json"})
	})
	var valRes okf.ValidationResult
	if err := json.Unmarshal([]byte(outVal), &valRes); err != nil {
		t.Fatalf("failed to decode validate JSON: %v\n%s", err, outVal)
	}
	foundDrift := false
	for _, w := range valRes.Warnings {
		if strings.Contains(w, "decisions/payment.md") && strings.Contains(w, "newer Git commit") {
			foundDrift = true
		}
	}
	if !foundDrift {
		t.Errorf("expected --drift-git warning for decisions/payment.md, got warnings=%v", valRes.Warnings)
	}
}
