package sync

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
	"github.com/okf-memory/okf-agent-memory/pkg/vault"
)

// Two devices (or agents) that create *different* concepts touch no common
// concept file, but `okf create` also rewrites the parent folder's index.md.
// The reconcile engine auto-merges only log.md, so a shared index.md becomes a
// same-file collision even though the work is disjoint.

// hubFixture drives HubPush/HubPull/HubSync the way the CLI does: every call
// builds a fresh Engine, as a new `okf hub ...` process would, so sync state
// must survive between calls via the persisted state file.
type hubFixture struct {
	t        *testing.T
	server   *Server
	vaultID  string
	password string
	secret   string
}

func newHubFixture(t *testing.T) *hubFixture {
	t.Helper()
	secret, err := vault.GenerateSecretKey()
	if err != nil {
		t.Fatalf("GenerateSecretKey: %v", err)
	}
	return &hubFixture{
		t:        t,
		server:   NewServer(""),
		vaultID:  "test-vault-index-merge",
		password: "master-pass-123",
		secret:   secret,
	}
}

func (f *hubFixture) op(dir, message string) HubOp {
	return HubOp{
		Dir:          dir,
		Client:       newTestClientFromHandlerForHub(f.server.Handler(), "token"),
		VaultID:      f.vaultID,
		Password:     f.password,
		SecretKey:    f.secret,
		Message:      message,
		AgentVersion: "test",
	}
}

func (f *hubFixture) push(dir, message string) {
	f.t.Helper()
	var out bytes.Buffer
	if err := HubPush(&out, f.op(dir, message)); err != nil {
		f.t.Fatalf("HubPush(%s): %v", message, err)
	}
}

func (f *hubFixture) pull(dir string) {
	f.t.Helper()
	var out bytes.Buffer
	if err := HubPull(&out, f.op(dir, "")); err != nil {
		f.t.Fatalf("HubPull: %v", err)
	}
}

func (f *hubFixture) sync(dir, message string) {
	f.t.Helper()
	var out bytes.Buffer
	if err := HubSync(&out, f.op(dir, message)); err != nil {
		f.t.Fatalf("HubSync(%s): %v", message, err)
	}
}

func readFile(t *testing.T, dir, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func conflictFiles(t *testing.T, dir string) []string {
	t.Helper()
	var found []string
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && strings.Contains(info.Name(), ".conflict-local") {
			rel, _ := filepath.Rel(dir, p)
			found = append(found, filepath.ToSlash(rel))
		}
		return nil
	})
	return found
}

// createConcept mirrors `okf create`: write the concept and run automatic
// index and log bookkeeping.
func createConcept(t *testing.T, dir, id, title string) {
	t.Helper()
	c := &okf.Concept{
		ID:          id,
		Path:        id + ".md",
		Type:        "Decision",
		Title:       title,
		Description: title + " summary.",
		Body:        "# " + title + "\n",
	}
	if err := okf.SaveConcept(dir, c, okf.SaveOptions{IsNew: true, AutoLog: true, AutoIndex: true}); err != nil {
		t.Fatalf("SaveConcept(%s): %v", id, err)
	}
}

// twoDevicesFromSeed returns two synced bundles that both contain the seed concepts.
func twoDevicesFromSeed(t *testing.T, seeds ...string) (*hubFixture, string, string) {
	t.Helper()
	f := newHubFixture(t)
	dirA := t.TempDir()
	if err := okf.InitBundle(dirA); err != nil {
		t.Fatalf("InitBundle: %v", err)
	}
	for _, id := range seeds {
		createConcept(t, dirA, id, "Seed "+id)
	}
	f.push(dirA, "seed")

	dirB := t.TempDir()
	f.pull(dirB)
	return f, dirA, dirB
}

func assertConverged(t *testing.T, dirs map[string]string, indexPath string, wantInIndex []string) {
	t.Helper()
	for name, dir := range dirs {
		if c := conflictFiles(t, dir); len(c) != 0 {
			t.Errorf("%s: disjoint creates should not conflict, found %v", name, c)
		}
		index := readFile(t, dir, indexPath)
		for _, want := range wantInIndex {
			if !strings.Contains(index, want) {
				t.Errorf("%s: %s is missing %q:\n%s", name, indexPath, want, index)
			}
		}
	}
}

// Fails today: both creates rewrite decisions/index.md.
func TestHub_DisjointCreatesInSameFolderDoNotConflict(t *testing.T) {
	f, dirA, dirB := twoDevicesFromSeed(t, "decisions/seed")

	createConcept(t, dirA, "decisions/alpha", "Alpha")
	createConcept(t, dirB, "decisions/beta", "Beta")
	f.sync(dirA, "A creates alpha")
	f.sync(dirB, "B creates beta")
	f.sync(dirA, "A catches up")

	assertConverged(t, map[string]string{"A": dirA, "B": dirB}, "decisions/index.md",
		[]string{"alpha.md", "beta.md", "seed.md"})
}

// Control: the same disjoint work in different folders shares no index file
// and merges cleanly today, isolating the shared index.md as the cause above.
func TestHub_DisjointCreatesInDifferentFoldersDoNotConflict(t *testing.T) {
	f, dirA, dirB := twoDevicesFromSeed(t, "decisions/seed", "facts/seed")

	createConcept(t, dirA, "decisions/alpha", "Alpha")
	createConcept(t, dirB, "facts/beta", "Beta")
	f.sync(dirA, "A creates alpha")
	f.sync(dirB, "B creates beta")
	f.sync(dirA, "A catches up")

	dirs := map[string]string{"A": dirA, "B": dirB}
	assertConverged(t, dirs, "decisions/index.md", []string{"alpha.md"})
	assertConverged(t, dirs, "facts/index.md", []string{"beta.md"})
}
