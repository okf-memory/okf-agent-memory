package okf_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", full, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}

func TestV03SupersededByParseSerializeRoundTrip(t *testing.T) {
	raw := `---
type: Metric
title: Legacy Gross Profit
description: Deprecated metric stub.
status: deprecated
superseded_by: /metrics/gross-margin.md
---

Replaced by gross margin.
`
	c, err := okf.ParseConcept("metrics/gross-profit.md", raw)
	if err != nil {
		t.Fatalf("ParseConcept failed: %v", err)
	}
	if c.SupersededBy != "/metrics/gross-margin.md" {
		t.Fatalf("SupersededBy = %q, want %q", c.SupersededBy, "/metrics/gross-margin.md")
	}

	serialized := okf.SerializeConcept(c)
	if !strings.Contains(serialized, "superseded_by: /metrics/gross-margin.md") {
		t.Fatalf("serialized frontmatter missing superseded_by:\n%s", serialized)
	}

	reparsed, err := okf.ParseConcept("metrics/gross-profit.md", serialized)
	if err != nil {
		t.Fatalf("re-parse failed: %v", err)
	}
	if reparsed.SupersededBy != c.SupersededBy {
		t.Fatalf("round-trip SupersededBy = %q, want %q", reparsed.SupersededBy, c.SupersededBy)
	}

	// Null and empty values are fault-tolerantly treated as absent with parse warning.
	nullRaw := "---\ntype: Metric\ntitle: Null Stub\nstatus: deprecated\nsuperseded_by: null\n---\nBody\n"
	nullC, err := okf.ParseConcept("metrics/null.md", nullRaw)
	if err != nil {
		t.Fatalf("ParseConcept(null) failed: %v", err)
	}
	if nullC.SupersededBy != "" {
		t.Fatalf("expected empty SupersededBy for null, got %q", nullC.SupersededBy)
	}
	if len(nullC.ParseWarnings) == 0 {
		t.Fatalf("expected parse warning for null superseded_by")
	}
}

func TestV03SupersededByGraphEdgePreventsOrphan(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "index.md", "---\nokf_version: \"0.3\"\n---\n\n# Root\n")
	writeTestFile(t, dir, "metrics/old-margin.md", `---
type: Metric
title: Old Margin
status: deprecated
superseded_by: /metrics/gross-margin.md
---

Deprecated stub with no body links.
`)
	writeTestFile(t, dir, "metrics/gross-margin.md", `---
type: Metric
title: Gross Margin
status: stable
---

Active replacement metric.
`)

	b, err := okf.LoadBundle(dir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}

	if !slices.Contains(b.Graph["metrics/old-margin"], "metrics/gross-margin") {
		t.Fatalf("expected outbound graph edge metrics/old-margin -> metrics/gross-margin, got %v", b.Graph["metrics/old-margin"])
	}
	if !slices.Contains(b.InboundGraph["metrics/gross-margin"], "metrics/old-margin") {
		t.Fatalf("expected inbound graph edge on metrics/gross-margin from metrics/old-margin, got %v", b.InboundGraph["metrics/gross-margin"])
	}
	if len(b.Orphans) != 0 {
		t.Fatalf("expected 0 orphans when connected via superseded_by, got %v", b.Orphans)
	}
	if len(b.BrokenLinks) != 0 {
		t.Fatalf("expected 0 broken links, got %v", b.BrokenLinks)
	}

	res := okf.Validate(b, okf.ValidateOptions{Strict: true})
	if !res.IsConformant || !res.GatePassed {
		t.Fatalf("expected conformant & passing gate, got errors=%v warnings=%v findings=%v", res.Errors, res.Warnings, res.GateFindings)
	}
}

func TestV03SupersededByMultiHopAndConsumerForwarding(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "index.md", "---\nokf_version: \"0.3\"\n---\n\n# Root\n")
	writeTestFile(t, dir, "metrics/v1.md", `---
type: Metric
title: Legacy Revenue V1
description: Original legacy revenue definition.
status: deprecated
superseded_by: v2.md
---

Legacy V1 stub.
`)
	writeTestFile(t, dir, "metrics/v2.md", `---
type: Metric
title: Intermediate Revenue V2
description: Intermediate revenue definition.
status: deprecated
superseded_by: /metrics/v3.md
---

Intermediate V2 stub.
`)
	writeTestFile(t, dir, "metrics/v3.md", `---
type: Metric
title: Canonical Recognized Revenue
description: Active canonical revenue metric.
status: stable
---

Recognized revenue across all channels.
`)

	b, err := okf.LoadBundle(dir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}

	// 1. ResolveSuperseded multi-hop chain
	sr, err := b.ResolveSuperseded("metrics/v1")
	if err != nil {
		t.Fatalf("ResolveSuperseded failed: %v", err)
	}
	if !sr.Forwarded || sr.Target == nil || sr.Target.ID != "metrics/v3" {
		t.Fatalf("expected forward to metrics/v3, got forwarded=%v target=%v", sr.Forwarded, sr.Target)
	}
	wantChain := []string{"metrics/v1", "metrics/v2", "metrics/v3"}
	if !slices.Equal(sr.Chain, wantChain) {
		t.Fatalf("chain = %v, want %v", sr.Chain, wantChain)
	}

	// 2. ResolveScopedConcept (show) forwards cleanly
	shown, err := okf.ResolveScopedConcept("metrics/v1", dir, "", "", "")
	if err != nil {
		t.Fatalf("ResolveScopedConcept failed: %v", err)
	}
	if shown.ID != "metrics/v3" {
		t.Fatalf("show forwarded ID = %q, want %q", shown.ID, "metrics/v3")
	}
	wantFrom := []string{"metrics/v1", "metrics/v2"}
	if !slices.Equal(shown.SupersededFrom, wantFrom) {
		t.Fatalf("show SupersededFrom = %v, want %v", shown.SupersededFrom, wantFrom)
	}

	// 3. Search matching old stub title forwards to replacement concept
	results, err := b.SearchAdvanced(okf.SearchOptions{Query: "Legacy Revenue V1", Limit: 5})
	if err != nil {
		t.Fatalf("SearchAdvanced failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("expected search results, got 0")
	}
	if results[0].ConceptID != "metrics/v3" {
		t.Fatalf("expected top search result forwarded to metrics/v3, got %q", results[0].ConceptID)
	}
	if results[0].SupersededFrom == "" {
		t.Fatalf("expected SupersededFrom to be populated on forwarded search result")
	}

	// 4. Search with explicit status=deprecated filter returns the deprecated stubs directly
	depResults, err := b.SearchAdvanced(okf.SearchOptions{Filter: "status=deprecated", Limit: 10})
	if err != nil {
		t.Fatalf("SearchAdvanced(status=deprecated) failed: %v", err)
	}
	if len(depResults) != 2 {
		t.Fatalf("expected 2 deprecated concepts when filtering status=deprecated, got %d (%v)", len(depResults), depResults)
	}
	for _, r := range depResults {
		if r.ConceptID == "metrics/v3" {
			t.Fatalf("did not expect forwarding when explicitly filtering status=deprecated")
		}
	}

	// 5. Filter by superseded_by works directly
	supResults, err := b.SearchAdvanced(okf.SearchOptions{Filter: "superseded_by!=null", Limit: 10})
	if err != nil {
		t.Fatalf("SearchAdvanced(superseded_by!=null) failed: %v", err)
	}
	if len(supResults) != 2 {
		t.Fatalf("expected 2 concepts with superseded_by!=null, got %d", len(supResults))
	}
}

func TestV03SupersededByBrokenAndCyclicSafety(t *testing.T) {
	t.Run("broken target", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "index.md", "---\nokf_version: \"0.3\"\n---\n\n# Root\n")
		writeTestFile(t, dir, "metrics/old.md", `---
type: Metric
title: Old Metric
status: deprecated
superseded_by: /metrics/does-not-exist.md
---

Broken replacement.
`)
		b, err := okf.LoadBundle(dir)
		if err != nil {
			t.Fatalf("LoadBundle failed: %v", err)
		}
		if len(b.BrokenLinks) == 0 {
			t.Fatalf("expected broken link for missing superseded_by target")
		}
		res := okf.Validate(b, okf.ValidateOptions{Strict: true})
		if res.IsConformant {
			t.Fatalf("expected non-conformant validation result for broken superseded_by target")
		}
		if _, err := b.ResolveSuperseded("metrics/old"); err == nil {
			t.Fatalf("expected error from ResolveSuperseded for broken target")
		}
	})

	t.Run("2-node cycle A -> B -> A", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "index.md", "---\nokf_version: \"0.3\"\n---\n\n# Root\n")
		writeTestFile(t, dir, "a.md", `---
type: Metric
title: Metric A
status: deprecated
superseded_by: b.md
---

Points to B.
`)
		writeTestFile(t, dir, "b.md", `---
type: Metric
title: Metric B
status: deprecated
superseded_by: a.md
---

Points back to A.
`)
		b, err := okf.LoadBundle(dir)
		if err != nil {
			t.Fatalf("LoadBundle failed: %v", err)
		}

		// ResolveSuperseded must terminate and return ErrSupersededCycle
		_, err = b.ResolveSuperseded("a")
		if !errors.Is(err, okf.ErrSupersededCycle) {
			t.Fatalf("expected ErrSupersededCycle, got %v", err)
		}

		// Validate must report cycle error and never hang
		res := okf.Validate(b, okf.ValidateOptions{Strict: true})
		if res.IsConformant {
			t.Fatalf("expected validation error for cyclic superseded_by")
		}
		foundCycleErr := false
		for _, e := range res.Errors {
			if strings.Contains(e, "cyclic superseded_by") {
				foundCycleErr = true
				break
			}
		}
		if !foundCycleErr {
			t.Fatalf("expected 'cyclic superseded_by' error in %v", res.Errors)
		}

		// Consumer queries (Search + ResolveScopedConcept) must not infinite-loop
		if _, err := b.SearchAdvanced(okf.SearchOptions{Query: "Metric A", Limit: 5}); err != nil {
			t.Fatalf("SearchAdvanced failed on cycle: %v", err)
		}
		shown, err := okf.ResolveScopedConcept("a", dir, "", "", "")
		if err != nil {
			t.Fatalf("ResolveScopedConcept failed on cycle: %v", err)
		}
		if shown.ID != "a" {
			t.Fatalf("expected safe fallback to original concept 'a' on cycle, got %q", shown.ID)
		}
	})

	t.Run("self cycle A -> A", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "index.md", "---\nokf_version: \"0.3\"\n---\n\n# Root\n")
		writeTestFile(t, dir, "self.md", `---
type: Metric
title: Self Loop
status: deprecated
superseded_by: self.md
---

Self-referencing stub.
`)
		b, err := okf.LoadBundle(dir)
		if err != nil {
			t.Fatalf("LoadBundle failed: %v", err)
		}
		if _, err := b.ResolveSuperseded("self"); !errors.Is(err, okf.ErrSupersededCycle) {
			t.Fatalf("expected ErrSupersededCycle on self loop, got %v", err)
		}
		res := okf.Validate(b, okf.ValidateOptions{Strict: true})
		if res.IsConformant {
			t.Fatalf("expected validation error for self-cyclic superseded_by")
		}
	})

	t.Run("non-deprecated concept with superseded_by warns and does not forward", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "index.md", "---\nokf_version: \"0.3\"\n---\n\n# Root\n")
		writeTestFile(t, dir, "active.md", `---
type: Metric
title: Still Active
status: stable
superseded_by: next.md
---

Still active.
`)
		writeTestFile(t, dir, "next.md", `---
type: Metric
title: Next
status: stable
---

Next.
`)
		b, err := okf.LoadBundle(dir)
		if err != nil {
			t.Fatalf("LoadBundle failed: %v", err)
		}
		sr, err := b.ResolveSuperseded("active")
		if err != nil {
			t.Fatalf("ResolveSuperseded failed: %v", err)
		}
		if sr.Forwarded || sr.Target.ID != "active" {
			t.Fatalf("expected non-deprecated concept not to forward, got forwarded=%v target=%s", sr.Forwarded, sr.Target.ID)
		}
		res := okf.Validate(b, okf.ValidateOptions{})
		foundWarn := false
		for _, w := range res.Warnings {
			if strings.Contains(w, "superseded_by") && strings.Contains(w, "deprecated") {
				foundWarn = true
				break
			}
		}
		if !foundWarn {
			t.Fatalf("expected warning for superseded_by on non-deprecated concept, got %v", res.Warnings)
		}
	})
}
