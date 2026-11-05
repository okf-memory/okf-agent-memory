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

### 1. Behavioral Invariants & Constraints (RFC 2119)
- MUST execute ` + "`okf_search(query=keywords, limit=3)`" + ` before proposing architecture, new dependencies, or substantial code changes.
- NEVER scan ` + "`knowledge/`" + ` via ` + "`list_dir`" + `, ` + "`grep_search`" + `, ` + "`find`" + `, or raw file readers.
- NEVER forge human verification (` + "`verified:`" + ` is human-only; declare ` + "`generated: { by: \"<actor>\", at: \"<iso-time>\" }`" + `).
- PREFER native ` + "`okf_*`" + ` MCP tools OVER CLI fallback commands.

### 2. Guard Clauses & Scope Governance
- ON edit(@path/):
    IF first_visit(@path/) => okf_search(for_path=@path/)
    IF governance == "hold" => STOP("Subsystem frozen by governance. Request explicit human confirmation.")
    IF governance == "constraint" => MUST adhere to all listed invariants
    IF governance == "context" => proceed with awareness
- ON user_query(architecture | requirements | conventions | domain_facts):
    okf_search(query=keywords, limit=3) => evaluate summary description
    IF relevant => okf_show(concept_id) ONLY on demand
    IF updating_existing_concept => PREFER okf_update OVER okf_create

### 3. Completion Pipeline (Sequential Assertion Gates)
1. IF arch_decisions_made => MUST okf_create(architecture/*, type="decision", title=..., desc=...)
2. IF requirements_discovered => MUST okf_update(concept_id)
3. IF concepts_mutated => MUST sync(knowledge/log.md, knowledge/index.md)
4. ASSERT(okf_validate(strict=true, drift=true) == {errors: 0, warnings: 0}, ELSE=fix_before_exit)
<!-- END OKF AGENT MEMORY -->
`

// BootstrapOptions controls which elements are installed during bootstrap.
type BootstrapOptions struct {
	ProjectName       string
	InstallSkill      bool
	InstallAgentsMD   bool
	OverwriteAgentsMD bool
	InstallSymlinks   bool
	InstallMakefile   bool
	InstallBundle     bool
}

// DefaultBootstrapOptions returns standard full bootstrap settings.
func DefaultBootstrapOptions() BootstrapOptions {
	return BootstrapOptions{
		InstallSkill:      true,
		InstallAgentsMD:   true,
		OverwriteAgentsMD: false,
		InstallSymlinks:   true,
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
			// #nosec G304 -- agentsMDPath is constructed directly within targetDir
			existingData, err := os.ReadFile(agentsMDPath)
			if err == nil {
				existingStr := string(existingData)
				if !strings.Contains(existingStr, "BEGIN OKF AGENT MEMORY") {
					newContent := strings.TrimRight(existingStr, "\n") + "\n\n" + defaultOKFAgentsBlock
					// #nosec G703 -- agentsMDPath is constructed directly within targetDir
					_ = os.WriteFile(agentsMDPath, []byte(newContent), 0o644)
				}
			}
		}
	}

	// 4. Create SSoT Tool Symlinks (CLAUDE.md, .cursorrules, .windsurfrules, .github/copilot-instructions.md)
	if opts.InstallSymlinks && opts.InstallAgentsMD {
		if _, err := CreateToolSymlinks(targetDir, false); err != nil {
			return fmt.Errorf("failed to create tool symlinks: %w", err)
		}
	}

	// 5. Install Makefile
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
