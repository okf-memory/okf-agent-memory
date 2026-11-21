package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/okf-memory/okf-agent-memory/pkg/registry"
)

// A registry manifest id (and an okf.lock id) names the directory that pull and
// restore install into and that rollback deletes recursively. These tests run
// the real commands against an in-process registry whose manifests are hostile,
// and check that nothing outside <workspace>/.okf/vendor is written or deleted.

const (
	ridValidIndex = "---\nokf_version: \"0.2\"\n---\n# Bundle\n"
	ridValidNote  = "---\ntype: Note\ntitle: Planted\ndescription: Planted by the test registry\n---\nplanted\n"
)

type ridFile struct{ name, body string }

func ridArchive(t *testing.T, files ...ridFile) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for _, f := range files {
		if err := tw.WriteHeader(&tar.Header{Name: f.name, Mode: 0o644, Size: int64(len(f.body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(f.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// ridRegistry serves /bundles/<slug>.json manifests (raw JSON) and
// /<name>.tar.gz archives through the registry client factory.
func ridRegistry(t *testing.T, manifests map[string]string, archives map[string][]byte) {
	t.Helper()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m, ok := manifests[strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/bundles/"), ".json")]; ok && strings.HasPrefix(r.URL.Path, "/bundles/") {
			_, _ = io.WriteString(w, m)
			return
		}
		if a, ok := archives[strings.TrimPrefix(r.URL.Path, "/")]; ok {
			_, _ = w.Write(a)
			return
		}
		http.NotFound(w, r)
	})
	orig := newRegistryClient
	t.Cleanup(func() { newRegistryClient = orig })
	newRegistryClient = func(baseURL string) *registry.Client {
		c := registry.NewClient(baseURL)
		c.HTTPClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			return rec.Result(), nil
		})
		return c
	}
}

// ridWorkspace creates base/ws (the working directory for the test) and
// base/outside, and returns their paths.
func ridWorkspace(t *testing.T) (ws, outside string) {
	t.Helper()
	base := t.TempDir()
	ws, outside = filepath.Join(base, "ws"), filepath.Join(base, "outside")
	for _, d := range []string{ws, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	origDir, _ := os.Getwd()
	if err := os.Chdir(ws); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	return ws, outside
}

// ridRun runs a CLI command and returns its exit code (0 if exitFunc was not
// called) and stderr.
func ridRun(t *testing.T, name string, args ...string) (int, string) {
	t.Helper()
	cmd, ok := FindCommand(name)
	if !ok {
		t.Fatalf("%s command not found", name)
	}
	exitCode := 0
	origExit := exitFunc
	exitFunc = func(code int) {
		if exitCode == 0 {
			exitCode = code
		}
	}
	defer func() { exitFunc = origExit }()

	r, w, _ := os.Pipe()
	origStderr := os.Stderr
	os.Stderr = w
	cmd.Run(args)
	_ = w.Close()
	os.Stderr = origStderr
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return exitCode, buf.String()
}

func ridMustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func ridMustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

// ridAssertOutside fails unless outside holds exactly the wanted relative files
// with their original content.
func ridAssertOutside(t *testing.T, outside string, want map[string]string) {
	t.Helper()
	got := map[string]string{}
	_ = filepath.WalkDir(outside, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(outside, p)
			b, _ := os.ReadFile(p)
			got[filepath.ToSlash(rel)] = string(b)
		}
		return nil
	})
	if len(got) != len(want) {
		t.Fatalf("outside directory changed: got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("outside file %s changed or missing: got %q, want %q (all: %v)", k, got[k], v, got)
		}
	}
}

func ridAssertRefused(t *testing.T, code int, stderr string, wantInMsg ...string) {
	t.Helper()
	if code != 1 {
		t.Fatalf("expected the command to be refused with exit 1, got exit %d (stderr: %q)", code, stderr)
	}
	for _, s := range wantInMsg {
		if !strings.Contains(stderr, s) {
			t.Fatalf("expected stderr to mention %q, got %q", s, stderr)
		}
	}
}

func TestCmdPull_RefusesEscapingManifestID(t *testing.T) {
	cases := []struct{ name, id, reason string }{
		{"parent traversal", "../../../outside/planted", "parent directory traversal"},
		{"nested parent traversal", "acme/../../../../outside/planted", "parent directory traversal"},
		{"absolute", "/outside/planted", "absolute path"},
		{"backslash", `acme\..\..\..\outside\planted`, "backslash"},
		{"empty", "", "empty id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws, outside := ridWorkspace(t)
			ridRegistry(t,
				map[string]string{"plant": `{"id":` + strconv.Quote(tc.id) + `,"version":"1.0.0","download_url":"/valid.tar.gz"}`},
				map[string][]byte{"valid.tar.gz": ridArchive(t, ridFile{"index.md", ridValidIndex}, ridFile{"planted.md", ridValidNote})})

			code, stderr := ridRun(t, "pull", "--registry", "https://registry.test", "plant")

			ridAssertOutside(t, outside, map[string]string{})
			ridAssertRefused(t, code, stderr, "unsafe bundle id "+strconv.Quote(tc.id), tc.reason)
			if _, err := os.Stat(filepath.Join(ws, "okf.lock")); !os.IsNotExist(err) {
				t.Fatalf("okf.lock must not be written for a refused id: %v", err)
			}
			// An absolute id must not be re-rooted into a non-canonical path
			// inside the vendor root either: nothing is installed at all.
			if entries, _ := os.ReadDir(filepath.Join(ws, ".okf", "vendor")); len(entries) != 0 {
				t.Fatalf("nothing may be installed for a refused id, found %d vendor entries", len(entries))
			}
		})
	}
}

func TestCmdPull_ForceRollbackRefusesEscapingID(t *testing.T) {
	_, outside := ridWorkspace(t)
	ridMustWrite(t, filepath.Join(outside, "victim-dir", "keep.md"), "user data")
	ridRegistry(t,
		map[string]string{"wipe": `{"id":"../../../outside/victim-dir","version":"1.0.0","download_url":"/invalid.tar.gz"}`},
		map[string][]byte{"invalid.tar.gz": ridArchive(t, ridFile{"random.txt", "no index.md"})})

	code, stderr := ridRun(t, "pull", "--registry", "https://registry.test", "--force", "wipe")

	ridAssertOutside(t, outside, map[string]string{"victim-dir/keep.md": "user data"})
	ridAssertRefused(t, code, stderr, `unsafe bundle id "../../../outside/victim-dir"`)
}

func TestCmdPull_RefusesInstallThroughVendorSymlink(t *testing.T) {
	ws, outside := ridWorkspace(t)
	ridMustSymlink(t, outside, filepath.Join(ws, ".okf", "vendor", "evil"))
	ridRegistry(t,
		map[string]string{"plant": `{"id":"evil/planted","version":"1.0.0","download_url":"/valid.tar.gz"}`},
		map[string][]byte{"valid.tar.gz": ridArchive(t, ridFile{"index.md", ridValidIndex})})

	code, stderr := ridRun(t, "pull", "--registry", "https://registry.test", "plant")

	ridAssertOutside(t, outside, map[string]string{})
	ridAssertRefused(t, code, stderr, `bundle id "evil/planted"`, "outside")
}

func TestCmdPull_ForceRollbackRefusesDeletingThroughVendorSymlink(t *testing.T) {
	ws, outside := ridWorkspace(t)
	ridMustWrite(t, filepath.Join(outside, "victim-dir", "keep.md"), "user data")
	ridMustSymlink(t, outside, filepath.Join(ws, ".okf", "vendor", "evil"))
	ridRegistry(t,
		map[string]string{"wipe": `{"id":"evil/victim-dir","version":"1.0.0","download_url":"/invalid.tar.gz"}`},
		map[string][]byte{"invalid.tar.gz": ridArchive(t, ridFile{"random.txt", "no index.md"})})

	code, stderr := ridRun(t, "pull", "--registry", "https://registry.test", "--force", "wipe")

	ridAssertOutside(t, outside, map[string]string{"victim-dir/keep.md": "user data"})
	ridAssertRefused(t, code, stderr, `bundle id "evil/victim-dir"`, "outside")
}

func TestCmdPull_RefusesVendorRootOutsideWorkspace(t *testing.T) {
	ws, outside := ridWorkspace(t)
	ridMustSymlink(t, outside, filepath.Join(ws, ".okf"))
	ridRegistry(t,
		map[string]string{"rules": `{"id":"acme/rules","version":"1.0.0","download_url":"/valid.tar.gz"}`},
		map[string][]byte{"valid.tar.gz": ridArchive(t, ridFile{"index.md", ridValidIndex})})

	code, stderr := ridRun(t, "pull", "--registry", "https://registry.test", "rules")

	ridAssertOutside(t, outside, map[string]string{})
	ridAssertRefused(t, code, stderr, "vendor root", "outside workspace")
}

// A --force reinstall extracts into a fresh staging directory and then
// replaces the previous installation, so symlinks left in the old vendor
// directory are never written through. (Extraction into a directory that
// already holds symlinks is refused by DownloadAndExtract itself; see
// pkg/registry.)
func TestCmdPull_ForceReinstallNeverWritesThroughPreexistingSymlink(t *testing.T) {
	t.Run("file symlink", func(t *testing.T) {
		ws, outside := ridWorkspace(t)
		ridMustWrite(t, filepath.Join(outside, "victim.md"), "user data")
		index := filepath.Join(ws, ".okf", "vendor", "acme", "rules", "index.md")
		ridMustSymlink(t, filepath.Join(outside, "victim.md"), index)
		ridRegistry(t,
			map[string]string{"rules": `{"id":"acme/rules","version":"1.0.0","download_url":"/valid.tar.gz"}`},
			map[string][]byte{"valid.tar.gz": ridArchive(t, ridFile{"index.md", ridValidIndex})})

		code, stderr := ridRun(t, "pull", "--registry", "https://registry.test", "--force", "rules")

		ridAssertOutside(t, outside, map[string]string{"victim.md": "user data"})
		if code != 0 {
			t.Fatalf("expected the reinstall to succeed, got exit %d (stderr: %q)", code, stderr)
		}
		if fi, err := os.Lstat(index); err != nil || !fi.Mode().IsRegular() {
			t.Fatalf("index.md must be replaced by a regular file, got %v (err %v)", fi, err)
		}
	})
	t.Run("directory symlink", func(t *testing.T) {
		ws, outside := ridWorkspace(t)
		sub := filepath.Join(ws, ".okf", "vendor", "acme", "rules", "sub")
		ridMustSymlink(t, outside, sub)
		ridRegistry(t,
			map[string]string{"rules": `{"id":"acme/rules","version":"1.0.0","download_url":"/valid.tar.gz"}`},
			map[string][]byte{"valid.tar.gz": ridArchive(t, ridFile{"index.md", ridValidIndex}, ridFile{"sub/planted.md", "---\ntype: Note\ntitle: \"   \"\n---\nplanted\n"})})

		code, stderr := ridRun(t, "pull", "--registry", "https://registry.test", "--force", "rules")

		ridAssertOutside(t, outside, map[string]string{})
		ridAssertRefused(t, code, stderr, `bundle "acme/rules"`, "rolled back")
		// The failed install is discarded; the previous installation (here the
		// user's own symlink) is left exactly as it was.
		if target, err := os.Readlink(sub); err != nil || target != outside {
			t.Fatalf("previous installation must be untouched: readlink=%q err=%v", target, err)
		}
	})
}

func TestCmdRestore_RefusesEscapingLockID(t *testing.T) {
	ws, outside := ridWorkspace(t)
	ridMustWrite(t, filepath.Join(outside, "victim-dir", "keep.md"), "user data")
	ridMustWrite(t, filepath.Join(ws, "okf.lock"), "version: 1\nbundles:\n  - id: \"../../../outside/victim-dir\"\n    source: \"acme/x\"\n    version: \"1.0.0\"\n")
	ridRegistry(t,
		map[string]string{"acme/x": `{"id":"acme/x","version":"1.0.0","download_url":"/invalid.tar.gz"}`},
		map[string][]byte{"invalid.tar.gz": ridArchive(t, ridFile{"random.txt", "no index.md"})})

	code, stderr := ridRun(t, "restore", "--registry", "https://registry.test", "--force")

	ridAssertOutside(t, outside, map[string]string{"victim-dir/keep.md": "user data"})
	ridAssertRefused(t, code, stderr, `unsafe bundle id "../../../outside/victim-dir"`)
}

func TestCmdVendorRemove_RefusesEscapingID(t *testing.T) {
	_, outside := ridWorkspace(t)
	ridMustWrite(t, filepath.Join(outside, "victim-dir", "index.md"), "user data")

	code, stderr := ridRun(t, "vendor", "remove", "../../../outside/victim-dir")

	ridAssertOutside(t, outside, map[string]string{"victim-dir/index.md": "user data"})
	ridAssertRefused(t, code, stderr, `unsafe bundle id "../../../outside/victim-dir"`)
}

// A failed --force reinstall must not cost the user the installation they
// already had: rollback removes only the staging directory this run created.
func TestCmdPull_ForceFailedReinstallKeepsPreviousInstall(t *testing.T) {
	ws, _ := ridWorkspace(t)
	ridRegistry(t,
		map[string]string{
			"rules":     `{"id":"acme/rules","version":"1.0.0","download_url":"/valid.tar.gz"}`,
			"rules-bad": `{"id":"acme/rules","version":"2.0.0","download_url":"/invalid.tar.gz"}`,
		},
		map[string][]byte{
			"valid.tar.gz":   ridArchive(t, ridFile{"index.md", ridValidIndex}),
			"invalid.tar.gz": ridArchive(t, ridFile{"random.txt", "no index.md"}),
		})
	if code, stderr := ridRun(t, "pull", "--registry", "https://registry.test", "rules"); code != 0 {
		t.Fatalf("initial install: exit %d, stderr %q", code, stderr)
	}

	code, stderr := ridRun(t, "pull", "--registry", "https://registry.test", "--force", "rules-bad")

	ridAssertRefused(t, code, stderr, "missing index.md", "rolled back")
	if _, err := os.Stat(filepath.Join(ws, ".okf", "vendor", "acme", "rules", "index.md")); err != nil {
		t.Fatalf("previous installation must survive a failed reinstall: %v", err)
	}
	ridAssertNoStaging(t, ws)
}

func TestCmdRestore_ForceFailedRestoreKeepsPreviousInstall(t *testing.T) {
	ws, _ := ridWorkspace(t)
	ridMustWrite(t, filepath.Join(ws, ".okf", "vendor", "acme", "x", "index.md"), ridValidIndex)
	ridMustWrite(t, filepath.Join(ws, "okf.lock"), "version: 1\nbundles:\n  - id: \"acme/x\"\n    source: \"acme/x\"\n    version: \"1.0.0\"\n")
	ridRegistry(t,
		map[string]string{"acme/x": `{"id":"acme/x","version":"1.0.0","download_url":"/invalid.tar.gz"}`},
		map[string][]byte{"invalid.tar.gz": ridArchive(t, ridFile{"random.txt", "no index.md"})})

	code, stderr := ridRun(t, "restore", "--registry", "https://registry.test", "--force")

	ridAssertRefused(t, code, stderr, "missing index.md", "rolled back")
	if _, err := os.Stat(filepath.Join(ws, ".okf", "vendor", "acme", "x", "index.md")); err != nil {
		t.Fatalf("previous installation must survive a failed restore: %v", err)
	}
	ridAssertNoStaging(t, ws)
}

func ridAssertNoStaging(t *testing.T, ws string) {
	t.Helper()
	entries, _ := os.ReadDir(filepath.Join(ws, ".okf"))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".vendor-") {
			t.Fatalf("staging directory %q left behind", e.Name())
		}
	}
}

// Control: legitimate scoped and nested ids keep installing where they always
// did, and a failing nested install rolls back only its own directory (plus its
// parent once empty).
func TestCmdPull_LegitimateScopedAndNestedIDs(t *testing.T) {
	ws, outside := ridWorkspace(t)
	valid := ridArchive(t, ridFile{"index.md", ridValidIndex})
	ridRegistry(t,
		map[string]string{
			"peter/django-5-rules": `{"id":"@peter/django-5-rules","version":"1.0.0","download_url":"/valid.tar.gz"}`,
			"acme/tools/linter":    `{"id":"acme/tools/linter","version":"1.0.0","download_url":"/valid.tar.gz"}`,
			"acme/tools/broken":    `{"id":"acme/tools/broken","version":"1.0.0","download_url":"/invalid.tar.gz"}`,
			"solo/broken":          `{"id":"solo/broken","version":"1.0.0","download_url":"/invalid.tar.gz"}`,
		},
		map[string][]byte{"valid.tar.gz": valid, "invalid.tar.gz": ridArchive(t, ridFile{"random.txt", "no index.md"})})

	for _, slug := range []string{"@peter/django-5-rules", "acme/tools/linter"} {
		if code, stderr := ridRun(t, "pull", "--registry", "https://registry.test", slug); code != 0 {
			t.Fatalf("pull %s: exit %d, stderr %q", slug, code, stderr)
		}
	}
	for _, p := range []string{"@peter/django-5-rules/index.md", "acme/tools/linter/index.md"} {
		if _, err := os.Stat(filepath.Join(ws, ".okf", "vendor", filepath.FromSlash(p))); err != nil {
			t.Fatalf("expected legitimate install at .okf/vendor/%s: %v", p, err)
		}
	}

	// Failing install next to a sibling: own dir removed, sibling and shared parent kept.
	if code, _ := ridRun(t, "pull", "--registry", "https://registry.test", "acme/tools/broken"); code != 1 {
		t.Fatalf("expected invalid bundle to be rolled back with exit 1, got %d", code)
	}
	if _, err := os.Stat(filepath.Join(ws, ".okf", "vendor", "acme", "tools", "broken")); !os.IsNotExist(err) {
		t.Fatalf("rolled-back bundle dir must be removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws, ".okf", "vendor", "acme", "tools", "linter", "index.md")); err != nil {
		t.Fatalf("sibling bundle must survive rollback: %v", err)
	}

	// Failing install alone in its scope: the now-empty parent is cleaned up.
	if code, _ := ridRun(t, "pull", "--registry", "https://registry.test", "solo/broken"); code != 1 {
		t.Fatalf("expected invalid bundle to be rolled back with exit 1, got %d", code)
	}
	if _, err := os.Stat(filepath.Join(ws, ".okf", "vendor", "solo")); !os.IsNotExist(err) {
		t.Fatalf("empty scope dir must be cleaned up after rollback: %v", err)
	}

	// vendor remove still works for both forms of a scoped id.
	if code, stderr := ridRun(t, "vendor", "remove", "acme/tools/linter"); code != 0 {
		t.Fatalf("vendor remove: exit %d, stderr %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(ws, ".okf", "vendor", "acme", "tools", "linter")); !os.IsNotExist(err) {
		t.Fatalf("vendor remove must delete the bundle: %v", err)
	}
	ridAssertOutside(t, outside, map[string]string{})
}
