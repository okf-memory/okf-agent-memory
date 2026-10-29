package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Regression test for the three-writer conflict fork collision: three devices
// edit the same concept from the same base and sync in turn. Each conflicting
// sync must preserve its local edit in a fork that no later conflicting sync
// can overwrite, so every committed edit stays in the hub's current tree.

func appendLine(t *testing.T, dir, rel, line string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(dir, filepath.FromSlash(rel)), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("append %s: %v", rel, err)
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		t.Fatalf("append %s: %v", rel, err)
	}
}

func treeContains(t *testing.T, dir, substr string) bool {
	t.Helper()
	found := false
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}
		data, rerr := os.ReadFile(p)
		if rerr == nil && strings.Contains(string(data), substr) {
			found = true
		}
		return nil
	})
	return found
}

func TestHub_ThreeWritersKeepEveryCommittedEdit(t *testing.T) {
	f, dirA, dirB := twoDevicesFromSeed(t, "decisions/seed")
	dirC := t.TempDir()
	f.pull(dirC)

	appendLine(t, dirA, "decisions/seed.md", "edit-from-A")
	appendLine(t, dirB, "decisions/seed.md", "edit-from-B")
	appendLine(t, dirC, "decisions/seed.md", "edit-from-C")

	f.sync(dirA, "A edits seed")
	f.sync(dirB, "B edits seed")
	f.sync(dirC, "C edits seed")

	dirD := t.TempDir()
	f.pull(dirD)

	if !treeContains(t, dirD, "edit-from-A") {
		t.Errorf("device A's committed edit \"edit-from-A\" is missing from the hub's current tree")
	}
	if !treeContains(t, dirD, "edit-from-B") {
		conflicts := conflictFiles(t, dirD)
		t.Errorf("device B's committed edit \"edit-from-B\" is missing from the hub's current tree (conflict forks: %v)", conflicts)
	}
	if !treeContains(t, dirD, "edit-from-C") {
		conflicts := conflictFiles(t, dirD)
		t.Errorf("device C's committed edit \"edit-from-C\" is missing from the hub's current tree (conflict forks: %v)", conflicts)
	}
}

// Control: with only two conflicting writers the fixed fork path is written
// once and both edits survive; this isolates the third writer as the cause.
func TestHub_TwoWritersKeepEveryCommittedEdit(t *testing.T) {
	f, dirA, dirB := twoDevicesFromSeed(t, "decisions/seed")

	appendLine(t, dirA, "decisions/seed.md", "edit-from-A")
	appendLine(t, dirB, "decisions/seed.md", "edit-from-B")

	f.sync(dirA, "A edits seed")
	f.sync(dirB, "B edits seed")

	dirD := t.TempDir()
	f.pull(dirD)

	if !treeContains(t, dirD, "edit-from-A") {
		t.Errorf("device A's committed edit \"edit-from-A\" is missing from the hub's current tree")
	}
	if !treeContains(t, dirD, "edit-from-B") {
		conflicts := conflictFiles(t, dirD)
		t.Errorf("device B's committed edit \"edit-from-B\" is missing from the hub's current tree (conflict forks: %v)", conflicts)
	}
}
