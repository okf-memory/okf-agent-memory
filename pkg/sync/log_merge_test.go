package sync_test

import (
	"strings"
	"testing"

	"github.com/okf-memory/okf-agent-memory/pkg/sync"
)

func TestMergeLogContent_UnionsEntriesByDateHeading(t *testing.T) {
	localLog := `# Knowledge Log

## 2026-09-17
* **Create**: Created concepts/local-feature.md

## 2026-09-15
* **Init**: Initialized knowledge bundle
`

	remoteLog := `# Knowledge Log

## 2026-09-17
* **Create**: Created concepts/remote-feature.md

## 2026-09-16
* **Decision**: Added decisions/arch-decision.md

## 2026-09-15
* **Init**: Initialized knowledge bundle
`

	merged, err := sync.MergeLogContent([]byte(localLog), []byte(remoteLog))
	if err != nil {
		t.Fatalf("unexpected MergeLogContent error: %v", err)
	}

	mergedStr := string(merged)

	// Must contain both 2026-09-17 entries under the single 2026-09-17 heading
	if strings.Count(mergedStr, "## 2026-09-17") != 1 {
		t.Fatalf("expected exactly one ## 2026-09-17 heading, got:\n%s", mergedStr)
	}
	if !strings.Contains(mergedStr, "* **Create**: Created concepts/local-feature.md") {
		t.Errorf("missing local entry in merged log")
	}
	if !strings.Contains(mergedStr, "* **Create**: Created concepts/remote-feature.md") {
		t.Errorf("missing remote entry in merged log")
	}

	// Must contain 2026-09-16 entry
	if !strings.Contains(mergedStr, "## 2026-09-16") {
		t.Errorf("missing 2026-09-16 heading")
	}
	if !strings.Contains(mergedStr, "* **Decision**: Added decisions/arch-decision.md") {
		t.Errorf("missing remote 2026-09-16 entry")
	}

	// Must contain 2026-09-15 entry exactly once (deduplicated)
	if strings.Count(mergedStr, "* **Init**: Initialized knowledge bundle") != 1 {
		t.Errorf("expected duplicate init entry to be deduplicated, got:\n%s", mergedStr)
	}

	// Headings must appear in descending chronological order
	idx17 := strings.Index(mergedStr, "## 2026-09-17")
	idx16 := strings.Index(mergedStr, "## 2026-09-16")
	idx15 := strings.Index(mergedStr, "## 2026-09-15")
	if idx17 >= idx16 || idx16 >= idx15 {
		t.Errorf("headings not in descending order: 17=%d, 16=%d, 15=%d", idx17, idx16, idx15)
	}
}

func TestMergeLogContent_TimestampedAndChangeNoteEntries(t *testing.T) {
	localLog := `# Knowledge Log

## 2026-11-04
* **Update** [2026-11-04T12:20:00Z]: Updated concept ` + "`people/stephan-k.md`" + ` (set stale_after 2027-03-01). — Linear-Team korrigiert
* **Creation**: Documented concept ` + "`legacy/old.md`" + ` (Legacy).
`

	remoteLog := `# Knowledge Log

## 2026-11-04
* **Update** [2026-11-04T12:20:00Z]: Updated concept ` + "`people/stephan-k.md`" + ` (set stale_after 2027-03-01). — Linear-Team korrigiert
* **Relate** [2026-11-04T12:31:00Z]: Linked ` + "`people/stephan-k.md`" + ` to ` + "`processes/bug-handling.md`" + `. — Added triage ownership
`

	merged, err := sync.MergeLogContent([]byte(localLog), []byte(remoteLog))
	if err != nil {
		t.Fatalf("unexpected MergeLogContent error: %v", err)
	}

	mergedStr := string(merged)
	if strings.Count(mergedStr, "## 2026-11-04") != 1 {
		t.Fatalf("expected 1 date heading, got:\n%s", mergedStr)
	}
	if strings.Count(mergedStr, "Linear-Team korrigiert") != 1 {
		t.Errorf("expected timestamped entry with change_note to be deduplicated, got:\n%s", mergedStr)
	}
	if !strings.Contains(mergedStr, "Added triage ownership") {
		t.Errorf("missing remote relate entry with change_note:\n%s", mergedStr)
	}
	if !strings.Contains(mergedStr, "* **Creation**: Documented concept `legacy/old.md` (Legacy).") {
		t.Errorf("missing legacy date-only entry:\n%s", mergedStr)
	}
}
