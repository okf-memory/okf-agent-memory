package okf

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed assets
var embeddedAssets embed.FS

const defaultOKFAgentsBlock = `<!-- BEGIN OKF AGENT MEMORY -->
---

## 🧠 Persistent Project Memory (OKF v0.2)

> Powered by [OKF Agent Memory](https://github.com/okf-memory/okf-agent-memory) — Open Knowledge Format (OKF) v0.2 persistent project memory for AI agents.

When working in this codebase, you must follow the memory conventions:

1. **Persistent Knowledge Lives in ` + "`knowledge/`" + `**:
   - The ` + "`knowledge/`" + ` directory is an **Open Knowledge Format (OKF) v0.2** bundle.
   - Store durable facts, architectural decisions, and project findings in ` + "`knowledge/`" + `. Never store transient conversational noise.

2. **Read Before Write (Search Before Create)**:
   - Before authoring new knowledge or code, query existing memory: ` + "`okf search \"<query>\"`" + ` or inspect ` + "`knowledge/index.md`" + `.
   - Update existing concepts instead of creating duplicates.

3. **Strict Context & Search-First Retrieval (No Blanket Scans)**:
   - **DO NOT** use ` + "`list_dir`" + `, ` + "`grep`" + `, or scan ` + "`knowledge/`" + ` in bulk.
   - Query knowledge via ` + "`okf search \"<query>\" --limit 3 --json`" + ` only when relevant or requested.
   - Inspect concept descriptions first and load full concepts only on demand using ` + "`okf show <id>`" + `.

4. **Preserve Trust & Provenance**:
   - Agent writes declare ` + "`generated: { by: \"<agent>\", at: \"<timestamp>\" }`" + `. Never forge human verification (` + "`verified:`" + `).

5. **Essential Memory Commands**:
   - ` + "`okf search \"<query>\"`" + ` — Query memory using in-memory BM25
   - ` + "`okf show <id>`" + ` — Inspect concept details and relationship graph
   - ` + "`okf create <id> --type <type> --title \"<title>\" --desc \"<desc>\"`" + ` — Document new fact
   - ` + "`okf update <id> --desc \"<updated-desc>\"`" + ` — Modify existing concept
   - ` + "`okf relate <src> <tgt> --desc \"<rel>\"`" + ` — Link concepts together
   - ` + "`okf validate knowledge --strict --drift`" + ` — Verify 100% OKF v0.2 conformance

6. **End-of-Task Review Checklist**:
   - Did I make an architectural decision? -> Record under ` + "`knowledge/architecture/`" + `
   - Did I add/update concepts? -> Ensure ` + "`knowledge/log.md`" + ` and parent ` + "`index.md`" + ` are updated.
   - Did I validate? -> Ensure 0 errors, 0 broken links (` + "`okf validate knowledge --strict`" + `).
<!-- END OKF AGENT MEMORY -->
`

// BootstrapOptions controls which elements are installed during bootstrap.
type BootstrapOptions struct {
	ProjectName       string
	InstallSkill      bool
	InstallAgentsMD   bool
	OverwriteAgentsMD bool
	InstallMakefile   bool
	InstallBundle     bool
}

// DefaultBootstrapOptions returns standard full bootstrap settings.
func DefaultBootstrapOptions() BootstrapOptions {
	return BootstrapOptions{
		InstallSkill:      true,
		InstallAgentsMD:   true,
		OverwriteAgentsMD: false,
		InstallMakefile:   true,
		InstallBundle:     true,
	}
}

// Bootstrap initializes a target project directory with everything needed for OKF Agent Memory.
func Bootstrap(targetDir string, opts BootstrapOptions) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	projectName := opts.ProjectName
	if projectName == "" {
		absTarget, err := filepath.Abs(targetDir)
		if err == nil {
			projectName = filepath.Base(absTarget)
		}
		if projectName == "" || projectName == "." || projectName == "/" {
			projectName = "my-project"
		}
	}

	// 1. Install Knowledge Bundle Scaffold
	if opts.InstallBundle {
		knowledgeDir := filepath.Join(targetDir, "knowledge")
		if err := os.MkdirAll(knowledgeDir, 0o755); err != nil {
			return fmt.Errorf("failed to create knowledge dir: %w", err)
		}

		rootIndex := filepath.Join(knowledgeDir, "index.md")
		if _, err := os.Stat(rootIndex); os.IsNotExist(err) {
			content := fmt.Sprintf("---\nokf_version: \"0.2\"\n---\n\n# %s Knowledge Base\n\nPersistent memory bundle for this repository.\n", projectName)
			if err := os.WriteFile(rootIndex, []byte(content), 0o644); err != nil {
				return fmt.Errorf("failed to write knowledge/index.md: %w", err)
			}
		}

		logFile := filepath.Join(knowledgeDir, "log.md")
		if _, err := os.Stat(logFile); os.IsNotExist(err) {
			today := time.Now().UTC().Format("2006-01-02")
			logContent := fmt.Sprintf("## %s\n* **Creation**: Initialized OKF v0.2 project memory.\n", today)
			if err := os.WriteFile(logFile, []byte(logContent), 0o644); err != nil {
				return fmt.Errorf("failed to write knowledge/log.md: %w", err)
			}
		}
	}

	// 2. Install Agent Skill (.agents/skills/okf-memory/)
	if opts.InstallSkill {
		skillDestDir := filepath.Join(targetDir, ".agents", "skills", "okf-memory")
		if err := os.MkdirAll(skillDestDir, 0o755); err != nil {
			return fmt.Errorf("failed to create skill directory: %w", err)
		}

		err := fs.WalkDir(embeddedAssets, "assets/skill", func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() {
				return walkErr
			}
			rel, _ := filepath.Rel("assets/skill", path)
			destPath := filepath.Join(skillDestDir, rel)
			if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
				return err
			}
			data, err := embeddedAssets.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(destPath, data, 0o644)
		})
		if err != nil {
			return fmt.Errorf("failed to install agent skill: %w", err)
		}
	}

	// 3. Install or Non-Destructive Enrich AGENTS.md
	if opts.InstallAgentsMD {
		agentsMDPath := filepath.Join(targetDir, "AGENTS.md")
		if _, err := os.Stat(agentsMDPath); os.IsNotExist(err) || opts.OverwriteAgentsMD {
			data, err := embeddedAssets.ReadFile("assets/templates/AGENTS.md")
			if err == nil {
				content := strings.ReplaceAll(string(data), "{{PROJECT_NAME}}", projectName)
				_ = os.WriteFile(agentsMDPath, []byte(content), 0o644)
			}
		} else {
			// Existing AGENTS.md: check and append delimited OKF section without overwriting user rules
			existingData, err := os.ReadFile(agentsMDPath)
			if err == nil {
				existingStr := string(existingData)
				if !strings.Contains(existingStr, "BEGIN OKF AGENT MEMORY") &&
					!strings.Contains(existingStr, "okf-agent-memory") &&
					!strings.Contains(existingStr, "Open Knowledge Format (OKF)") {
					newContent := strings.TrimRight(existingStr, "\n") + "\n\n" + defaultOKFAgentsBlock
					_ = os.WriteFile(agentsMDPath, []byte(newContent), 0o644)
				}
			}
		}
	}

	// 4. Install Makefile
	if opts.InstallMakefile {
		makefilePath := filepath.Join(targetDir, "Makefile")
		if _, err := os.Stat(makefilePath); os.IsNotExist(err) {
			data, err := embeddedAssets.ReadFile("assets/templates/Makefile")
			if err == nil {
				_ = os.WriteFile(makefilePath, data, 0o644)
			}
		}
	}

	return nil
}
