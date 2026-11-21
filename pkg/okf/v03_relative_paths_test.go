package okf_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

func buildPathMatrixBundle(t *testing.T, indexContent string) *okf.Bundle {
	t.Helper()
	dir := t.TempDir()
	if indexContent != "" {
		writeTestFile(t, dir, "index.md", indexContent)
	}
	writeTestFile(t, dir, "tables/orders.md", `---
type: BigQuery Table
title: Orders
---

Orders table.
`)
	writeTestFile(t, dir, "tables/customers.md", `---
type: BigQuery Table
title: Customers
---

Customers table.
`)
	writeTestFile(t, dir, "metrics/revenue.md", `---
type: Metric
title: Revenue
---

Revenue metric.
`)
	writeTestFile(t, dir, "references/attesters/sql-equality.py", "def check(receipt): return {'ok': True}\n")
	writeTestFile(t, dir, "references/skills/run-bq.md", "# Non-concept skill without frontmatter\n")

	b, err := okf.LoadBundle(dir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}
	return b
}

func TestV03UnifiedPathResolutionMatrix(t *testing.T) {
	versions := []struct {
		name         string
		indexContent string
		expectV02    bool
	}{
		{
			name:         "v0.3 explicit",
			indexContent: "---\nokf_version: \"0.3\"\n---\n\n# Root\n",
			expectV02:    false,
		},
		{
			name:         "v0.2 explicit",
			indexContent: "---\nokf_version: \"0.2\"\n---\n\n# Root\n",
			expectV02:    true,
		},
		{
			name:         "unversioned bundle",
			indexContent: "# Root without frontmatter\n",
			expectV02:    true,
		},
	}

	for _, ver := range versions {
		t.Run(ver.name, func(t *testing.T) {
			b := buildPathMatrixBundle(t, ver.indexContent)
			if b.IsV02OrEarlier() != ver.expectV02 {
				t.Fatalf("IsV02OrEarlier() = %v, want %v", b.IsV02OrEarlier(), ver.expectV02)
			}

			// 1. Absolute URLs with schemes
			for _, u := range []string{
				"https://example.com/docs",
				"http://internal.acme/wiki",
				"mailto:owner@acme.example",
				"gs://acme-bucket/exports/orders.parquet",
				"okf://@acme/rules/auth",
				"@acme/rules/auth",
			} {
				rp := b.ResolvePath("metrics/revenue.md", u)
				if rp.Kind != okf.PathKindExternal {
					t.Errorf("ResolvePath(%q) kind = %v, want PathKindExternal", u, rp.Kind)
				}
			}

			// 2. Root-relative paths (leading '/')
			rpRootConcept := b.ResolvePath("metrics/revenue.md", "/tables/orders.md")
			if rpRootConcept.Kind != okf.PathKindConcept || rpRootConcept.ConceptID != "tables/orders" || rpRootConcept.UsedFallback {
				t.Errorf("root-relative concept = %+v, want Concept(tables/orders, fallback=false)", rpRootConcept)
			}
			rpRootAsset := b.ResolvePath("metrics/revenue.md", "/references/attesters/sql-equality.py")
			if rpRootAsset.Kind != okf.PathKindAsset || rpRootAsset.RelPath != "references/attesters/sql-equality.py" {
				t.Errorf("root-relative asset = %+v, want Asset(references/attesters/sql-equality.py)", rpRootAsset)
			}
			rpRootSkipped := b.ResolvePath("metrics/revenue.md", "/references/skills/run-bq.md")
			if rpRootSkipped.Kind != okf.PathKindSkippedMD {
				t.Errorf("root-relative skipped md = %+v, want PathKindSkippedMD", rpRootSkipped)
			}

			// 3. File-relative paths (sibling and parent traversal within bundle)
			rpSibling := b.ResolvePath("tables/orders.md", "customers.md")
			if rpSibling.Kind != okf.PathKindConcept || rpSibling.ConceptID != "tables/customers" || rpSibling.UsedFallback {
				t.Errorf("sibling file-relative = %+v, want Concept(tables/customers, fallback=false)", rpSibling)
			}
			rpParentRel := b.ResolvePath("metrics/revenue.md", "../tables/orders.md")
			if rpParentRel.Kind != okf.PathKindConcept || rpParentRel.ConceptID != "tables/orders" || rpParentRel.UsedFallback {
				t.Errorf("parent file-relative = %+v, want Concept(tables/orders, fallback=false)", rpParentRel)
			}
			rpAssetRel := b.ResolvePath("metrics/revenue.md", "../references/attesters/sql-equality.py")
			if rpAssetRel.Kind != okf.PathKindAsset {
				t.Errorf("file-relative asset = %+v, want PathKindAsset", rpAssetRel)
			}

			// 4. Legacy unprefixed bundle-root path from a nested file (metrics/revenue.md -> "tables/orders.md")
			rpLegacy := b.ResolvePath("metrics/revenue.md", "tables/orders.md")
			if ver.expectV02 {
				if rpLegacy.Kind != okf.PathKindConcept || rpLegacy.ConceptID != "tables/orders" || !rpLegacy.UsedFallback {
					t.Errorf("v0.2 fallback = %+v, want Concept(tables/orders, fallback=true)", rpLegacy)
				}
			} else {
				if rpLegacy.Kind != okf.PathKindMissing || rpLegacy.UsedFallback {
					t.Errorf("v0.3 strict file-relative = %+v, want PathKindMissing(fallback=false)", rpLegacy)
				}
			}
		})
	}
}

func TestV03FrontmatterAndBodyUnifiedResolution(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "index.md", "---\nokf_version: \"0.3\"\n---\n\n# Root\n")
	writeTestFile(t, dir, "references/attesters/sql-equality.py", "def check(r): return {'ok': True}\n")
	writeTestFile(t, dir, "references/computations/rev.sql", "SELECT 1;\n")
	writeTestFile(t, dir, "references/skills/run-bq.md", "# Skill instructions\n")
	writeTestFile(t, dir, "tables/orders.md", `---
type: BigQuery Table
title: Orders
---

Orders table.
`)
	writeTestFile(t, dir, "computations/revenue.md", `---
type: Attested Computation
title: Revenue Computation
runtime: bigquery
computation: /references/computations/rev.sql
executor:
  resource: ../references/skills/run-bq.md
  receipt: [job_id, executed_sql]
attester:
  resource: /references/attesters/sql-equality.py
sources:
  - id: orders-src
    resource: ../tables/orders.md
  - id: scope-desc
    resource: all queries in BigQuery project X
---

Uses [orders](../tables/orders.md) and [attester](/references/attesters/sql-equality.py).
`)

	b, err := okf.LoadBundle(dir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}
	if len(b.BrokenLinks) != 0 {
		t.Fatalf("expected 0 broken links for valid assets/concepts/skills, got %+v", b.BrokenLinks)
	}
	if !slices.Contains(b.Graph["computations/revenue"], "tables/orders") {
		t.Fatalf("expected directed edge computations/revenue -> tables/orders, got %v", b.Graph["computations/revenue"])
	}
	// Edge should be deduplicated even though both body link and sources[].resource point to tables/orders
	count := 0
	for _, target := range b.Graph["computations/revenue"] {
		if target == "tables/orders" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected deduplicated single edge to tables/orders, got %d", count)
	}

	res := okf.Validate(b, okf.ValidateOptions{Strict: true})
	if !res.IsConformant || !res.GatePassed {
		t.Fatalf("expected conformant & passing bundle, got errors=%v warnings=%v findings=%v", res.Errors, res.Warnings, res.GateFindings)
	}
}

func TestV03PathTraversalRejectionCWE22(t *testing.T) {
	b := buildPathMatrixBundle(t, "---\nokf_version: \"0.3\"\n---\n\n# Root\n")

	escapes := []string{
		"../../etc/passwd",
		"../../../etc/passwd.md",
		"/../../etc/passwd",
		"/tables/../../../etc/passwd",
		"%2e%2e/%2e%2e/etc/passwd",
		`..\..\etc\passwd`,
		`C:\Windows\System32\drivers\etc\hosts`,
		"C:/etc/passwd",
		"//evil.example/share/secret.md",
	}

	for _, target := range escapes {
		rp := b.ResolvePath("metrics/revenue.md", target)
		if rp.Kind != okf.PathKindEscapes || rp.Err == nil {
			t.Errorf("ResolvePath(%q) = %+v, want PathKindEscapes with non-nil error", target, rp)
		}
	}

	// Verify validator rejects path traversal across body links and all frontmatter path fields.
	dir := t.TempDir()
	writeTestFile(t, dir, "index.md", "---\nokf_version: \"0.3\"\n---\n\n# Root\n")
	writeTestFile(t, dir, "metrics/bad.md", `---
type: Attested Computation
title: Malicious Paths
status: deprecated
superseded_by: ../../etc/passwd
resource: ../../etc/shadow
computation: ../../../secret.sql
executor:
  resource: /../../bin/sh
attester:
  resource: "%2e%2e/%2e%2e/etc/passwd"
sources:
  - id: bad-src
    resource: ../../etc/hosts
---

See [secret](../../etc/passwd) for details.
`)

	loaded, err := okf.LoadBundle(dir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}
	res := okf.Validate(loaded, okf.ValidateOptions{Strict: true})
	if res.IsConformant {
		t.Fatalf("expected IsConformant=false for path traversal bundle")
	}

	expectedFields := []string{
		"superseded_by",
		"resource",
		"computation",
		"executor.resource",
		"attester.resource",
		"sources[0].resource",
		"link \"../../etc/passwd\"",
	}
	allErrors := strings.Join(res.Errors, "\n")
	for _, want := range expectedFields {
		if !strings.Contains(allErrors, want) {
			t.Errorf("expected validation error mentioning %q, got:\n%s", want, allErrors)
		}
	}
}

func TestV03FrontmatterPathFieldCreatesConceptEdge(t *testing.T) {
	// Verify §6.4: a frontmatter path field (e.g. resource or executor.resource)
	// pointing to another concept creates a directed graph edge even with no body links.
	dir := t.TempDir()
	writeTestFile(t, dir, "index.md", "---\nokf_version: \"0.3\"\n---\n\n# Root\n")
	writeTestFile(t, dir, "skills/run-bq.md", `---
type: Playbook
title: Run BigQuery Skill
---

Instructions to execute BigQuery jobs.
`)
	writeTestFile(t, dir, "computations/rev.md", `---
type: Attested Computation
title: Revenue
runtime: bigquery
executor:
  resource: /skills/run-bq.md
---

No body links here.
`)

	b, err := okf.LoadBundle(dir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}
	if !slices.Contains(b.Graph["computations/rev"], "skills/run-bq") {
		t.Fatalf("expected concept edge via executor.resource, got %v", b.Graph["computations/rev"])
	}
	if len(b.Orphans) != 0 {
		t.Fatalf("expected 0 orphans when connected via executor.resource, got %v", b.Orphans)
	}
}
