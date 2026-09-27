package cli

import (
	"bytes"
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
		"concept_b: verified date predates generated date",
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
