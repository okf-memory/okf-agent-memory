package okf

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestEffectiveGovernance(t *testing.T) {
	tests := []struct {
		name     string
		concept  Concept
		expected string
	}{
		{
			name: "explicit constraint",
			concept: Concept{
				ID:         "architecture/adr-1",
				Governance: "constraint",
			},
			expected: "constraint",
		},
		{
			name: "explicit hold normalized to lowercase",
			concept: Concept{
				ID:         "architecture/auth",
				Governance: "HOLD",
			},
			expected: "hold",
		},
		{
			name: "explicit context",
			concept: Concept{
				ID:         "convention/notes",
				Governance: "context",
			},
			expected: "context",
		},
		{
			name: "implicit constraint for convention",
			concept: Concept{
				ID: "convention/coding-style",
			},
			expected: "constraint",
		},
		{
			name: "implicit context for architecture",
			concept: Concept{
				ID: "architecture/storage-engine",
			},
			expected: "context",
		},
		{
			name: "implicit context for domain project",
			concept: Concept{
				ID: "project/overview",
			},
			expected: "context",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := tc.concept.EffectiveGovernance()
			if actual != tc.expected {
				t.Errorf("EffectiveGovernance() = %q, want %q", actual, tc.expected)
			}
		})
	}
}

func TestGovernanceAndCodeRefsParsingAndSerialization(t *testing.T) {
	raw := `---
type: convention
title: "Testing Guidelines"
description: "Rules for unit and integration testing."
governance: constraint
code_refs: ["pkg/okf/**/*.go", "cmd/okf/main.go"]
---

# Testing Guidelines
All tests must pass cleanly.
`

	concept, err := ParseConcept("convention/testing.md", raw)
	if err != nil {
		t.Fatalf("ParseConcept failed: %v", err)
	}

	if concept.Governance != "constraint" {
		t.Errorf("expected Governance 'constraint', got %q", concept.Governance)
	}
	if len(concept.CodeRefs) != 2 {
		t.Fatalf("expected 2 code_refs, got %d", len(concept.CodeRefs))
	}
	if concept.CodeRefs[0] != "pkg/okf/**/*.go" || concept.CodeRefs[1] != "cmd/okf/main.go" {
		t.Errorf("unexpected code_refs: %v", concept.CodeRefs)
	}

	serialized := SerializeConcept(concept)
	reparsed, err := ParseConcept("convention/testing.md", serialized)
	if err != nil {
		t.Fatalf("re-ParseConcept failed: %v", err)
	}

	if reparsed.Governance != "constraint" {
		t.Errorf("expected reparsed Governance 'constraint', got %q", reparsed.Governance)
	}
	if len(reparsed.CodeRefs) != 2 || reparsed.CodeRefs[0] != "pkg/okf/**/*.go" || reparsed.CodeRefs[1] != "cmd/okf/main.go" {
		t.Errorf("unexpected reparsed code_refs: %v", reparsed.CodeRefs)
	}
}

func TestSearchForPath(t *testing.T) {
	b := &Bundle{
		Concepts: map[string]*Concept{
			"convention/style": {
				ID:          "convention/style",
				Title:       "Go Code Style",
				Type:        "convention",
				Description: "Pure standard library rules",
				Governance:  "constraint",
				CodeRefs:    []string{"pkg/**/*.go"},
			},
			"architecture/auth-freeze": {
				ID:          "architecture/auth-freeze",
				Title:       "Auth Subsystem Freeze",
				Type:        "architecture",
				Description: "Refactoring in progress, do not modify without signoff",
				Governance:  "hold",
				CodeRefs:    []string{"pkg/auth/login.go", "pkg/auth/session.go"},
			},
			"architecture/storage": {
				ID:          "architecture/storage",
				Title:       "Storage Architecture",
				Type:        "architecture",
				Description: "How the bundle is stored on disk",
				Governance:  "context",
				CodeRefs:    []string{"pkg/okf/bundle.go"},
			},
		},
		Graph:        make(map[string][]string),
		InboundGraph: make(map[string][]string),
	}

	t.Run("matches exact path with hold priority", func(t *testing.T) {
		// Both convention/style (via pkg/**/*.go) and architecture/auth-freeze (via pkg/auth/login.go) match.
		// architecture/auth-freeze has hold governance, so it must rank first!
		results := b.SearchForPath("pkg/auth/login.go", "", 10)
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if results[0].ConceptID != "architecture/auth-freeze" {
			t.Errorf("expected first result to be 'architecture/auth-freeze' (hold), got %q", results[0].ConceptID)
		}
		if results[0].Governance != "hold" {
			t.Errorf("expected first result governance 'hold', got %q", results[0].Governance)
		}
		if results[1].ConceptID != "convention/style" {
			t.Errorf("expected second result to be 'convention/style' (constraint), got %q", results[1].ConceptID)
		}
	})

	t.Run("matches glob path", func(t *testing.T) {
		results := b.SearchForPath("pkg/okf/parser.go", "", 10)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].ConceptID != "convention/style" {
			t.Errorf("expected 'convention/style', got %q", results[0].ConceptID)
		}
	})

	t.Run("returns empty when no code_refs match", func(t *testing.T) {
		results := b.SearchForPath("docs/README.md", "", 10)
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("text query combined with path boosts relevant concept", func(t *testing.T) {
		results := b.SearchForPath("pkg/auth/login.go", "freeze", 10)
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if results[0].ConceptID != "architecture/auth-freeze" {
			t.Errorf("expected 'architecture/auth-freeze' first, got %q", results[0].ConceptID)
		}
	})

	t.Run("handles leading slashes and absolute paths gracefully", func(t *testing.T) {
		// Leading slash: matches both convention/style (constraint) and architecture/storage (context)
		results := b.SearchForPath("/pkg/okf/bundle.go", "", 10)
		if len(results) != 2 || results[0].ConceptID != "convention/style" || results[1].ConceptID != "architecture/storage" {
			t.Errorf("unexpected results for leading slash path: %v", results)
		}

		// Absolute path
		cwd, _ := os.Getwd()
		repoRoot := filepath.Dir(filepath.Dir(cwd)) // up from pkg/okf to repo root
		absPath := filepath.Join(repoRoot, "pkg", "okf", "bundle.go")
		resultsAbs := b.SearchForPath(absPath, "", 10)
		if len(resultsAbs) != 2 || resultsAbs[0].ConceptID != "convention/style" || resultsAbs[1].ConceptID != "architecture/storage" {
			t.Errorf("unexpected results for absolute path %s: %v", absPath, resultsAbs)
		}
	})
}

func TestGovernanceAndCodeRefsValidation(t *testing.T) {
	tempDir := t.TempDir()
	bundleDir := filepath.Join(tempDir, "knowledge")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create valid source file
	srcDir := filepath.Join(tempDir, "pkg", "auth")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "auth.go"), []byte("package auth"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a concept with invalid governance
	b := &Bundle{
		RootPath:    bundleDir,
		DeclaredVer: "0.2",
		Concepts: map[string]*Concept{
			"architecture/bad-gov": {
				Path:        "architecture/bad-gov.md",
				Type:        "architecture",
				Title:       "Bad Gov",
				Description: "Has invalid governance field",
				Governance:  "super-strict",
			},
			"architecture/stale-ref": {
				Path:        "architecture/stale-ref.md",
				Type:        "architecture",
				Title:       "Stale Ref",
				Description: "Points to non-existent code",
				Governance:  "constraint",
				CodeRefs:    []string{"pkg/auth/auth.go", "pkg/deleted/old.go"},
			},
			"architecture/traversal-ref": {
				Path:        "architecture/traversal-ref.md",
				Type:        "architecture",
				Title:       "Traversal Ref",
				Description: "Attempts path traversal",
				Governance:  "constraint",
				CodeRefs:    []string{"../../etc/passwd", "/etc/shadow"},
			},
		},
		Indexes:      make(map[string]string),
		Graph:        make(map[string][]string),
		InboundGraph: make(map[string][]string),
	}

	res := Validate(b, ValidateOptions{Strict: true, Drift: true})

	// Check that invalid governance produced a GateFinding
	foundGovError := false
	for _, f := range res.GateFindings {
		if filepath.Base(f) != "" && (f == "architecture/bad-gov.md: governance 'super-strict' is not constraint|hold|context") {
			foundGovError = true
			break
		}
	}
	if !foundGovError {
		t.Errorf("expected gate finding for invalid governance, got: %v", res.GateFindings)
	}

	// Check that path traversal in code_refs produced GateFindings
	foundTraversalError := slices.Contains(res.GateFindings, "architecture/traversal-ref.md: code_refs '../../etc/passwd' contains forbidden '..' traversal")
	foundAbsError := slices.Contains(res.GateFindings, "architecture/traversal-ref.md: code_refs '/etc/shadow' must be a relative path")
	if !foundTraversalError || !foundAbsError {
		t.Errorf("expected gate findings for traversal and absolute code_refs, got: %v", res.GateFindings)
	}

	// Check that drift warning was produced for "pkg/deleted/old.go"
	foundDriftWarning := slices.Contains(res.Warnings, "architecture/stale-ref.md: code_refs 'pkg/deleted/old.go' points to non-existent path")
	if !foundDriftWarning {
		t.Errorf("expected drift warning for missing code_ref, got: %v", res.Warnings)
	}
}
