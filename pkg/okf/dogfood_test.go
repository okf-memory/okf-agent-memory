package okf

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDogfoodingAssetDrift ensures that the skills actively dogfooded by this repository
// in `.agents/skills/okf-memory/` stay 100% byte-for-byte identical to the embedded
// assets in `pkg/okf/assets/skill/` used by `okf bootstrap`.
// It also verifies that the scaffolded `pkg/okf/assets/templates/AGENTS.md` remains
// project-neutral and does not leak internal repository paths.
func TestDogfoodingAssetDrift(t *testing.T) {
	// Locate repository root relative to pkg/okf test execution directory
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}

	activeSkillDir := filepath.Join(repoRoot, ".agents", "skills", "okf-memory")
	embeddedSkillDir := filepath.Join(repoRoot, "pkg", "okf", "assets", "skill")

	// 1. Verify all files in active skill directory exist and match embedded skill directory
	err = filepath.WalkDir(activeSkillDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(activeSkillDir, path)
		if err != nil {
			return err
		}

		activeBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		embeddedPath := filepath.Join(embeddedSkillDir, relPath)
		embeddedBytes, err := os.ReadFile(embeddedPath)
		if err != nil {
			t.Errorf("Dogfooding asset missing: file %s exists in .agents/skills/okf-memory/ but not in pkg/okf/assets/skill/. Run 'make sync-assets' to fix.", relPath)
			return nil
		}

		if !bytes.Equal(activeBytes, embeddedBytes) {
			t.Errorf("Dogfooding drift detected: %s differs between .agents/skills/okf-memory/ and pkg/okf/assets/skill/. Run 'make sync-assets' to synchronize.", relPath)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("failed walking active skill dir: %v", err)
	}

	// 2. Verify no orphan files exist in embedded skill dir
	err = filepath.WalkDir(embeddedSkillDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(embeddedSkillDir, path)
		if err != nil {
			return err
		}

		activePath := filepath.Join(activeSkillDir, relPath)
		if _, err := os.Stat(activePath); os.IsNotExist(err) {
			t.Errorf("Orphan embedded asset: file %s exists in pkg/okf/assets/skill/ but not in .agents/skills/okf-memory/. Clean it up or run 'make sync-assets'.", relPath)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed walking embedded skill dir: %v", err)
	}

	// 3. Verify that the scaffold template AGENTS.md does NOT leak internal repo-specific paths
	templatePath := filepath.Join(repoRoot, "pkg", "okf", "assets", "templates", "AGENTS.md")
	templateBytes, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("failed to read template AGENTS.md: %v", err)
	}

	templateContent := string(templateBytes)
	forbiddenTokens := []string{
		"pkg/okf/assets",
		"Dogfooding constraint",
		"Dogfooding",
	}

	for _, token := range forbiddenTokens {
		if strings.Contains(templateContent, token) {
			t.Errorf("Generic template leakage: %s contains repository-specific token %q. Templates must stay project-neutral!", templatePath, token)
		}
	}
}
