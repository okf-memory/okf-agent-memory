package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

func mutationTestBundle(t *testing.T) string {
	t.Helper()
	bundle := t.TempDir()
	if err := os.WriteFile(filepath.Join(bundle, "index.md"), []byte("---\nokf_version: \"0.2\"\n---\n# Bundle\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "log.md"), []byte("# Log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return bundle
}

func TestCLIMutationMetadata(t *testing.T) {
	bundle := mutationTestBundle(t)
	cmdCreate([]string{"item", bundle, "--type", "Fact", "--status", "draft", "--desc", "original", "--tags", " first, second "})
	cmdUpdate([]string{"item", bundle, "--type", "Decision", "--status", "deprecated", "--tags", " next, final "})
	b, err := okf.LoadBundle(bundle)
	if err != nil {
		t.Fatal(err)
	}
	c := b.Concepts["item"]
	if c.Type != "Decision" || c.Status != "deprecated" || c.Description != "original" || !reflect.DeepEqual(c.Tags, []string{"next", "final"}) {
		t.Fatalf("unexpected updated metadata: %+v", c)
	}
	cmdUpdate([]string{"item", bundle, "--tags", ""})
	b, err = okf.LoadBundle(bundle)
	if err != nil {
		t.Fatal(err)
	}
	c = b.Concepts["item"]
	if len(c.Tags) != 0 || c.Type != "Decision" || c.Status != "deprecated" {
		t.Fatalf("clearing tags altered other fields: %+v", c)
	}
	cmdCreate([]string{"default-status", bundle, "--type", "Fact"})
	b, err = okf.LoadBundle(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if b.Concepts["default-status"].Status != "stable" {
		t.Fatalf("omitted create status = %q, want stable", b.Concepts["default-status"].Status)
	}
}

func TestCLIMutationInvalidStatus(t *testing.T) {
	if mode := os.Getenv("OKF_TEST_INVALID_MUTATION"); mode != "" {
		bundle := os.Getenv("OKF_TEST_MUTATION_BUNDLE")
		if mode == "create" {
			cmdCreate([]string{"invalid", bundle, "--status", "active"})
		} else {
			cmdUpdate([]string{"item", bundle, "--status", "active"})
		}
		return
	}
	bundle := mutationTestBundle(t)
	cmdCreate([]string{"item", bundle, "--status", "stable"})
	before, err := os.ReadFile(filepath.Join(bundle, "item.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"create", "update"} {
		t.Run(mode, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestCLIMutationInvalidStatus$")
			cmd.Env = append(os.Environ(), "OKF_TEST_INVALID_MUTATION="+mode, "OKF_TEST_MUTATION_BUNDLE="+bundle)
			output, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(output), "Invalid status") {
				t.Fatalf("expected status error, got %v: %s", err, output)
			}
		})
	}
	if _, err := os.Stat(filepath.Join(bundle, "invalid.md")); !os.IsNotExist(err) {
		t.Fatalf("invalid create wrote file: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(bundle, "item.md"))
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("invalid update modified concept: %v", err)
	}
}

// Issue #49: custom list-valued frontmatter fields are lists when read and stay
// lists after `okf update`, as tags do.
func TestCLIUpdateKeepsCustomListFields(t *testing.T) {
	bundle := mutationTestBundle(t)
	concept := "---\ntype: Fact\ntitle: Item\ntags: [git]\ncommands:\n  - git commit\n  - \"git push\"\nrepos: [contextopia, easygov]\n---\n# Item\n"
	if err := os.WriteFile(filepath.Join(bundle, "item.md"), []byte(concept), 0o644); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"commands": []any{"git commit", "git push"},
		"repos":    []any{"contextopia", "easygov"},
	}
	check := func(when string) string {
		t.Helper()
		b, err := okf.LoadBundle(bundle)
		if err != nil {
			t.Fatal(err)
		}
		c := b.Concepts["item"]
		for key, value := range want {
			if !reflect.DeepEqual(c.Extra[key], value) {
				t.Fatalf("%s: extra.%s = %#v, want %#v", when, key, c.Extra[key], value)
			}
		}
		if !reflect.DeepEqual(c.Tags, []string{"git"}) {
			t.Fatalf("%s: tags = %#v", when, c.Tags)
		}
		raw, err := os.ReadFile(filepath.Join(bundle, "item.md"))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}

	check("before update")
	cmdUpdate([]string{"item", bundle, "--desc", "updated"})
	written := check("after update")
	if strings.Contains(written, `"[`) {
		t.Fatalf("update wrote a list as a quoted string:\n%s", written)
	}
	cmdUpdate([]string{"item", bundle, "--desc", "updated again"})
	if again := check("after a second update"); again != strings.Replace(written, "updated", "updated again", 1) {
		t.Fatalf("custom lists drifted across updates:\nfirst:\n%s\nsecond:\n%s", written, again)
	}
}
