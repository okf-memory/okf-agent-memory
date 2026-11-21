package okf_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

func TestRootIndexFrontmatterMetadata(t *testing.T) {
	dir := t.TempDir()
	rootIndex := `---
okf_version: "0.3"
title: Acme retail analytics
description: Tables, metrics, and sanctioned computations for Acme retail.
license: CC-BY-4.0
x-acme-domain: retail
---

# Knowledge Base

* [Orders](orders.md) - Customer orders table
`
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(rootIndex), 0o644); err != nil {
		t.Fatal(err)
	}
	concept := "---\ntype: Table\ntitle: Orders\ndescription: Customer orders table\n---\n\n# Orders\n"
	if err := os.WriteFile(filepath.Join(dir, "orders.md"), []byte(concept), 0o644); err != nil {
		t.Fatal(err)
	}

	b, err := okf.LoadBundle(dir)
	if err != nil {
		t.Fatalf("LoadBundle() failed: %v", err)
	}
	if b.DeclaredVer != "0.3" {
		t.Errorf("DeclaredVer = %q, want %q", b.DeclaredVer, "0.3")
	}
	if b.Title != "Acme retail analytics" {
		t.Errorf("Title = %q, want %q", b.Title, "Acme retail analytics")
	}
	if b.Description != "Tables, metrics, and sanctioned computations for Acme retail." {
		t.Errorf("Description = %q, want expected description", b.Description)
	}
	if b.License != "CC-BY-4.0" {
		t.Errorf("License = %q, want %q", b.License, "CC-BY-4.0")
	}
	if got := b.Extra["x-acme-domain"]; got != "retail" {
		t.Errorf("Extra[x-acme-domain] = %#v, want %q", got, "retail")
	}
	if b.IsV02OrEarlier() {
		t.Errorf("IsV02OrEarlier() = true for okf_version 0.3, want false")
	}
}

func TestBundleIsV02OrEarlier(t *testing.T) {
	var nilBundle *okf.Bundle
	if !nilBundle.IsV02OrEarlier() {
		t.Errorf("nil Bundle.IsV02OrEarlier() = false, want true")
	}

	tests := []struct {
		ver  string
		want bool
	}{
		{"", true},
		{"  ", true},
		{"0.1", true},
		{"0.2", true},
		{"v0.2", true},
		{"0.2.1", true},
		{"0.3", false},
		{"v0.3", false},
		{"0.4", false},
		{"1.0", false},
		{"2.1", false},
		{"invalid", false},
	}
	for _, tc := range tests {
		b := &okf.Bundle{DeclaredVer: tc.ver}
		if got := b.IsV02OrEarlier(); got != tc.want {
			t.Errorf("Bundle{DeclaredVer: %q}.IsV02OrEarlier() = %v, want %v", tc.ver, got, tc.want)
		}
	}
}

func TestRootIndexFrontmatterPreservedOnWritePaths(t *testing.T) {
	dir := t.TempDir()
	// Include a parenthetical matching the concept filename inside frontmatter description
	// to verify UpdateParentIndex never mutates lines inside the frontmatter block.
	fmBlock := `---
okf_version: "0.3"
title: Acme Knowledge
description: Core bundle (orders.md) for Acme systems.
license: Apache-2.0
scopes: [project]
x-acme-owner: team:platform
# Comment preserved verbatim
---
`
	initialIndex := fmBlock + "\n# Knowledge Base\n"
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(initialIndex), 0o644); err != nil {
		t.Fatal(err)
	}

	// 1. InitBundle must preserve existing root index.md frontmatter verbatim.
	if err := okf.InitBundle(dir); err != nil {
		t.Fatalf("InitBundle() failed: %v", err)
	}

	// 2. SaveConcept (create with AutoIndex) must preserve frontmatter verbatim.
	c := &okf.Concept{
		ID:          "orders",
		Path:        "orders.md",
		Type:        "Table",
		Title:       "Orders",
		Description: "Initial description.",
		Body:        "# Orders\n",
	}
	if err := okf.SaveConcept(dir, c, okf.SaveOptions{
		IsNew:     true,
		AutoIndex: true,
		AutoLog:   true,
		Actor:     "agent/test",
	}); err != nil {
		t.Fatalf("SaveConcept(create) failed: %v", err)
	}

	afterCreate, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(afterCreate), fmBlock) {
		t.Fatalf("SaveConcept altered root index.md frontmatter:\nGOT:\n%s\nWANT PREFIX:\n%s", string(afterCreate), fmBlock)
	}
	if !strings.Contains(string(afterCreate), "* [Orders](orders.md) - Initial description.") {
		t.Fatalf("SaveConcept did not add listing to body:\n%s", string(afterCreate))
	}

	// 3. UpdateConcept (update with AutoIndex) must update the body listing while preserving frontmatter verbatim.
	if _, err := okf.UpdateConcept(dir, "orders", okf.SaveOptions{
		AutoIndex: true,
		AutoLog:   true,
		Actor:     "agent/test",
	}, func(u *okf.Concept) error {
		u.Description = "Updated description."
		return nil
	}); err != nil {
		t.Fatalf("UpdateConcept() failed: %v", err)
	}

	afterUpdate, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(afterUpdate), fmBlock) {
		t.Fatalf("UpdateConcept altered root index.md frontmatter:\nGOT:\n%s\nWANT PREFIX:\n%s", string(afterUpdate), fmBlock)
	}
	if !strings.Contains(string(afterUpdate), "* [Orders](orders.md) - Updated description.") {
		t.Fatalf("UpdateConcept did not update listing in body:\n%s", string(afterUpdate))
	}

	// 4. Reload and confirm metadata survived intact.
	b, err := okf.LoadBundle(dir)
	if err != nil {
		t.Fatalf("LoadBundle() after writes failed: %v", err)
	}
	if b.DeclaredVer != "0.3" || b.Title != "Acme Knowledge" || b.License != "Apache-2.0" {
		t.Fatalf("reloaded bundle metadata drifted: ver=%q title=%q license=%q", b.DeclaredVer, b.Title, b.License)
	}
	if b.Description != "Core bundle (orders.md) for Acme systems." {
		t.Fatalf("reloaded bundle description drifted: %q", b.Description)
	}
}

func TestBundleWithoutIndexFrontmatterUnchanged(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte("# Plain Bundle\n\n* [Fact](fact.md) - A fact\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fact.md"), []byte("---\ntype: Fact\ntitle: Fact\ndescription: A fact\n---\n\n# Fact\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	b, err := okf.LoadBundle(dir)
	if err != nil {
		t.Fatalf("LoadBundle() failed: %v", err)
	}
	if b.DeclaredVer != "" || b.Title != "" || b.Description != "" || b.License != "" {
		t.Fatalf("expected empty metadata for frontmatter-less index.md, got ver=%q title=%q desc=%q lic=%q",
			b.DeclaredVer, b.Title, b.Description, b.License)
	}
	if !b.IsV02OrEarlier() {
		t.Errorf("bundle without okf_version should report IsV02OrEarlier() == true")
	}

	// Updating a concept must still work cleanly without adding unexpected frontmatter.
	if _, err := okf.UpdateConcept(dir, "fact", okf.SaveOptions{
		AutoIndex: true,
		Actor:     "agent/test",
	}, func(u *okf.Concept) error {
		u.Description = "Updated fact"
		return nil
	}); err != nil {
		t.Fatalf("UpdateConcept() failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(string(data), "---") {
		t.Fatalf("index.md without frontmatter should not gain frontmatter on update:\n%s", string(data))
	}
	if !strings.Contains(string(data), "* [Fact](fact.md) - Updated fact") {
		t.Fatalf("index.md listing was not updated:\n%s", string(data))
	}
}
