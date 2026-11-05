package okf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateAndCheckToolSymlinks(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "okf-symlink-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create canonical AGENTS.md
	agentsPath := filepath.Join(tempDir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("# Canonical AGENTS.md\n"), 0o644); err != nil {
		t.Fatalf("failed to write AGENTS.md: %v", err)
	}

	// 1. Check symlinks before creation -> all should be missing
	statuses, err := CheckToolSymlinks(tempDir)
	if err != nil {
		t.Fatalf("unexpected error checking symlinks: %v", err)
	}
	if len(statuses) < 4 {
		t.Fatalf("expected at least 4 tool symlink targets, got %d", len(statuses))
	}
	for _, st := range statuses {
		if st.Exists {
			t.Errorf("expected %s to not exist yet", st.ToolPath)
		}
	}

	// 2. Create symlinks
	results, err := CreateToolSymlinks(tempDir, false)
	if err != nil {
		t.Fatalf("failed to create tool symlinks: %v", err)
	}
	if len(results) != len(statuses) {
		t.Errorf("expected %d results, got %d", len(statuses), len(results))
	}

	// 3. Verify all created symlinks point correctly
	afterStatuses, err := CheckToolSymlinks(tempDir)
	if err != nil {
		t.Fatalf("failed to check symlinks after creation: %v", err)
	}
	for _, st := range afterStatuses {
		if !st.Exists {
			t.Errorf("expected %s to exist as symlink", st.ToolPath)
		}
		if !st.IsValid {
			t.Errorf("expected %s to be valid symlink pointing to %s (error: %s)", st.ToolPath, st.ExpectedTarget, st.ErrorMessage)
		}
	}

	// 4. Force recreate without error
	_, err = CreateToolSymlinks(tempDir, true)
	if err != nil {
		t.Errorf("expected force recreate to succeed, got: %v", err)
	}
}

func TestIsGitSymlinkStub(t *testing.T) {
	tempDir := t.TempDir()

	stubPath := filepath.Join(tempDir, "CLAUDE.md")
	if err := os.WriteFile(stubPath, []byte("AGENTS.md\n"), 0o644); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	if !isGitSymlinkStub(stubPath, "AGENTS.md") {
		t.Errorf("expected stub pointing to AGENTS.md to be recognized")
	}
	if isGitSymlinkStub(stubPath, "../AGENTS.md") {
		t.Errorf("expected mismatched target to return false")
	}

	nestedStub := filepath.Join(tempDir, "copilot-instructions.md")
	if err := os.WriteFile(nestedStub, []byte("..\\AGENTS.md"), 0o644); err != nil {
		t.Fatalf("write nested stub: %v", err)
	}
	if !isGitSymlinkStub(nestedStub, "../AGENTS.md") {
		t.Errorf("expected backslash stub ..\\AGENTS.md to match ../AGENTS.md")
	}
}
