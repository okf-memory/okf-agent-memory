package okf_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

// TestCrashConsistency_MemoryWriteWithoutIndex simulates a process crash immediately
// after writing a concept file to disk, before UpdateParentIndex and AppendLogEntry run.
// This reproduces Kurisu's "Shape 2": memory exists on disk, but index and log are out of sync.
func TestCrashConsistency_MemoryWriteWithoutIndex(t *testing.T) {
	tmpDir := t.TempDir()

	if err := okf.InitBundle(tmpDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	// 1. Initial stable state: 1 concept properly indexed
	c1 := &okf.Concept{
		Path:        "architecture/storage.md",
		Type:        "Decision",
		Title:       "Storage Architecture",
		Description: "Filesystem markdown as SSoT.",
		Body:        "# Storage\n\nSSoT in git.",
	}
	if err := okf.SaveConcept(tmpDir, c1, okf.SaveOptions{IsNew: true, AutoLog: true, AutoIndex: true, Actor: "agent/test"}); err != nil {
		t.Fatalf("SaveConcept c1 failed: %v", err)
	}

	// 2. Simulate crash during SaveConcept of c2:
	// SaveConcept writes the markdown file, but process dies before autoIndex & autoLog!
	c2 := &okf.Concept{
		Path:        "architecture/cache.md",
		Type:        "Decision",
		Title:       "Caching Strategy",
		Description: "In-memory LRU cache for BM25 index.",
		Body:        "# Caching\n\nIn-memory cache.",
	}
	// Calling SaveConcept with autoIndex=false, autoLog=false simulates the crash gap:
	if err := okf.SaveConcept(tmpDir, c2, okf.SaveOptions{IsNew: true, Actor: "agent/test"}); err != nil {
		t.Fatalf("SaveConcept c2 failed: %v", err)
	}

	// 3. Inspect what the bundle sees
	b, err := okf.LoadBundle(tmpDir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}

	// Check BM25 search behavior:
	// In OKF, Search scans markdown files on disk directly, so it DOES find cache.md!
	searchResults := b.Search("caching strategy", 5)
	if len(searchResults) == 0 {
		t.Errorf("Expected BM25 search to find unindexed concept, but got 0 results")
	} else if searchResults[0].ConceptID != "architecture/cache" {
		t.Errorf("Expected top match to be 'architecture/cache', got '%s'", searchResults[0].ConceptID)
	}

	// Check Parent index.md content:
	// The parent index (architecture/index.md) does NOT list cache.md!
	indexPath := filepath.Join(tmpDir, "architecture", "index.md")
	indexBytes, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("Failed to read parent index: %v", err)
	}
	if strings.Contains(string(indexBytes), "cache.md") {
		t.Errorf("Index should NOT contain cache.md after simulated crash")
	}

	// Check Validation:
	// okf.Validate catches that cache.md is missing from architecture/index.md during drift check!
	res := okf.Validate(b, okf.ValidateOptions{Strict: true, Drift: true})
	if !res.IsConformant {
		t.Errorf("Expected bundle to remain conformant, but got errors: %v", res.Errors)
	}

	foundWarning := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "concept is not listed in parent index") && strings.Contains(w, "architecture/cache.md") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("Expected drift warning for unindexed concept, got warnings: %v", res.Warnings)
	}
}

// TestCrashConsistency_PhantomIndexEntry simulates Kurisu's "Shape 1":
// The index points to a concept that does NOT exist on disk (or was deleted/crashed before write).
func TestCrashConsistency_PhantomIndexEntry(t *testing.T) {
	tmpDir := t.TempDir()

	if err := okf.InitBundle(tmpDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	// Manually inject a dangling link into the root index.md
	rootIndex := filepath.Join(tmpDir, "index.md")
	phantomEntry := "* [Ghost Concept](ghost.md) - This concept file never made it to disk.\n"
	data, _ := os.ReadFile(rootIndex)
	_ = os.WriteFile(rootIndex, append(data, []byte(phantomEntry)...), 0o644)

	b, err := okf.LoadBundle(tmpDir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}

	// Check if Search finds the ghost:
	// Search scans disk, so ghost.md is not found.
	results := b.Search("Ghost Concept", 5)
	if len(results) > 0 {
		t.Errorf("Expected 0 results for phantom index entry, got %d", len(results))
	}

	// Check if Validate flags the dangling link in index.md:
	res := okf.Validate(b, okf.ValidateOptions{Strict: true, Drift: true})
	// OKF v0.2 conformance allows broken links:
	if !res.IsConformant {
		t.Errorf("Expected bundle to remain conformant despite broken links, got errors: %v", res.Errors)
	}
	// But broken link MUST be discovered and tracked:
	if len(res.BrokenLinks) != 1 || res.BrokenLinks[0].TargetHref != "ghost.md" {
		t.Errorf("Expected 1 broken link for ghost.md, got: %+v", res.BrokenLinks)
	}
	// Under --strict, broken links fail the gate:
	if res.GatePassed {
		t.Errorf("Expected GatePassed=false under strict validation due to broken link")
	}
}

// TestCrashConsistency_DeduplicationDivergence simulates an agent that checks
// the directory index before writing, experiencing duplication after a crash.
func TestCrashConsistency_DeduplicationDivergence(t *testing.T) {
	tmpDir := t.TempDir()

	if err := okf.InitBundle(tmpDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	// Step 1: Agent creates a concept, but crashes before index update
	c1 := &okf.Concept{
		Path:        "auth/session.md",
		Type:        "Fact",
		Title:       "Session Lifetime",
		Description: "Session timeout is 15 minutes.",
		Body:        "# Session Lifetime\n\nTTL = 900s.",
	}
	if err := okf.SaveConcept(tmpDir, c1, okf.SaveOptions{IsNew: true, Actor: "agent/v1"}); err != nil {
		t.Fatalf("SaveConcept c1 failed: %v", err)
	}

	// Step 2: Next agent session inspects auth/index.md (or reads files via index traversal)
	authIndex := filepath.Join(tmpDir, "auth", "index.md")
	idxContent, _ := os.ReadFile(authIndex)

	// Because auth/index.md is empty / missing session.md:
	isListedInIndex := strings.Contains(string(idxContent), "session.md")
	if isListedInIndex {
		t.Errorf("auth/index.md should not list session.md yet")
	}

	// If the agent naively relies on index inspection instead of BM25 search:
	// It decides to create "auth/session-timeout.md" thinking no session config exists!
	c2 := &okf.Concept{
		Path:        "auth/session-timeout.md",
		Type:        "Fact",
		Title:       "Session Timeout",
		Description: "Session lifetime is 15 minutes.",
		Body:        "# Session Timeout\n\nTTL = 900 seconds.",
	}
	if err := okf.SaveConcept(tmpDir, c2, okf.SaveOptions{IsNew: true, AutoLog: true, AutoIndex: true, Actor: "agent/v2"}); err != nil {
		t.Fatalf("SaveConcept c2 failed: %v", err)
	}

	// Now we have 2 redundant concepts describing the exact same fact!
	b, err := okf.LoadBundle(tmpDir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}

	if len(b.Concepts) != 2 {
		t.Fatalf("Expected 2 duplicate concepts, got %d", len(b.Concepts))
	}
	t.Logf("Duplication confirmed: bundle has both '%s' and '%s' due to unindexed crash state",
		b.Concepts["auth/session"].ID, b.Concepts["auth/session-timeout"].ID)
}
