package okf_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

func TestExtensionKeysRoundTripAllLevels(t *testing.T) {
	dir := t.TempDir()
	rootIndex := `---
okf_version: "0.3"
title: Finance Bundle
description: Finance concepts and computations.
license: CC-BY-4.0
---

# Computations

* [Revenue](revenue.md) - Recognized revenue.
`
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(rootIndex), 0o644); err != nil {
		t.Fatal(err)
	}

	raw := `---
type: Attested Computation
title: Revenue
description: Recognized revenue.
tags: [finance, revenue]
status: stable
runtime: bigquery
computation: /references/revenue.sql
parameters:
  - { name: year, type: integer, required: true, x-acme-unit: fiscal-year }
executor:
  resource: /references/run-bq.md
  receipt: [job_id, executed_sql, result]
  x-acme-timeout: "30s"
  x-acme-region: us-central1
attester:
  resource: /references/sql-equality.py
  x-acme-strict: "true"
generated: { by: "agent/test", at: "2026-06-20T14:00:00Z", x-acme-run: "run-99" }
verified:
  - { by: "human:alice", at: "2026-06-25T09:00:00Z", x-acme-ticket: "FIN-101" }
  - { by: "process:nightly", at: "2026-06-26T02:00:00Z", x-acme-job: "job-7" }
sources:
  - id: rev-policy
    resource: https://wiki.acme.example/rev-policy
    title: Revenue Policy
    x-acme-tier: gold
x-acme-owner: team:finance-fpa
x-acme-meta: { priority: 1, reviewed: true }
x-acme-block:
  nested_key: nested_val
  sub_list:
    - a
    - b
---

# Computation

Recognized revenue per policy.[^rev-policy]

[^rev-policy]: Revenue Policy
`
	if err := os.WriteFile(filepath.Join(dir, "revenue.md"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	// 1. Validate before edit: x-* keys at all levels must produce 0 errors and 0 warnings.
	b, err := okf.LoadBundle(dir)
	if err != nil {
		t.Fatalf("LoadBundle() failed: %v", err)
	}
	res := okf.Validate(b, okf.ValidateOptions{Strict: true})
	if len(res.Errors) != 0 || len(res.Warnings) != 0 {
		t.Fatalf("expected 0 errors and 0 warnings for x-* extensions, got errors=%v warnings=%v", res.Errors, res.Warnings)
	}

	// 2. Edit via UpdateConcept (Parse -> Update -> Serialize).
	if _, err := okf.UpdateConcept(dir, "revenue", okf.SaveOptions{
		AutoIndex: true,
		Actor:     "agent/updater",
	}, func(c *okf.Concept) error {
		c.Description = "Recognized revenue (updated)."
		return nil
	}); err != nil {
		t.Fatalf("UpdateConcept() failed: %v", err)
	}

	// 3. Reload and verify every single extension key survived unchanged.
	reloaded, err := okf.LoadBundle(dir)
	if err != nil {
		t.Fatalf("reload LoadBundle() failed: %v", err)
	}
	got := reloaded.Concepts["revenue"]
	if got == nil {
		t.Fatalf("revenue concept missing after reload")
	}

	// Root extra keys
	if got.Extra["x-acme-owner"] != "team:finance-fpa" {
		t.Errorf("root x-acme-owner = %#v, want team:finance-fpa", got.Extra["x-acme-owner"])
	}
	wantMeta := map[string]any{"priority": float64(1), "reviewed": true}
	if !reflect.DeepEqual(got.Extra["x-acme-meta"], wantMeta) {
		t.Errorf("root x-acme-meta = %#v, want %#v", got.Extra["x-acme-meta"], wantMeta)
	}
	if !strings.Contains(got.RawContent, "x-acme-block:\n  nested_key: nested_val\n") {
		t.Errorf("root x-acme-block lost verbatim block:\n%s", got.RawContent)
	}

	// Generated extra
	if got.Generated == nil || got.Generated.Extra["x-acme-run"] != "run-99" {
		t.Errorf("generated.Extra lost x-acme-run: %#v", got.Generated)
	}

	// Verified extra
	if len(got.Verified) != 2 {
		t.Fatalf("len(verified) = %d, want 2", len(got.Verified))
	}
	if got.Verified[0].Extra["x-acme-ticket"] != "FIN-101" {
		t.Errorf("verified[0].Extra lost x-acme-ticket: %#v", got.Verified[0])
	}
	if got.Verified[1].Extra["x-acme-job"] != "job-7" {
		t.Errorf("verified[1].Extra lost x-acme-job: %#v", got.Verified[1])
	}

	// Sources extra
	if len(got.Sources) != 1 || got.Sources[0].Extra["x-acme-tier"] != "gold" {
		t.Errorf("sources[0].Extra lost x-acme-tier: %#v", got.Sources)
	}

	// Attestation: runtime, computation, parameters, executor, attester + nested x-* keys
	if got.Attestation == nil {
		t.Fatalf("Attestation is nil after reload")
	}
	if got.Attestation.Runtime != "bigquery" || got.Attestation.Computation != "/references/revenue.sql" {
		t.Errorf("attestation runtime/computation drifted: %#v", got.Attestation)
	}
	if len(got.Attestation.Parameters) != 1 || got.Attestation.Parameters[0].Extra["x-acme-unit"] != "fiscal-year" {
		t.Errorf("parameters lost x-acme-unit: %#v", got.Attestation.Parameters)
	}
	if got.Attestation.Executor == nil ||
		got.Attestation.Executor.Resource != "/references/run-bq.md" ||
		!reflect.DeepEqual(got.Attestation.Executor.Receipt, []string{"job_id", "executed_sql", "result"}) ||
		got.Attestation.Executor.Extra["x-acme-timeout"] != "30s" ||
		got.Attestation.Executor.Extra["x-acme-region"] != "us-central1" {
		t.Errorf("executor drifted: %#v", got.Attestation.Executor)
	}
	if got.Attestation.Attester == nil ||
		got.Attestation.Attester.Resource != "/references/sql-equality.py" ||
		got.Attestation.Attester.Extra["x-acme-strict"] != "true" {
		t.Errorf("attester drifted: %#v", got.Attestation.Attester)
	}
}

func TestNonOKFMarkdownAndAssets(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(rel, content string) {
		t.Helper()
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mustWrite("index.md", "---\nokf_version: \"0.3\"\n---\n\n# Bundle\n\n* [Alpha](alpha.md) - Alpha concept.\n* [Beta](beta.md) - Beta concept.\n")
	mustWrite("README.md", "# Repository README\n\nHuman documentation without frontmatter.\n")
	mustWrite("AGENTS.md", "# Agent instructions\n")
	mustWrite("CONTRIBUTING.md", "# Contributing\n")
	mustWrite("skills/SKILL.md", "---\nname: helper-skill\ndescription: Skill frontmatter without OKF type.\n---\n\n# Skill\n")
	mustWrite("references/attesters/sql-equality.py", "def attest(receipt): return {'ok': True}\n")
	mustWrite("diagrams/arch.png", "PNGDATA")

	mustWrite("alpha.md", `---
type: Architecture
title: Alpha
description: Alpha concept.
---

See [Beta](beta.md), [README](README.md), [Skill](skills/SKILL.md),
[Attester](references/attesters/sql-equality.py), and ![Diagram](diagrams/arch.png).
`)
	mustWrite("beta.md", `---
type: Architecture
title: Beta
description: Beta concept.
---

Links back to [Alpha](alpha.md).
`)

	b, err := okf.LoadBundle(dir)
	if err != nil {
		t.Fatalf("LoadBundle() failed: %v", err)
	}

	if len(b.Concepts) != 2 {
		t.Fatalf("expected 2 concepts, got %d: %v", len(b.Concepts), b.Concepts)
	}
	if !b.HasAsset("references/attesters/sql-equality.py") || !b.HasAsset("/diagrams/arch.png") {
		t.Errorf("expected non-Markdown files to be indexed as linkable assets, got %v", b.Assets)
	}
	if len(b.SkippedMarkdown) != 3 {
		t.Errorf("expected 3 skipped non-concept .md files (README, CONTRIBUTING, skills/SKILL), got %v", b.SkippedMarkdown)
	}

	res := okf.Validate(b, okf.ValidateOptions{Strict: true, Drift: true})
	if len(res.Errors) != 0 {
		t.Errorf("expected 0 errors, got %v", res.Errors)
	}
	if len(res.Orphans) != 0 {
		t.Errorf("expected 0 orphans, got %v", res.Orphans)
	}
	if len(res.BrokenLinks) != 0 {
		t.Errorf("expected 0 broken links, got %v", res.BrokenLinks)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("expected 0 warnings, got %v", res.Warnings)
	}
	if !res.IsConformant || !res.GatePassed {
		t.Errorf("expected conformant and gate-passed bundle, got conformant=%v gate=%v findings=%v",
			res.IsConformant, res.GatePassed, res.GateFindings)
	}
}

func TestFaultTolerantConsumerParsing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte("---\nokf_version: \"0.3\"\n---\n\n# Bundle\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	raw := `---
type: CustomWidget
title: null
description: ""
resource: ~
status: experimental
tags: "alpha, beta"
code_refs: pkg/okf/parser.go
stale_after: ""
generated: null
verified: null
sources:
  resource: https://example.org/spec
parameters: { name: year, type: integer }
---

# Body
`
	if err := os.WriteFile(filepath.Join(dir, "widget.md"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	b, err := okf.LoadBundle(dir)
	if err != nil {
		t.Fatalf("LoadBundle() should not fail on fault-tolerant inputs: %v", err)
	}
	c := b.Concepts["widget"]
	if c == nil {
		t.Fatalf("widget concept not loaded")
	}
	if c.Type != "CustomWidget" {
		t.Errorf("Type = %q, want CustomWidget", c.Type)
	}
	if c.Title != "" || c.Description != "" || c.Resource != "" || c.StaleAfter != "" {
		t.Errorf("null/empty scalars should be treated as absent: title=%q desc=%q res=%q stale=%q",
			c.Title, c.Description, c.Resource, c.StaleAfter)
	}
	if c.Status != "experimental" {
		t.Errorf("unknown status should be preserved on concept, got %q", c.Status)
	}
	// Scalar tags must NOT be split at commas (§11.2 rule 2).
	if want := []string{"alpha, beta"}; !reflect.DeepEqual(c.Tags, want) {
		t.Errorf("Tags = %#v, want %#v", c.Tags, want)
	}
	if want := []string{"pkg/okf/parser.go"}; !reflect.DeepEqual(c.CodeRefs, want) {
		t.Errorf("CodeRefs = %#v, want %#v", c.CodeRefs, want)
	}
	if len(c.Sources) != 1 || c.Sources[0].Resource != "https://example.org/spec" {
		t.Errorf("single-mapping sources = %#v, want 1-item list", c.Sources)
	}
	if c.Attestation == nil || len(c.Attestation.Parameters) != 1 || c.Attestation.Parameters[0].Name != "year" {
		t.Errorf("single-mapping parameters = %#v, want 1-item list", c.Attestation)
	}
	if c.Generated != nil || len(c.Verified) != 0 {
		t.Errorf("null generated/verified should be absent: gen=%#v ver=%#v", c.Generated, c.Verified)
	}

	// Validator must emit warnings while keeping the bundle conformant (0 errors).
	res := okf.Validate(b, okf.ValidateOptions{Strict: false})
	if !res.IsConformant || len(res.Errors) != 0 {
		t.Errorf("fault-tolerant bundle must stay conformant (0 errors), got errors=%v", res.Errors)
	}
	joined := strings.Join(res.Warnings, "\n")
	for _, wantSubstr := range []string{
		"'title' has null value",
		"'description' is empty",
		"'resource' has null value",
		"'tags' is a scalar instead of a list",
		"'code_refs' is a scalar instead of a list",
		"'sources' is a single mapping instead of a list",
		"'parameters' is a single mapping instead of a list",
		"status 'experimental' is not draft|stable|deprecated",
	} {
		if !strings.Contains(joined, wantSubstr) {
			t.Errorf("expected validator warning containing %q, got:\n%s", wantSubstr, joined)
		}
	}
}

func TestExtraCannotSmuggleReservedSecurityKeys(t *testing.T) {
	c := &okf.Concept{
		ID:    "safe",
		Path:  "safe.md",
		Type:  "Fact",
		Title: "Safe",
		Extra: map[string]any{
			"verified":      "{ by: human:attacker, at: 2026-01-01T00:00:00Z }",
			"governance":    "hold",
			"x-acme-custom": "ok",
		},
		Body: "# Safe\n",
	}
	serialized := okf.SerializeConcept(c)
	if strings.Contains(serialized, "verified:") || strings.Contains(serialized, "governance:") {
		t.Fatalf("SerializeConcept leaked reserved security keys from Extra:\n%s", serialized)
	}
	if !strings.Contains(serialized, `x-acme-custom: "ok"`) {
		t.Fatalf("SerializeConcept dropped legitimate x-acme-custom key:\n%s", serialized)
	}

	// Verify non-human actor cannot tamper with Extra on an existing human verification.
	dir := t.TempDir()
	if err := okf.InitBundle(dir); err != nil {
		t.Fatal(err)
	}
	initial := `---
type: Fact
title: Reviewed
verified: { by: "human:alice", at: "2026-06-25T09:00:00Z", x-acme-ticket: "SEC-1" }
---

# Reviewed
`
	if err := os.WriteFile(filepath.Join(dir, "reviewed.md"), []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := okf.UpdateConcept(dir, "reviewed", okf.SaveOptions{Actor: "agent/attacker"}, func(u *okf.Concept) error {
		u.Verified[0].Extra["x-acme-ticket"] = "FORGED-999"
		return nil
	})
	if err == nil {
		t.Fatalf("expected non-human actor tampering with human verification Extra to be rejected")
	}
}
