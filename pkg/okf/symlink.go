package okf

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// SymlinkDefinition specifies a tool symlink target.
type SymlinkDefinition struct {
	ToolName    string
	RelPath     string // Relative path from repository root (e.g. "CLAUDE.md", ".github/copilot-instructions.md")
	RelTarget   string // Relative symlink target (e.g. "AGENTS.md", "../AGENTS.md")
	Description string
}

// GetDefaultSymlinkDefs returns the standard SSoT tool symlink mappings.
func GetDefaultSymlinkDefs() []SymlinkDefinition {
	return []SymlinkDefinition{
		{
			ToolName:    "Claude Code",
			RelPath:     "CLAUDE.md",
			RelTarget:   "AGENTS.md",
			Description: "Anthropic Claude Code instructions",
		},
		{
			ToolName:    "Cursor",
			RelPath:     ".cursorrules",
			RelTarget:   "AGENTS.md",
			Description: "Cursor IDE rules",
		},
		{
			ToolName:    "Windsurf",
			RelPath:     ".windsurfrules",
			RelTarget:   "AGENTS.md",
			Description: "Windsurf Codeium rules",
		},
		{
			ToolName:    "GitHub Copilot",
			RelPath:     filepath.ToSlash(filepath.Join(".github", "copilot-instructions.md")),
			RelTarget:   "../AGENTS.md",
			Description: "GitHub Copilot custom instructions",
		},
	}
}

// SymlinkResult records the outcome of creating a symlink.
type SymlinkResult struct {
	ToolName    string `json:"tool_name"`
	Path        string `json:"path"`
	Target      string `json:"target"`
	Created     bool   `json:"created"`
	Overwritten bool   `json:"overwritten"`
	Error       string `json:"error,omitempty"`
}

// SymlinkStatus records the current verification status of a tool symlink.
type SymlinkStatus struct {
	ToolName       string `json:"tool_name"`
	ToolPath       string `json:"tool_path"`
	ExpectedTarget string `json:"expected_target"`
	ActualTarget   string `json:"actual_target,omitempty"`
	Exists         bool   `json:"exists"`
	IsSymlink      bool   `json:"is_symlink"`
	IsValid        bool   `json:"is_valid"`
	ErrorMessage   string `json:"error_message,omitempty"`
}

// CreateToolSymlinks creates all default SSoT symlinks from canonical AGENTS.md.
func CreateToolSymlinks(root string, force bool) ([]SymlinkResult, error) {
	agentsPath := filepath.Join(root, "AGENTS.md")
	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("canonical AGENTS.md not found in %s", root)
	}

	defs := GetDefaultSymlinkDefs()
	results := make([]SymlinkResult, 0, len(defs))

	for _, def := range defs {
		fullPath := filepath.Join(root, def.RelPath)
		parentDir := filepath.Dir(fullPath)

		if err := os.MkdirAll(parentDir, 0o755); err != nil {
			results = append(results, SymlinkResult{
				ToolName: def.ToolName,
				Path:     def.RelPath,
				Target:   def.RelTarget,
				Error:    fmt.Sprintf("failed to create directory %s: %v", parentDir, err),
			})
			continue
		}

		overwritten := false
		if fi, err := os.Lstat(fullPath); err == nil {
			if !force {
				// Check if already valid
				if fi.Mode()&os.ModeSymlink != 0 {
					target, err := os.Readlink(fullPath)
					if err == nil && filepath.ToSlash(target) == filepath.ToSlash(def.RelTarget) {
						results = append(results, SymlinkResult{
							ToolName: def.ToolName,
							Path:     def.RelPath,
							Target:   def.RelTarget,
							Created:  false,
						})
						continue
					}
				} else if runtime.GOOS == "windows" && isGitSymlinkStub(fullPath, def.RelTarget) {
					results = append(results, SymlinkResult{
						ToolName: def.ToolName,
						Path:     def.RelPath,
						Target:   def.RelTarget,
						Created:  false,
					})
					continue
				}
				results = append(results, SymlinkResult{
					ToolName: def.ToolName,
					Path:     def.RelPath,
					Target:   def.RelTarget,
					Error:    "file exists (use --force to overwrite)",
				})
				continue
			}

			if err := os.Remove(fullPath); err != nil {
				results = append(results, SymlinkResult{
					ToolName: def.ToolName,
					Path:     def.RelPath,
					Target:   def.RelTarget,
					Error:    fmt.Sprintf("failed to remove existing file: %v", err),
				})
				continue
			}
			overwritten = true
		}

		if err := os.Symlink(filepath.FromSlash(def.RelTarget), fullPath); err != nil {
			results = append(results, SymlinkResult{
				ToolName: def.ToolName,
				Path:     def.RelPath,
				Target:   def.RelTarget,
				Error:    fmt.Sprintf("failed to create symlink: %v", err),
			})
			continue
		}

		results = append(results, SymlinkResult{
			ToolName:    def.ToolName,
			Path:        def.RelPath,
			Target:      def.RelTarget,
			Created:     true,
			Overwritten: overwritten,
		})
	}

	return results, nil
}

// CheckToolSymlinks inspects whether all SSoT symlinks exist and point correctly.
func CheckToolSymlinks(root string) ([]SymlinkStatus, error) {
	defs := GetDefaultSymlinkDefs()
	statuses := make([]SymlinkStatus, 0, len(defs))

	for _, def := range defs {
		fullPath := filepath.Join(root, def.RelPath)
		st := SymlinkStatus{
			ToolName:       def.ToolName,
			ToolPath:       def.RelPath,
			ExpectedTarget: def.RelTarget,
		}

		fi, err := os.Lstat(fullPath)
		if os.IsNotExist(err) {
			st.Exists = false
			st.ErrorMessage = "symlink does not exist"
			statuses = append(statuses, st)
			continue
		} else if err != nil {
			st.Exists = false
			st.ErrorMessage = err.Error()
			statuses = append(statuses, st)
			continue
		}

		st.Exists = true
		if fi.Mode()&os.ModeSymlink == 0 {
			if runtime.GOOS == "windows" && isGitSymlinkStub(fullPath, def.RelTarget) {
				st.IsSymlink = true
				st.ActualTarget = def.RelTarget
				st.IsValid = true
				statuses = append(statuses, st)
				continue
			}
			st.IsSymlink = false
			st.ErrorMessage = "file exists but is a regular file, not a symlink"
			statuses = append(statuses, st)
			continue
		}

		st.IsSymlink = true
		target, err := os.Readlink(fullPath)
		if err != nil {
			st.ErrorMessage = fmt.Sprintf("failed to read symlink target: %v", err)
			statuses = append(statuses, st)
			continue
		}

		st.ActualTarget = target
		if filepath.ToSlash(target) == filepath.ToSlash(def.RelTarget) {
			st.IsValid = true
		} else {
			st.ErrorMessage = fmt.Sprintf("points to %q, expected %q", target, def.RelTarget)
		}

		statuses = append(statuses, st)
	}

	return statuses, nil
}

// isGitSymlinkStub returns true if fullPath is a small regular file generated by
// Git on Windows when core.symlinks=false whose trimmed content equals expectedTarget.
func isGitSymlinkStub(fullPath, expectedTarget string) bool {
	fi, err := os.Stat(fullPath)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > 256 {
		return false
	}
	// #nosec G304 -- fullPath is constructed from repository root and fixed GetDefaultSymlinkDefs
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return false
	}
	content := strings.ReplaceAll(filepath.ToSlash(strings.TrimSpace(string(data))), "\\", "/")
	expected := strings.ReplaceAll(filepath.ToSlash(expectedTarget), "\\", "/")
	return content == expected
}
