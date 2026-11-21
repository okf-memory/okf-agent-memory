package okf

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runGit(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	fullArgs := append([]string{"-c", "commit.gpgsign=false"}, args...)
	cmd := exec.Command("git", fullArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=Test User",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test User",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
}

func TestFrontmatterStaleness_GeneratedAtAndMtimeFallback(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "no-gen.md")
	if err := os.WriteFile(docPath, []byte("---\ntype: Fact\ntitle: No Gen\n---\nBody\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fixedTime := time.Date(2026, 5, 1, 12, 30, 0, 0, time.UTC)
	if err := os.Chtimes(docPath, fixedTime, fixedTime); err != nil {
		t.Fatal(err)
	}

	cWithGen := &Concept{
		ID:   "with-gen",
		Path: "with-gen.md",
		Generated: &Generated{
			By: "agent/test",
			At: "2026-04-15T08:00:00Z",
		},
	}
	st1 := FrontmatterStaleness(cWithGen, dir)
	if st1.Provider != FreshnessProviderFrontmatter || st1.LastTouchedAt != "2026-04-15T08:00:00Z" || st1.Commit != "" {
		t.Fatalf("unexpected staleness for generated.at concept: %+v", st1)
	}

	cNoGen := &Concept{
		ID:   "no-gen",
		Path: "no-gen.md",
	}
	st2 := FrontmatterStaleness(cNoGen, dir)
	if st2.Provider != FreshnessProviderFrontmatter || st2.LastTouchedAt != "2026-05-01T12:30:00Z" || st2.Commit != "" {
		t.Fatalf("unexpected mtime fallback staleness: %+v", st2)
	}

	st3 := FrontmatterStaleness(&Concept{ID: "missing", Path: "missing.md"}, dir)
	if st3.Provider != FreshnessProviderFrontmatter || st3.LastTouchedAt != "" {
		t.Fatalf("unexpected missing file staleness: %+v", st3)
	}
}

func TestFreshness_NonGitFallback(t *testing.T) {
	// t.TempDir() is outside any Git repository
	bundleDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(bundleDir, "index.md"), []byte("---\nokf_version: \"0.3\"\n---\n# Index\n- [Alpha](alpha.md) - Alpha concept\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	alphaContent := `---
type: Decision
title: Alpha Decision
description: Alpha concept
code_refs:
  - pkg/alpha/service.go
generated:
  by: agent/test
  at: "2026-06-01T10:00:00Z"
verified:
  - by: human:alice
    at: "2026-06-02T10:00:00Z"
---
Alpha body
`
	if err := os.WriteFile(filepath.Join(bundleDir, "alpha.md"), []byte(alphaContent), 0o600); err != nil {
		t.Fatal(err)
	}

	b, err := LoadBundle(bundleDir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}

	// Search with WithGitFreshness: true outside Git falls back cleanly to frontmatter
	results, err := b.SearchAdvanced(SearchOptions{
		Query:            "alpha",
		WithGitFreshness: true,
	})
	if err != nil {
		t.Fatalf("SearchAdvanced failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.TrustTier != TrustTierHumanReviewed {
		t.Errorf("expected trust_tier %q, got %q", TrustTierHumanReviewed, r.TrustTier)
	}
	if r.Staleness == nil {
		t.Fatal("expected staleness to be populated")
	}
	if r.Staleness.Provider != FreshnessProviderFrontmatter {
		t.Errorf("expected provider %q outside Git, got %q", FreshnessProviderFrontmatter, r.Staleness.Provider)
	}
	if r.Staleness.LastTouchedAt != "2026-06-01T10:00:00Z" {
		t.Errorf("expected last_touched_at from generated.at, got %q", r.Staleness.LastTouchedAt)
	}
	if r.Staleness.Commit != "" {
		t.Errorf("expected empty commit outside Git, got %q", r.Staleness.Commit)
	}

	// Single-concept resolution outside Git
	st := ResolveConceptStaleness(b.Concepts["alpha"], bundleDir, true)
	if st.Provider != FreshnessProviderFrontmatter || st.LastTouchedAt != "2026-06-01T10:00:00Z" {
		t.Fatalf("unexpected ResolveConceptStaleness outside Git: %+v", st)
	}

	// Validate with DriftGit: true outside Git succeeds gracefully with no drift warnings
	valRes := Validate(b, ValidateOptions{DriftGit: true})
	if !valRes.IsConformant || !valRes.GatePassed {
		t.Fatalf("expected conformant validation outside Git, got errors=%v warnings=%v", valRes.Errors, valRes.Warnings)
	}
	for _, w := range valRes.Warnings {
		if strings.Contains(w, "newer Git commit") {
			t.Errorf("unexpected git drift warning outside Git: %s", w)
		}
	}
}

func TestFreshness_GitRepoEnrichmentAndUntrackedFallback(t *testing.T) {
	requireGit(t)

	repoDir := t.TempDir()
	runGit(t, repoDir, nil, "init")

	bundleDir := filepath.Join(repoDir, "knowledge")
	if err := os.MkdirAll(filepath.Join(bundleDir, "decisions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "index.md"), []byte("---\nokf_version: \"0.3\"\n---\n# Index\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	trackedContent := `---
type: Decision
title: Tracked Auth Decision
description: Uses OAuth2 PKCE
generated:
  by: agent/test
  at: "2026-01-01T00:00:00Z"
verified:
  - by: human:reviewer
    at: "2026-01-02T00:00:00Z"
---
Tracked auth body
`
	trackedPath := filepath.Join(bundleDir, "decisions", "auth.md")
	if err := os.WriteFile(trackedPath, []byte(trackedContent), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoDir, nil, "add", ".")
	commitDate := "2026-08-15T14:30:00+00:00"
	runGit(t, repoDir, []string{
		"GIT_AUTHOR_DATE=" + commitDate,
		"GIT_COMMITTER_DATE=" + commitDate,
	}, "commit", "-m", "add auth decision")
	expectedHash := runGit(t, repoDir, nil, "rev-parse", "--short", "HEAD")

	// Add an untracked concept (not committed to Git yet)
	untrackedContent := `---
type: Fact
title: Untracked Auth Note
description: Draft auth note
generated:
  by: agent/test
  at: "2026-09-01T09:00:00Z"
verified:
  - by: tool/ci-bot
    at: "2026-09-02T09:00:00Z"
---
Untracked auth body
`
	untrackedPath := filepath.Join(bundleDir, "decisions", "draft-auth.md")
	if err := os.WriteFile(untrackedPath, []byte(untrackedContent), 0o600); err != nil {
		t.Fatal(err)
	}

	b, err := LoadBundle(bundleDir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}

	// 1. Default search (WithGitFreshness: false) -> frontmatter only, zero git calls
	defaultRes, err := b.SearchAdvanced(SearchOptions{Query: "auth"})
	if err != nil {
		t.Fatalf("default search failed: %v", err)
	}
	if len(defaultRes) != 2 {
		t.Fatalf("expected 2 results, got %d", len(defaultRes))
	}
	for _, r := range defaultRes {
		if r.Staleness == nil || r.Staleness.Provider != FreshnessProviderFrontmatter {
			t.Errorf("expected frontmatter provider by default for %s, got %+v", r.ConceptID, r.Staleness)
		}
		if r.Staleness.Commit != "" {
			t.Errorf("expected empty commit by default for %s, got %q", r.ConceptID, r.Staleness.Commit)
		}
	}

	// 2. Git-enriched search (WithGitFreshness: true)
	gitRes, err := b.SearchAdvanced(SearchOptions{Query: "auth", WithGitFreshness: true})
	if err != nil {
		t.Fatalf("git search failed: %v", err)
	}
	byID := make(map[string]SearchResult)
	for _, r := range gitRes {
		byID[r.ConceptID] = r
	}

	authRes := byID["decisions/auth"]
	if authRes.TrustTier != TrustTierHumanReviewed {
		t.Errorf("expected human-reviewed trust_tier, got %q", authRes.TrustTier)
	}
	if authRes.Staleness == nil || authRes.Staleness.Provider != FreshnessProviderGit {
		t.Fatalf("expected git provider for tracked concept, got %+v", authRes.Staleness)
	}
	if authRes.Staleness.Commit != expectedHash {
		t.Errorf("expected commit %q, got %q", expectedHash, authRes.Staleness.Commit)
	}
	gotTime, ok := parseTimestamp(authRes.Staleness.LastTouchedAt)
	wantTime, _ := parseTimestamp(commitDate)
	if !ok || !gotTime.Equal(wantTime) {
		t.Errorf("expected last_touched_at %s, got %s", commitDate, authRes.Staleness.LastTouchedAt)
	}

	draftRes := byID["decisions/draft-auth"]
	if draftRes.TrustTier != TrustTierMachineConfirmed {
		t.Errorf("expected machine-confirmed trust_tier, got %q", draftRes.TrustTier)
	}
	if draftRes.Staleness == nil || draftRes.Staleness.Provider != FreshnessProviderFrontmatter {
		t.Fatalf("expected frontmatter fallback for untracked concept, got %+v", draftRes.Staleness)
	}
	if draftRes.Staleness.LastTouchedAt != "2026-09-01T09:00:00Z" {
		t.Errorf("expected frontmatter generated.at for untracked concept, got %q", draftRes.Staleness.LastTouchedAt)
	}

	// 3. Single-concept Git staleness
	singleSt := ResolveConceptStaleness(b.Concepts["decisions/auth"], bundleDir, true)
	if singleSt.Provider != FreshnessProviderGit || singleSt.Commit != expectedHash {
		t.Errorf("unexpected ResolveConceptStaleness for tracked concept: %+v", singleSt)
	}
	singleUntracked := ResolveConceptStaleness(b.Concepts["decisions/draft-auth"], bundleDir, true)
	if singleUntracked.Provider != FreshnessProviderFrontmatter {
		t.Errorf("unexpected ResolveConceptStaleness for untracked concept: %+v", singleUntracked)
	}

	// 4. Design invariant: no new frontmatter keys written to disk
	rawBytes, err := os.ReadFile(trackedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(rawBytes) != trackedContent {
		t.Errorf("concept file on disk was modified unexpectedly")
	}
}

func TestValidate_DriftGit(t *testing.T) {
	requireGit(t)

	repoDir := t.TempDir()
	runGit(t, repoDir, nil, "init")

	bundleDir := filepath.Join(repoDir, "knowledge")
	if err := os.MkdirAll(filepath.Join(bundleDir, "decisions"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"pkg/auth", "pkg/stable", "pkg/globmod"} {
		if err := os.MkdirAll(filepath.Join(repoDir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	write := func(rel, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repoDir, filepath.FromSlash(rel)), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write("pkg/auth/service.go", "package auth\nfunc Login() {}\n")
	write("pkg/stable/core.go", "package stable\nfunc Core() {}\n")
	write("pkg/globmod/a.go", "package globmod\nfunc A() {}\n")
	write("knowledge/index.md", "---\nokf_version: \"0.3\"\n---\n# Index\n")
	write("knowledge/decisions/auth.md", `---
type: Decision
title: Auth Service
description: Auth decision
code_refs:
  - pkg/auth/service.go
generated:
  by: agent/test
  at: "2026-07-01T10:00:00Z"
---
Auth body
`)
	write("knowledge/decisions/stable.md", `---
type: Decision
title: Stable Core
description: Stable core decision
code_refs:
  - pkg/stable/core.go
generated:
  by: agent/test
  at: "2026-07-01T10:00:00Z"
---
Stable body
`)
	write("knowledge/decisions/glob.md", `---
type: Decision
title: Glob Module
description: Glob module decision
code_refs:
  - pkg/globmod/*.go
generated:
  by: agent/test
  at: "2026-07-01T10:00:00Z"
---
Glob body
`)

	t1 := "2026-07-01T10:00:00+00:00"
	runGit(t, repoDir, nil, "add", ".")
	runGit(t, repoDir, []string{"GIT_AUTHOR_DATE=" + t1, "GIT_COMMITTER_DATE=" + t1}, "commit", "-m", "initial")

	// At T1, code and concepts were committed together -> 0 git drift warnings
	b1, err := LoadBundle(bundleDir)
	if err != nil {
		t.Fatal(err)
	}
	res1 := Validate(b1, ValidateOptions{DriftGit: true})
	for _, w := range res1.Warnings {
		if strings.Contains(w, "newer Git commit") {
			t.Fatalf("expected no drift warnings at T1, got: %s", w)
		}
	}

	// At T2, modify pkg/auth/service.go and pkg/globmod/a.go, leave pkg/stable/core.go untouched
	write("pkg/auth/service.go", "package auth\nfunc Login() { /* v2 */ }\n")
	write("pkg/globmod/a.go", "package globmod\nfunc A() { /* v2 */ }\n")
	t2 := "2026-07-10T15:00:00+00:00"
	runGit(t, repoDir, nil, "add", "pkg/auth/service.go", "pkg/globmod/a.go")
	runGit(t, repoDir, []string{"GIT_AUTHOR_DATE=" + t2, "GIT_COMMITTER_DATE=" + t2}, "commit", "-m", "update auth and globmod")

	b2, err := LoadBundle(bundleDir)
	if err != nil {
		t.Fatal(err)
	}
	res2 := Validate(b2, ValidateOptions{DriftGit: true})
	var authDrifted, globDrifted, stableDrifted bool
	for _, w := range res2.Warnings {
		if strings.Contains(w, "decisions/auth.md") && strings.Contains(w, "newer Git commit") {
			authDrifted = true
		}
		if strings.Contains(w, "decisions/glob.md") && strings.Contains(w, "newer Git commit") {
			globDrifted = true
		}
		if strings.Contains(w, "decisions/stable.md") && strings.Contains(w, "newer Git commit") {
			stableDrifted = true
		}
	}
	if !authDrifted {
		t.Errorf("expected decisions/auth.md to be flagged for git drift, warnings=%v", res2.Warnings)
	}
	if !globDrifted {
		t.Errorf("expected decisions/glob.md to be flagged for git drift, warnings=%v", res2.Warnings)
	}
	if stableDrifted {
		t.Errorf("expected untouched decisions/stable.md NOT to be flagged, warnings=%v", res2.Warnings)
	}

	// At T3, update decisions/auth.md so it catches up with pkg/auth/service.go
	write("knowledge/decisions/auth.md", `---
type: Decision
title: Auth Service
description: Auth decision updated
code_refs:
  - pkg/auth/service.go
generated:
  by: agent/test
  at: "2026-07-12T10:00:00Z"
---
Auth body v2
`)
	t3 := "2026-07-12T10:00:00+00:00"
	runGit(t, repoDir, nil, "add", "knowledge/decisions/auth.md")
	runGit(t, repoDir, []string{"GIT_AUTHOR_DATE=" + t3, "GIT_COMMITTER_DATE=" + t3}, "commit", "-m", "sync auth concept")

	b3, err := LoadBundle(bundleDir)
	if err != nil {
		t.Fatal(err)
	}
	res3 := Validate(b3, ValidateOptions{DriftGit: true})
	for _, w := range res3.Warnings {
		if strings.Contains(w, "decisions/auth.md") && strings.Contains(w, "newer Git commit") {
			t.Errorf("expected decisions/auth.md drift warning to clear after update, got: %s", w)
		}
	}
}

func TestSearch_TrustTierAndStalenessAllSubcases(t *testing.T) {
	bundleDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(bundleDir, "index.md"), []byte("# Index\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	doc := `---
type: Decision
title: Unverified Rule
description: Rule for testing all search paths
code_refs:
  - pkg/demo/file.go
generated:
  by: agent/test
  at: "2026-08-01T00:00:00Z"
---
Body text
`
	if err := os.WriteFile(filepath.Join(bundleDir, "rule.md"), []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}

	b, err := LoadBundle(bundleDir)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		opts SearchOptions
	}{
		{"for-path", SearchOptions{TargetPath: "pkg/demo/file.go"}},
		{"query", SearchOptions{Query: "Unverified"}},
		{"filter", SearchOptions{Filter: "type=Decision"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			results, err := b.SearchAdvanced(tc.opts)
			if err != nil {
				t.Fatalf("SearchAdvanced failed: %v", err)
			}
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}
			if results[0].TrustTier != TrustTierUnverified {
				t.Errorf("expected trust_tier %q, got %q", TrustTierUnverified, results[0].TrustTier)
			}
			if results[0].Staleness == nil || results[0].Staleness.Provider != FreshnessProviderFrontmatter || results[0].Staleness.LastTouchedAt != "2026-08-01T00:00:00Z" {
				t.Errorf("unexpected staleness: %+v", results[0].Staleness)
			}
		})
	}
}
