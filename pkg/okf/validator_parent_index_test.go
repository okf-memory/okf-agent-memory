package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateParentIndexLinkVariants(t *testing.T) {
	bundle := t.TempDir()

	if err := os.WriteFile(filepath.Join(bundle, "index.md"), []byte("---\nokf_version: \"0.2\"\n---\n# Root\n* [Decisions](decisions/index.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "log.md"), []byte("# Log\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	decDir := filepath.Join(bundle, "decisions")
	if err := os.MkdirAll(decDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// decisions/index.md with bundle-absolute, dot-relative, bare, and anchor hrefs
	decIndex := `# Decisions

* [Absolute](/decisions/absolute.md) - Concept absolute.
* [Dot Relative](./dot-relative.md) - Concept dot relative.
* [Plain](plain.md) - Concept plain.
* [Anchored](anchored.md#architecture) - Concept anchored.
`
	if err := os.WriteFile(filepath.Join(decDir, "index.md"), []byte(decIndex), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"absolute", "dot-relative", "plain", "anchored", "unlisted"} {
		content := "---\ntype: Fact\ntitle: " + name + "\ndescription: Concept " + strings.ReplaceAll(name, "-", " ") + ".\n---\n# Body\n"
		if err := os.WriteFile(filepath.Join(decDir, name+".md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	b, err := LoadBundle(bundle)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}

	res := Validate(b, ValidateOptions{Strict: true, Drift: true})

	var unlistedWarnings []string
	for _, w := range res.Warnings {
		if strings.Contains(w, "concept is not listed in parent index") {
			unlistedWarnings = append(unlistedWarnings, w)
		}
	}

	// Only decisions/unlisted.md should produce a warning
	if len(unlistedWarnings) != 1 {
		t.Fatalf("expected exactly 1 unlisted warning, got %d: %v", len(unlistedWarnings), unlistedWarnings)
	}
	if !strings.Contains(unlistedWarnings[0], "decisions/unlisted.md") {
		t.Errorf("expected warning for decisions/unlisted.md, got: %s", unlistedWarnings[0])
	}
}
