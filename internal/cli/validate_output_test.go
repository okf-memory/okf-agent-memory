package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

func countPrefixedLines(output, prefix string) int {
	count := 0
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, prefix) {
			count++
		}
	}
	return count
}

func TestPrintValidationOutput_NonStrictReconciliation(t *testing.T) {
	// Recreate Issue #35 scenario: 96 warnings, 2 broken links, 120 orphans, 0 errors
	warnings := make([]string, 96)
	for i := range warnings {
		warnings[i] = "concept: generated.by is not a valid actor"
	}
	brokenLinks := []okf.BrokenLink{
		{SourceConcept: "c1", TargetHref: "t1.md", Reason: "target does not exist"},
		{SourceConcept: "c2", TargetHref: "t2.md", Reason: "target does not exist"},
	}
	orphans := make([]string, 120)
	for i := range orphans {
		orphans[i] = "orphan_concept"
	}

	res := &okf.ValidationResult{
		BundlePath:   "knowledge",
		DeclaredVer:  "0.2",
		ConceptCount: 152,
		Errors:       []string{},
		Warnings:     warnings,
		GateFindings: []string{},
		BrokenLinks:  brokenLinks,
		Orphans:      orphans,
		StaleCount:   0,
		IsConformant: true,
		GatePassed:   true,
	}

	var buf bytes.Buffer
	printValidationOutput(&buf, res, "knowledge", false, false)
	out := buf.String()

	warnLines := countPrefixedLines(out, "warn  ")
	gateLines := countPrefixedLines(out, "gate  ")
	errorLines := countPrefixedLines(out, "error ")

	expectedTotalWarns := len(warnings) + len(brokenLinks) + len(orphans) // 96 + 2 + 120 = 218
	if warnLines != expectedTotalWarns {
		t.Fatalf("expected %d 'warn  ' lines, got %d", expectedTotalWarns, warnLines)
	}
	if gateLines != 0 {
		t.Fatalf("expected 0 'gate  ' lines in non-strict mode, got %d", gateLines)
	}
	if errorLines != 0 {
		t.Fatalf("expected 0 'error ' lines, got %d", errorLines)
	}

	expectedSummary := "OKF v0.2 check of \"knowledge\" (v0.2): 152 concept(s), 0 error(s), 218 warning(s); 2 broken link(s), 120 orphan(s), 0 stale. Conformant."
	if !strings.Contains(out, expectedSummary) {
		t.Errorf("summary mismatch.\nWant substring: %s\nGot output:\n%s", expectedSummary, out)
	}
}

func TestPrintValidationOutput_StrictTaxonomy(t *testing.T) {
	// Strict mode: GateFindings, BrokenLinks, and Orphans must print as 'gate  ' and be counted in 'gate finding(s)'
	gateFindings := []string{
		"concept_a: missing 'by' field in 'generated'",
		"concept_b: governance 'invalid' is not constraint|hold|context",
	}
	brokenLinks := []okf.BrokenLink{
		{SourceConcept: "c1", TargetHref: "missing.md", Reason: "not found"},
	}
	orphans := []string{"orphan_1", "orphan_2"}
	warnings := []string{
		"concept_x: index description differs from concept",
		"concept_y: code_refs points to non-existent file",
	}
	errors := []string{
		"concept_z: 'type' field is missing or empty",
	}

	res := &okf.ValidationResult{
		BundlePath:   "knowledge",
		DeclaredVer:  "0.2",
		ConceptCount: 5,
		Errors:       errors,
		Warnings:     warnings,
		GateFindings: gateFindings,
		BrokenLinks:  brokenLinks,
		Orphans:      orphans,
		StaleCount:   0,
		IsConformant: false,
		GatePassed:   false,
	}

	var buf bytes.Buffer
	printValidationOutput(&buf, res, "knowledge", true, false)
	out := buf.String()

	gateLines := countPrefixedLines(out, "gate  ")
	warnLines := countPrefixedLines(out, "warn  ")
	errorLines := countPrefixedLines(out, "error ")

	expectedGates := len(gateFindings) + len(brokenLinks) + len(orphans) // 2 + 1 + 2 = 5
	if gateLines != expectedGates {
		t.Errorf("expected %d 'gate  ' lines, got %d", expectedGates, gateLines)
	}
	if warnLines != len(warnings) {
		t.Errorf("expected %d 'warn  ' lines, got %d", len(warnings), warnLines)
	}
	if errorLines != len(errors) {
		t.Errorf("expected %d 'error ' lines, got %d", len(errors), errorLines)
	}

	expectedSummary := "OKF v0.2 check of \"knowledge\" (v0.2): 5 concept(s), 1 error(s), 5 gate finding(s), 2 warning(s); 1 broken link(s), 2 orphan(s), 0 stale [--strict]. NOT conformant."
	if !strings.Contains(out, expectedSummary) {
		t.Errorf("strict summary mismatch.\nWant substring: %s\nGot output:\n%s", expectedSummary, out)
	}
}

func TestPrintValidationOutput_StrictClean(t *testing.T) {
	res := &okf.ValidationResult{
		BundlePath:   "knowledge",
		DeclaredVer:  "0.2",
		ConceptCount: 7,
		Errors:       []string{},
		Warnings:     []string{},
		GateFindings: []string{},
		BrokenLinks:  []okf.BrokenLink{},
		Orphans:      []string{},
		StaleCount:   0,
		IsConformant: true,
		GatePassed:   true,
	}

	var buf bytes.Buffer
	printValidationOutput(&buf, res, "knowledge", true, false)
	out := buf.String()

	expectedSummary := "OKF v0.2 check of \"knowledge\" (v0.2): 7 concept(s), 0 error(s), 0 gate finding(s), 0 warning(s); 0 broken link(s), 0 orphan(s), 0 stale [--strict]. Conformant."
	if !strings.Contains(out, expectedSummary) {
		t.Errorf("strict clean summary mismatch.\nWant substring: %s\nGot output:\n%s", expectedSummary, out)
	}
}

func TestPrintValidationOutput_StrictGateFailedConformant(t *testing.T) {
	// Syntactically conformant (errors == 0), but gate failed due to an orphan
	res := &okf.ValidationResult{
		BundlePath:   "knowledge",
		DeclaredVer:  "0.2",
		ConceptCount: 1,
		Errors:       []string{},
		Warnings:     []string{},
		GateFindings: []string{},
		BrokenLinks:  []okf.BrokenLink{},
		Orphans:      []string{"orphan_1"},
		StaleCount:   0,
		IsConformant: true,
		GatePassed:   false,
	}

	var buf bytes.Buffer
	printValidationOutput(&buf, res, "knowledge", true, false)
	out := buf.String()

	expectedSummary := "OKF v0.2 check of \"knowledge\" (v0.2): 1 concept(s), 0 error(s), 1 gate finding(s), 0 warning(s); 0 broken link(s), 1 orphan(s), 0 stale [--strict]. Conformant, but the producer gate failed."
	if !strings.Contains(out, expectedSummary) {
		t.Errorf("gate failure summary mismatch.\nWant substring: %s\nGot output:\n%s", expectedSummary, out)
	}
}

func TestValidateCLI_TrustTierTransitions(t *testing.T) {
	validateCmd, ok := FindCommand("validate")
	if !ok {
		t.Fatalf("validate command not registered")
	}

	writeBundle := func(t *testing.T, conceptContent string) string {
		t.Helper()
		dir := t.TempDir()
		rootIndex := "---\nokf_version: \"0.3\"\n---\n\n# Bundle\n\n* [Policy](policy.md) - Policy doc.\n"
		if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(rootIndex), 0o644); err != nil {
			t.Fatalf("write index.md: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "policy.md"), []byte(conceptContent), 0o644); err != nil {
			t.Fatalf("write policy.md: %v", err)
		}
		return dir
	}

	runValidateJSON := func(t *testing.T, dir string) struct {
		IsConformant bool     `json:"is_conformant"`
		GatePassed   bool     `json:"gate_passed"`
		Errors       []string `json:"errors"`
		GateFindings []string `json:"gate_findings"`
		Scorecard    *struct {
			Stewardship int `json:"stewardship"`
		} `json:"scorecard"`
	} {
		t.Helper()
		out := captureOutput(func() {
			validateCmd.Run([]string{dir, "--strict", "--scorecard", "--json"})
		})
		var res struct {
			IsConformant bool     `json:"is_conformant"`
			GatePassed   bool     `json:"gate_passed"`
			Errors       []string `json:"errors"`
			GateFindings []string `json:"gate_findings"`
			Scorecard    *struct {
				Stewardship int `json:"stewardship"`
			} `json:"scorecard"`
		}
		if err := json.Unmarshal([]byte(out), &res); err != nil {
			t.Fatalf("unmarshal validate json: %v\noutput:\n%s", err, out)
		}
		return res
	}

	t.Run("historical human sign-off after agent edit passes strict gate with reduced stewardship", func(t *testing.T) {
		dir := writeBundle(t, `---
type: Policy
title: Historical Review
description: Policy doc.
generated: { by: "reference_agent/gemini-2.5-pro", at: "2026-06-25T10:00:00Z" }
verified:
  - { by: "human:alice", at: "2026-06-20T09:00:00Z" }
---
# Policy
`)
		res := runValidateJSON(t, dir)
		if !res.IsConformant || !res.GatePassed {
			t.Fatalf("expected strict gate pass, got conformant=%v gate=%v errors=%v findings=%v",
				res.IsConformant, res.GatePassed, res.Errors, res.GateFindings)
		}
		if len(res.GateFindings) != 0 {
			t.Errorf("expected 0 gate findings for historical verification, got %v", res.GateFindings)
		}
		if res.Scorecard == nil || res.Scorecard.Stewardship != 20 {
			t.Errorf("expected Stewardship=20 for historical-only human review, got %+v", res.Scorecard)
		}
	})

	t.Run("team actor passes strict gate without human stewardship", func(t *testing.T) {
		dir := writeBundle(t, `---
type: Policy
title: Team Review
description: Policy doc.
generated: { by: "reference_agent/gemini-2.5-pro", at: "2026-06-20T09:00:00Z" }
verified:
  - { by: "team:platform", at: "2026-06-25T10:00:00Z" }
---
# Policy
`)
		res := runValidateJSON(t, dir)
		if !res.IsConformant || !res.GatePassed || len(res.GateFindings) != 0 {
			t.Fatalf("expected strict gate pass for team actor, got conformant=%v gate=%v findings=%v",
				res.IsConformant, res.GatePassed, res.GateFindings)
		}
		if res.Scorecard == nil || res.Scorecard.Stewardship != 20 {
			t.Errorf("expected Stewardship=20 for team verification, got %+v", res.Scorecard)
		}
	})

	t.Run("historical plus current human sign-off awards full stewardship", func(t *testing.T) {
		dir := writeBundle(t, `---
type: Policy
title: Re-verified Policy
description: Policy doc.
generated: { by: "reference_agent/gemini-2.5-pro", at: "2026-06-25T10:00:00Z" }
verified:
  - { by: "human:alice", at: "2026-06-20T09:00:00Z" }
  - { by: "human:bob", at: "2026-06-26T09:00:00Z" }
---
# Policy
`)
		res := runValidateJSON(t, dir)
		if !res.IsConformant || !res.GatePassed || len(res.GateFindings) != 0 {
			t.Fatalf("expected strict gate pass, got conformant=%v gate=%v findings=%v",
				res.IsConformant, res.GatePassed, res.GateFindings)
		}
		if res.Scorecard == nil || res.Scorecard.Stewardship != 100 {
			t.Errorf("expected Stewardship=100 for current human review, got %+v", res.Scorecard)
		}
	})
}
