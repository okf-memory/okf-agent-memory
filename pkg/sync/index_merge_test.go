package sync

import (
	"strings"
	"testing"
)

const indexBase = `# Decisions

Architecture decisions for the project.

* [Seed](seed.md) - Seed summary.
* [Old](old.md) - Old summary.
`

func mergeIndex(t *testing.T, base, local, remote string) (string, bool) {
	t.Helper()
	var b []byte
	if base != "" {
		b = []byte(base)
	}
	out, ok := MergeIndexContent(b, []byte(local), []byte(remote))
	return string(out), ok
}

func TestMergeIndex_DisjointAdditionsBothKept(t *testing.T) {
	local := indexBase + "* [Alpha](alpha.md) - Alpha summary.\n"
	remote := indexBase + "* [Beta](beta.md) - Beta summary.\n"

	got, ok := mergeIndex(t, indexBase, local, remote)
	if !ok {
		t.Fatal("expected a clean merge")
	}
	for _, want := range []string{"(seed.md)", "(old.md)", "(alpha.md)", "(beta.md)", "Architecture decisions for the project."} {
		if !strings.Contains(got, want) {
			t.Errorf("merged index missing %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "(seed.md)") != 1 {
		t.Errorf("seed listed more than once:\n%s", got)
	}
}

func TestMergeIndex_DeletionOnOneSideIsRespected(t *testing.T) {
	local := strings.Replace(indexBase, "* [Old](old.md) - Old summary.\n", "", 1)
	remote := indexBase + "* [Beta](beta.md) - Beta summary.\n"

	got, ok := mergeIndex(t, indexBase, local, remote)
	if !ok || strings.Contains(got, "(old.md)") || !strings.Contains(got, "(beta.md)") {
		t.Fatalf("expected old.md removed and beta.md added (ok=%v):\n%s", ok, got)
	}
}

func TestMergeIndex_EditBeatsDeletion(t *testing.T) {
	local := strings.Replace(indexBase, "Old summary.", "Old summary, revised.", 1)
	remote := strings.Replace(indexBase, "* [Old](old.md) - Old summary.\n", "", 1)

	got, ok := mergeIndex(t, indexBase, local, remote)
	if !ok || !strings.Contains(got, "Old summary, revised.") {
		t.Fatalf("an edited listing must not be lost to a concurrent delete (ok=%v):\n%s", ok, got)
	}
}

func TestMergeIndex_OneSidedUpdateWins(t *testing.T) {
	local := strings.Replace(indexBase, "Seed summary.", "Seed summary, updated.", 1)

	got, ok := mergeIndex(t, indexBase, local, indexBase)
	if !ok || !strings.Contains(got, "Seed summary, updated.") {
		t.Fatalf("expected the local update (ok=%v):\n%s", ok, got)
	}
}

func TestMergeIndex_ConcurrentUpdatesPreferRemote(t *testing.T) {
	local := strings.Replace(indexBase, "Seed summary.", "Seed from local.", 1)
	remote := strings.Replace(indexBase, "Seed summary.", "Seed from remote.", 1)

	got, ok := mergeIndex(t, indexBase, local, remote)
	if !ok || !strings.Contains(got, "Seed from remote.") || strings.Contains(got, "Seed from local.") {
		t.Fatalf("expected remote to win a same-listing collision (ok=%v):\n%s", ok, got)
	}
}

func TestMergeIndex_OneSidedProseChangeKept(t *testing.T) {
	local := strings.Replace(indexBase, "Architecture decisions for the project.", "Decisions, newest first.", 1)
	remote := indexBase + "* [Beta](beta.md) - Beta summary.\n"

	got, ok := mergeIndex(t, indexBase, local, remote)
	if !ok || !strings.Contains(got, "Decisions, newest first.") || !strings.Contains(got, "(beta.md)") {
		t.Fatalf("expected local prose plus remote listing (ok=%v):\n%s", ok, got)
	}
}

func TestMergeIndex_ConflictingProseFallsBack(t *testing.T) {
	local := strings.Replace(indexBase, "Architecture decisions for the project.", "Local intro.", 1)
	remote := strings.Replace(indexBase, "Architecture decisions for the project.", "Remote intro.", 1)

	if _, ok := mergeIndex(t, indexBase, local, remote); ok {
		t.Fatal("conflicting prose edits must fall back to the collision failsafe")
	}
}

func TestMergeIndex_NoBaseUnionsListings(t *testing.T) {
	local := "# Decisions\n\n* [Alpha](alpha.md) - A.\n"
	remote := "# Decisions\n\n* [Beta](beta.md) - B.\n"

	got, ok := mergeIndex(t, "", local, remote)
	if !ok || !strings.Contains(got, "(alpha.md)") || !strings.Contains(got, "(beta.md)") {
		t.Fatalf("expected a union when both sides created the index (ok=%v):\n%s", ok, got)
	}
}
