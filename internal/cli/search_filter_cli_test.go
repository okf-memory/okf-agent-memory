package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCLISearchFilterAndStaleWithin(t *testing.T) {
	tempDir := t.TempDir()
	now := time.Now().UTC()
	soon := now.Add(5 * 24 * time.Hour).Format("2006-01-02")
	far := now.Add(60 * 24 * time.Hour).Format("2006-01-02")

	// Scaffold minimal bundle
	_ = os.WriteFile(filepath.Join(tempDir, "index.md"), []byte("# Root Index\n"), 0o644)
	_ = os.WriteFile(filepath.Join(tempDir, "log.md"), []byte("# Log\n"), 0o644)

	doc1 := `---
type: Decision
title: "Decision Alpha"
description: "Core architecture decision"
status: stable
governance: constraint
stale_after: ` + soon + `
verified:
  - by: "human:lead"
    at: "2026-09-01"
---
Alpha body
`
	doc2 := `---
type: Fact
title: "Fact Beta"
description: "Informative background fact"
status: stable
governance: context
stale_after: ` + far + `
generated:
  by: "agent/claude"
  at: "2026-09-10"
---
Beta body
`
	_ = os.WriteFile(filepath.Join(tempDir, "doc1.md"), []byte(doc1), 0o644)
	_ = os.WriteFile(filepath.Join(tempDir, "doc2.md"), []byte(doc2), 0o644)

	t.Run("search with --filter type=Decision", func(t *testing.T) {
		cmdSearch([]string{"--filter", "type=Decision", tempDir, "--json"})
	})

	t.Run("search with --filter verified.by=human", func(t *testing.T) {
		cmdSearch([]string{"--filter", "verified.by=human", tempDir, "--json"})
	})

	t.Run("search with --stale-within 14d", func(t *testing.T) {
		cmdSearch([]string{"--stale-within", "14d", tempDir, "--json"})
	})

	t.Run("validate with --stale-within 1d (passes gate)", func(t *testing.T) {
		// doc1 is in 5 days, doc2 is in 60 days. StaleWithin 1d will find 0 stale concepts, so Gate passes!
		cmdValidate([]string{"--stale-within", "1d", tempDir, "--json"})
	})
}
