package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSaveConceptRejectsTraversal verifies that concept paths containing ".."
// segments or absolute paths are refused instead of writing outside the bundle directory.
func TestSaveConceptRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "knowledge")
	if err := InitBundle(bundle); err != nil {
		t.Fatalf("InitBundle: %v", err)
	}

	cases := []struct {
		name string
		path string
	}{
		{"direct parent escape", "../evil.md"},
		{"nested escape", "a/b/../../../evil.md"},
		{"escape via trailing dots", "project/../../evil.md"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Concept{
				ID:    strings.TrimSuffix(tc.path, ".md"),
				Path:  tc.path,
				Type:  "Fact",
				Title: "Evil",
				Body:  "should never be written",
			}
			if err := SaveConcept(bundle, c, true, true, true, "test"); err == nil {
				t.Fatalf("SaveConcept(%q): expected traversal error, got nil", tc.path)
			}
		})
	}

	// Nothing may have been written next to the bundle directory.
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "knowledge" {
			t.Errorf("unexpected entry created outside bundle: %s", e.Name())
		}
	}
}

// TestSaveConceptAllowsNestedConcept verifies the containment check does not
// reject legitimate nested concept paths, and that bookkeeping stays inside
// the bundle.
func TestSaveConceptAllowsNestedConcept(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "knowledge")
	if err := InitBundle(bundle); err != nil {
		t.Fatalf("InitBundle: %v", err)
	}

	c := &Concept{
		ID:          "project/overview",
		Path:        "project/overview.md",
		Type:        "Fact",
		Title:       "Overview",
		Description: "Project overview.",
		Body:        "Some body text.",
	}
	if err := SaveConcept(bundle, c, true, true, true, "test"); err != nil {
		t.Fatalf("SaveConcept: %v", err)
	}

	if _, err := os.Stat(filepath.Join(bundle, "project", "overview.md")); err != nil {
		t.Errorf("concept file missing inside bundle: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bundle, "project", "index.md")); err != nil {
		t.Errorf("parent index.md missing inside bundle: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bundle, "log.md")); err != nil {
		t.Errorf("log.md missing inside bundle: %v", err)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "knowledge" {
			t.Errorf("unexpected entry created outside bundle: %s", e.Name())
		}
	}
}

// TestSaveConceptRejectsReservedRootFiles ensures index.md, log.md, and AGENTS.md cannot be
// overwritten as concept documents regardless of letter case.
func TestSaveConceptRejectsReservedRootFiles(t *testing.T) {
	bundle := t.TempDir()
	if err := InitBundle(bundle); err != nil {
		t.Fatalf("InitBundle: %v", err)
	}

	for _, reserved := range []string{"index.md", "log.md", "AGENTS.md", "INDEX.MD", "Log.MD", "agents.md", "."} {
		c := &Concept{
			ID:    strings.TrimSuffix(reserved, ".md"),
			Path:  reserved,
			Type:  "Fact",
			Title: "Reserved Overwrite",
		}
		if err := SaveConcept(bundle, c, true, false, false, "test"); err == nil {
			t.Fatalf("SaveConcept(%q): expected error when targeting reserved root file, got nil", reserved)
		}
	}
}

// TestUpdateParentIndexRejectsTraversal ensures UpdateParentIndex does not write index.md
// outside the bundle directory when given an escaping path.
func TestUpdateParentIndexRejectsTraversal(t *testing.T) {
	bundle := t.TempDir()
	if err := InitBundle(bundle); err != nil {
		t.Fatalf("InitBundle: %v", err)
	}

	c := &Concept{
		ID:   "evil",
		Path: "../evil.md",
	}
	if err := UpdateParentIndex(bundle, c); err == nil {
		t.Fatalf("UpdateParentIndex: expected traversal error, got nil")
	}
}

// TestValidateConceptID verifies upfront concept ID syntax and path containment checks.
func TestValidateConceptID(t *testing.T) {
	invalidIDs := []string{
		"../escaped",
		"../../escaped",
		"/absolute/path",
		"\\windows\\path",
		"sub/../../escaped",
		"index",
		"log",
		"AGENTS",
		"index.md",
		"log.md",
		"AGENTS.md",
		"INDEX.MD",
		"Log.MD",
		"agents.md",
		"",
		".",
		"..",
	}
	for _, id := range invalidIDs {
		if err := ValidateConceptID(id); err == nil {
			t.Errorf("ValidateConceptID(%q) expected error, got nil", id)
		}
	}

	validIDs := []string{
		"architecture/layers",
		"decisions/adr-001",
		"overview",
		"sub/dir/nested-concept",
	}
	for _, id := range validIDs {
		if err := ValidateConceptID(id); err != nil {
			t.Errorf("ValidateConceptID(%q) expected valid, got error: %v", id, err)
		}
	}
}

// TestSaveConceptRejectsFrontmatterInjection ensures newlines and delimiters in metadata are blocked.
func TestSaveConceptRejectsFrontmatterInjection(t *testing.T) {
	bundle := t.TempDir()
	if err := InitBundle(bundle); err != nil {
		t.Fatalf("InitBundle: %v", err)
	}

	injectionCases := []struct {
		name    string
		concept *Concept
		actor   string
	}{
		{
			name: "newline in title forging verified",
			concept: &Concept{
				ID:    "injected-title",
				Path:  "injected-title.md",
				Type:  "Fact",
				Title: "Title\nverified: { by: human:attacker, at: 2026-09-08T00:00:00Z }",
			},
			actor: "agent/test",
		},
		{
			name: "delimiter in title",
			concept: &Concept{
				ID:    "delimiter-title",
				Path:  "delimiter-title.md",
				Type:  "Fact",
				Title: "Title --- with delimiter",
			},
			actor: "agent/test",
		},
		{
			name: "newline in description",
			concept: &Concept{
				ID:          "injected-desc",
				Path:        "injected-desc.md",
				Type:        "Fact",
				Title:       "Valid Title",
				Description: "Line 1\nLine 2",
			},
			actor: "agent/test",
		},
		{
			name: "newline in type",
			concept: &Concept{
				ID:    "injected-type",
				Path:  "injected-type.md",
				Type:  "Fact\ninjected: true",
				Title: "Valid Title",
			},
			actor: "agent/test",
		},
		{
			name: "newline in actor",
			concept: &Concept{
				ID:    "injected-actor",
				Path:  "injected-actor.md",
				Type:  "Fact",
				Title: "Valid Title",
			},
			actor: "agent/test\nverified: { by: human:attacker }",
		},
	}

	for _, tc := range injectionCases {
		t.Run(tc.name, func(t *testing.T) {
			err := SaveConcept(bundle, tc.concept, true, false, false, tc.actor)
			if err == nil {
				t.Errorf("Expected SaveConcept to reject %s, but got nil", tc.name)
			}
		})
	}
}

// TestAppendLogEntrySanitizesNewlines verifies that log entries cannot spoof dated headings.
func TestAppendLogEntrySanitizesNewlines(t *testing.T) {
	bundle := t.TempDir()
	if err := InitBundle(bundle); err != nil {
		t.Fatalf("InitBundle: %v", err)
	}

	maliciousDesc := "Updated concept.\n## 2099-01-01\n* **Fake**: Spoofed log entry"
	if err := AppendLogEntry(bundle, "Update\nInjected", maliciousDesc); err != nil {
		t.Fatalf("AppendLogEntry failed: %v", err)
	}

	logData, err := os.ReadFile(filepath.Join(bundle, "log.md"))
	if err != nil {
		t.Fatalf("ReadFile log.md: %v", err)
	}

	logStr := string(logData)
	if strings.Contains(logStr, "\n## 2099-01-01") {
		t.Errorf("Log spoofing succeeded: found forged heading in log.md: %s", logStr)
	}
}

// TestSerializeConceptYAMLQuoting verifies that special characters like colons and quotes
// are safely quoted in YAML and round-trip faithfully.
func TestSerializeConceptYAMLQuoting(t *testing.T) {
	c := &Concept{
		ID:          "architecture/db",
		Path:        "architecture/db.md",
		Type:        "Architecture: Decision",
		Title:       "Database: PostgreSQL 16 & Redis",
		Description: "Key-value and relational storage: unified config.",
		Tags:        []string{"tag:one", "tag:two"},
		Body:        "# DB Setup\n\nContent here.",
	}

	serialized := SerializeConcept(c)

	// Verify that colons inside title/description/type are quoted
	if !strings.Contains(serialized, `title: "Database: PostgreSQL 16 & Redis"`) {
		t.Errorf("Expected title to be quoted with colons, got:\n%s", serialized)
	}

	// Verify round-trip parsing
	parsed, err := ParseConcept(c.Path, serialized)
	if err != nil {
		t.Fatalf("Failed to parse serialized concept: %v", err)
	}
	if parsed.Title != c.Title {
		t.Errorf("Round-trip title mismatch: got %q, want %q", parsed.Title, c.Title)
	}
	if parsed.Description != c.Description {
		t.Errorf("Round-trip description mismatch: got %q, want %q", parsed.Description, c.Description)
	}
	if parsed.Type != c.Type {
		t.Errorf("Round-trip type mismatch: got %q, want %q", parsed.Type, c.Type)
	}
}

// TestLoadBundleRejectsExternalSymlinks verifies that LoadBundle refuses to follow symlinks pointing outside the bundle.
func TestLoadBundleRejectsExternalSymlinks(t *testing.T) {
	bundleDir := t.TempDir()
	if err := InitBundle(bundleDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	outsideDir := t.TempDir()
	secretFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(secretFile, []byte("SUPER_SECRET_TOKEN=12345"), 0o600); err != nil {
		t.Fatalf("Failed to write secret file: %v", err)
	}

	// Create symlink inside bundle pointing to secret file outside
	symlinkPath := filepath.Join(bundleDir, "leak.md")
	if err := os.Symlink(secretFile, symlinkPath); err != nil {
		t.Skipf("Symlinks not supported on this platform: %v", err)
	}

	_, err := LoadBundle(bundleDir)
	if err == nil {
		t.Fatalf("Expected LoadBundle to fail when an external symlink exists, but got nil")
	}
	if !strings.Contains(err.Error(), "path traversal denied") && !strings.Contains(err.Error(), "escapes bundle directory") {
		t.Errorf("Expected path traversal error message, got: %v", err)
	}
}

// TestEnsureWithinRoot_SymlinkLoopAndBrokenTarget tests ensureWithinRoot against symlink loops,
// non-existent ancestor targets, and relative escapes.
func TestEnsureWithinRoot_SymlinkLoopAndBrokenTarget(t *testing.T) {
	bundleDir := t.TempDir()
	if err := InitBundle(bundleDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	// 1. Symlink loop inside bundle
	loopA := filepath.Join(bundleDir, "loopA.md")
	loopB := filepath.Join(bundleDir, "loopB.md")
	if err := os.Symlink("loopB.md", loopA); err == nil {
		if err := os.Symlink("loopA.md", loopB); err == nil {
			_, err := ensureWithinRoot(bundleDir, loopA)
			if err == nil {
				t.Errorf("Expected ensureWithinRoot to fail on symlink loop, got nil")
			}
		}
	}

	// 2. Non-existent deeply nested path that resolves inside bundle
	nonExistentNested := filepath.Join(bundleDir, "deep", "dir", "nonexistent.md")
	resolved, err := ensureWithinRoot(bundleDir, nonExistentNested)
	if err != nil {
		t.Errorf("ensureWithinRoot for non-existent nested concept inside bundle failed: %v", err)
	}
	if !strings.HasPrefix(resolved, bundleDir) {
		t.Errorf("Resolved path %q does not start with bundleDir %q", resolved, bundleDir)
	}

	// 3. Non-existent path attempting traversal escape
	nonExistentEscape := filepath.Join(bundleDir, "deep", "..", "..", "..", "outside.md")
	_, err = ensureWithinRoot(bundleDir, nonExistentEscape)
	if err == nil {
		t.Errorf("Expected ensureWithinRoot to reject escaping path %q, got nil", nonExistentEscape)
	}
}

// TestValidateConceptID_ReservedAndTraversal tests ValidateConceptID against backslash separators,
// mixed-case reserved file names, trailing/leading dot patterns, and empty components.
func TestValidateConceptID_ReservedAndTraversal(t *testing.T) {
	cases := []struct {
		id      string
		wantErr bool
	}{
		{"concept", false},
		{"sub/concept", false},
		{"sub\\concept", true},      // backslash separator
		{"sub/../../escaped", true}, // traversal
		{"/abs/concept", true},      // leading slash
		{"\\abs\\concept", true},    // leading backslash
		{"index", true},             // reserved base
		{"INDEX", true},             // reserved case-insensitive
		{"InDeX.mD", true},          // reserved extension case-insensitive
		{"log", true},               // reserved log
		{"LOG.MD", true},            // reserved log.md
		{"AGENTS", true},            // reserved AGENTS
		{"agents.md", true},         // reserved agents.md
		{"sub/index.md", true},      // reserved index in subfolder
		{"sub/dir/index", true},     // reserved index in subfolder
		{".", true},                 // current dir
		{"..", true},                // parent dir
		{"", true},                  // empty string
		{"   ", true},               // whitespace only
		{"sub/./concept", false},    // clean relative path (clean resolves to sub/concept)
	}

	for _, tc := range cases {
		err := ValidateConceptID(tc.id)
		if tc.wantErr && err == nil {
			t.Errorf("ValidateConceptID(%q): expected error, got nil", tc.id)
		} else if !tc.wantErr && err != nil {
			t.Errorf("ValidateConceptID(%q): expected valid, got error: %v", tc.id, err)
		}
	}
}

// TestParseConcept_FrontmatterSmugglingAndMalformedYAML tests ParseConcept against
// frontmatter delimiter smuggling in flow mappings, malformed list mappings, and unexpected formatting.
func TestParseConcept_FrontmatterSmugglingAndMalformedYAML(t *testing.T) {
	// 1. Smuggled frontmatter delimiter inside body vs valid frontmatter
	contentWithBodyDelimiter := `---
type: Fact
title: Sample Concept
description: A concept with --- in body
---

# Title

Body with delimiter:
---
More body text.
`
	c, err := ParseConcept("sample.md", contentWithBodyDelimiter)
	if err != nil {
		t.Fatalf("ParseConcept failed: %v", err)
	}
	if c.Type != "Fact" || c.Title != "Sample Concept" {
		t.Errorf("Unexpected frontmatter values: type=%q title=%q", c.Type, c.Title)
	}
	if !strings.Contains(c.Body, "More body text.") {
		t.Errorf("Body content was cut off by body delimiter")
	}

	// 2. Flow mapping parsing for verified/generated with nested braces
	contentFlowMapping := `---
type: Decision
title: Flow Mapping Test
generated: { by: "agent/test", at: "2026-01-01T00:00:00Z" }
verified: [ { by: "human/lead", at: "2026-01-02T00:00:00Z" } ]
---

# Body
`
	cFlow, err := ParseConcept("flow.md", contentFlowMapping)
	if err != nil {
		t.Fatalf("ParseConcept flow mapping failed: %v", err)
	}
	if cFlow.Generated == nil || cFlow.Generated.By != "agent/test" {
		t.Errorf("Failed to parse flow generated.by: %+v", cFlow.Generated)
	}
	if len(cFlow.Verified) != 1 || cFlow.Verified[0].By != "human/lead" {
		t.Errorf("Failed to parse flow verified list: %+v", cFlow.Verified)
	}

	// 3. Missing frontmatter header
	_, errMissing := ParseConcept("no_fm.md", "# Only Body\nNo frontmatter at all.")
	if errMissing == nil {
		t.Errorf("Expected ParseConcept to return error when frontmatter is missing")
	}
}

// TestSaveConceptRejectsSubdirectoryReservedFiles verifies that reserved files (index.md anywhere, root log.md, root AGENTS.md)
// cannot be targeted as concepts in subdirectories.
func TestSaveConceptRejectsSubdirectoryReservedFiles(t *testing.T) {
	bundleDir := t.TempDir()
	if err := InitBundle(bundleDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	reservedPaths := []string{
		"sub/index.md",
		"sub/index",
		"sub/dir/index.md",
		"log.md",
		"AGENTS.md",
	}

	for _, relPath := range reservedPaths {
		c := &Concept{
			ID:    strings.TrimSuffix(relPath, ".md"),
			Path:  relPath,
			Title: "Reserved Overwrite Attempt",
			Type:  "Fact",
		}
		if err := SaveConcept(bundleDir, c, true, false, false, "attacker"); err == nil {
			t.Errorf("SaveConcept(%q) expected error for reserved file, got nil", relPath)
		}
		if err := ValidateConceptID(c.ID); err == nil {
			t.Errorf("ValidateConceptID(%q) expected error for reserved file, got nil", c.ID)
		}
	}
}

// TestSaveConceptRejectsSymlinkToReservedOrNonMarkdown verifies that SaveConcept refuses to write through symlinks
// targeting reserved documents or non-markdown files within the bundle.
func TestSaveConceptRejectsSymlinkToReservedOrNonMarkdown(t *testing.T) {
	bundleDir := t.TempDir()
	if err := InitBundle(bundleDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	// Create non-markdown file inside bundle
	dataJsonPath := filepath.Join(bundleDir, "data.json")
	if err := os.WriteFile(dataJsonPath, []byte(`{"key":"value"}`), 0o644); err != nil {
		t.Fatalf("WriteFile data.json: %v", err)
	}

	// Create symlink concept_link.md -> data.json
	symlinkNonMD := filepath.Join(bundleDir, "concept_json.md")
	if err := os.Symlink("data.json", symlinkNonMD); err != nil {
		t.Skipf("Symlinks not supported: %v", err)
	}

	// Create symlink concept_index.md -> index.md
	symlinkIndex := filepath.Join(bundleDir, "concept_index.md")
	if err := os.Symlink("index.md", symlinkIndex); err != nil {
		t.Skipf("Symlinks not supported: %v", err)
	}

	cJSON := &Concept{Path: "concept_json.md", Title: "JSON", Type: "Fact", Body: "PWNED"}
	if err := SaveConcept(bundleDir, cJSON, false, false, false, "attacker"); err == nil {
		t.Errorf("Expected SaveConcept through symlink to non-markdown file to fail, got nil")
	}

	cIndex := &Concept{Path: "concept_index.md", Title: "Index", Type: "Fact", Body: "PWNED"}
	if err := SaveConcept(bundleDir, cIndex, false, false, false, "attacker"); err == nil {
		t.Errorf("Expected SaveConcept through symlink to index.md to fail, got nil")
	}

	// Ensure index.md content was preserved
	idxContent, _ := os.ReadFile(filepath.Join(bundleDir, "index.md"))
	if strings.Contains(string(idxContent), "PWNED") {
		t.Errorf("index.md was overwritten via symlink!")
	}
}

// TestLoadBundleSubdirectoryLogIsolation verifies that a log.md in a subdirectory does not overwrite root LogContent.
func TestLoadBundleSubdirectoryLogIsolation(t *testing.T) {
	bundleDir := t.TempDir()
	if err := InitBundle(bundleDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	subDir := filepath.Join(bundleDir, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	subLogPath := filepath.Join(subDir, "log.md")
	if err := os.WriteFile(subLogPath, []byte("## 2000-01-01\n* Sub log\n"), 0o644); err != nil {
		t.Fatalf("WriteFile sub/log.md: %v", err)
	}

	b, err := LoadBundle(bundleDir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}

	if strings.Contains(b.LogContent, "Sub log") {
		t.Errorf("Root LogContent was overwritten by sub/log.md: %s", b.LogContent)
	}
}

// TestLoadBundleAllowsInternalSymlinks verifies that symlinks pointing within the bundle are allowed.
func TestLoadBundleAllowsInternalSymlinks(t *testing.T) {
	bundleDir := t.TempDir()
	if err := InitBundle(bundleDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	origConcept := &Concept{
		Path:        "project/overview.md",
		Title:       "Project Overview",
		Type:        "Overview",
		Description: "Main overview",
		Body:        "# Overview\n\nBody content.",
	}
	if err := SaveConcept(bundleDir, origConcept, true, false, true, "test/agent"); err != nil {
		t.Fatalf("SaveConcept failed: %v", err)
	}

	// Create internal relative symlink: alias.md -> project/overview.md
	aliasPath := filepath.Join(bundleDir, "alias.md")
	targetRel := filepath.Join("project", "overview.md")
	if err := os.Symlink(targetRel, aliasPath); err != nil {
		t.Skipf("Symlinks not supported on this platform: %v", err)
	}

	b, err := LoadBundle(bundleDir)
	if err != nil {
		t.Fatalf("Expected LoadBundle to allow internal symlinks, but got error: %v", err)
	}
	if _, ok := b.Concepts["project/overview"]; !ok {
		t.Errorf("Expected concept 'project/overview' in loaded bundle")
	}
}

// TestSaveConceptRejectsSymlinkTraversal verifies that SaveConcept refuses to write through symlinks pointing outside.
func TestSaveConceptRejectsSymlinkTraversal(t *testing.T) {
	bundleDir := t.TempDir()
	if err := InitBundle(bundleDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	outsideDir := t.TempDir()
	targetFile := filepath.Join(outsideDir, "critical_config.json")
	if err := os.WriteFile(targetFile, []byte(`{"protected": true}`), 0o600); err != nil {
		t.Fatalf("Failed to write target file: %v", err)
	}

	// Symlink pointing to external file
	symlinkPath := filepath.Join(bundleDir, "exploit.md")
	if err := os.Symlink(targetFile, symlinkPath); err != nil {
		t.Skipf("Symlinks not supported on this platform: %v", err)
	}

	maliciousConcept := &Concept{
		Path:        "exploit.md",
		Title:       "Exploit",
		Type:        "Attack",
		Description: "Overwrite attack",
		Body:        "PWNED",
	}

	err := SaveConcept(bundleDir, maliciousConcept, false, false, false, "attacker")
	if err == nil {
		t.Fatalf("Expected SaveConcept to reject writing through symlink pointing outside, but got nil")
	}
	if !strings.Contains(err.Error(), "path traversal denied") && !strings.Contains(err.Error(), "escapes bundle directory") {
		t.Errorf("Expected path traversal error message, got: %v", err)
	}

	// Verify original file was not modified
	data, _ := os.ReadFile(targetFile)
	if string(data) != `{"protected": true}` {
		t.Errorf("External file was modified! Got: %s", string(data))
	}
}

// TestSaveConceptRejectsDirectorySymlinkEscape verifies that SaveConcept refuses to write through directory symlinks pointing outside.
func TestSaveConceptRejectsDirectorySymlinkEscape(t *testing.T) {
	bundleDir := t.TempDir()
	if err := InitBundle(bundleDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	outsideDir := t.TempDir()
	dirSymlink := filepath.Join(bundleDir, "escapedir")
	if err := os.Symlink(outsideDir, dirSymlink); err != nil {
		t.Skipf("Symlinks not supported on this platform: %v", err)
	}

	maliciousConcept := &Concept{
		Path:        "escapedir/evil.md",
		Title:       "Evil",
		Type:        "Attack",
		Description: "Directory escape",
		Body:        "PWNED",
	}

	err := SaveConcept(bundleDir, maliciousConcept, true, false, false, "attacker")
	if err == nil {
		t.Fatalf("Expected SaveConcept to reject directory symlink escape, but got nil")
	}
	if !strings.Contains(err.Error(), "path traversal denied") && !strings.Contains(err.Error(), "escapes bundle directory") {
		t.Errorf("Expected path traversal error message, got: %v", err)
	}
}
