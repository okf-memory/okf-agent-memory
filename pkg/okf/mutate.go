package okf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

func titleCase(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// InitBundle initializes a new OKF v0.2 bundle with root index.md and log.md.
func InitBundle(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	rootIndex := filepath.Join(dir, "index.md")
	if _, err := os.Stat(rootIndex); os.IsNotExist(err) {
		indexContent := "---\nokf_version: \"0.2\"\n---\n\n# Knowledge Base\n\n"
		if err := os.WriteFile(rootIndex, []byte(indexContent), 0o644); err != nil {
			return fmt.Errorf("failed to write root index.md: %w", err)
		}
	}

	logFile := filepath.Join(dir, "log.md")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		today := time.Now().UTC().Format("2006-01-02")
		logContent := fmt.Sprintf("## %s\n* **Creation**: Initialized OKF v0.2 knowledge bundle.\n", today)
		if err := os.WriteFile(logFile, []byte(logContent), 0o644); err != nil {
			return fmt.Errorf("failed to write log.md: %w", err)
		}
	}

	return nil
}

// AppendLogEntry prepends a new dated change entry to log.md.
func AppendLogEntry(bundleDir, entryType, description string) error {
	logPath := filepath.Join(bundleDir, "log.md")
	today := time.Now().UTC().Format("2006-01-02")
	newEntry := fmt.Sprintf("* **%s**: %s\n", entryType, description)

	existingContent := ""
	if data, err := os.ReadFile(logPath); err == nil {
		existingContent = string(data)
	}

	heading := fmt.Sprintf("## %s\n", today)
	if strings.Contains(existingContent, heading) {
		// Insert under existing today heading
		existingContent = strings.Replace(existingContent, heading, heading+newEntry, 1)
	} else {
		// Prepend today heading
		existingContent = heading + newEntry + "\n" + strings.TrimLeft(existingContent, "\n")
	}

	return os.WriteFile(logPath, []byte(existingContent), 0o644)
}

// UpdateParentIndex ensures the concept is listed in its immediate directory index.md.
func UpdateParentIndex(bundleDir string, c *Concept) error {
	dir := filepath.Dir(c.Path)
	indexRelPath := "index.md"
	if dir != "." {
		indexRelPath = filepath.Join(dir, "index.md")
	}
	indexPath := filepath.Join(bundleDir, indexRelPath)

	targetFilename := filepath.Base(c.Path)
	targetTitle := c.Title
	if targetTitle == "" {
		targetTitle = targetFilename
	}
	targetDesc := c.Description

	newListing := fmt.Sprintf("* [%s](%s) - %s", targetTitle, targetFilename, targetDesc)
	if targetDesc == "" {
		newListing = fmt.Sprintf("* [%s](%s)", targetTitle, targetFilename)
	}

	existingContent := ""
	if data, err := os.ReadFile(indexPath); err == nil {
		existingContent = string(data)
	} else {
		// Create index.md with default header
		header := fmt.Sprintf("# %s\n\n", titleCase(filepath.Base(dir)))
		if dir == "." {
			header = "---\nokf_version: \"0.2\"\n---\n\n# Knowledge Base\n\n"
		}
		existingContent = header
	}

	// Check if already listed
	linkTarget := fmt.Sprintf("(%s)", targetFilename)
	if strings.Contains(existingContent, linkTarget) {
		// Update existing line
		lines := strings.Split(existingContent, "\n")
		for i, l := range lines {
			if strings.Contains(l, linkTarget) {
				lines[i] = newListing
				break
			}
		}
		existingContent = strings.Join(lines, "\n")
	} else {
		// Append listing
		existingContent = strings.TrimRight(existingContent, "\n") + "\n" + newListing + "\n"
	}

	return os.WriteFile(indexPath, []byte(existingContent), 0o644)
}

// SaveConcept writes a concept file to disk and optionally executes automatic bookkeeping.
func SaveConcept(bundleDir string, c *Concept, isNew, autoLog, autoIndex bool, actor string) error {
	fullPath := filepath.Join(bundleDir, c.Path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Update generated timestamp & actor
	if actor == "" {
		actor = "agent/okf-tool"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	c.Generated = &Generated{
		By: actor,
		At: now,
	}

	raw := SerializeConcept(c)
	if err := os.WriteFile(fullPath, []byte(raw), 0o644); err != nil {
		return fmt.Errorf("failed to write concept: %w", err)
	}

	// Automated Bookkeeping
	if autoIndex {
		if err := UpdateParentIndex(bundleDir, c); err != nil {
			return fmt.Errorf("failed to update parent index: %w", err)
		}
	}

	if autoLog {
		entryType := "Update"
		desc := fmt.Sprintf("Updated concept `%s`.", c.Path)
		if isNew {
			entryType = "Creation"
			desc = fmt.Sprintf("Documented concept `%s` (%s).", c.Path, c.Title)
		}
		if err := AppendLogEntry(bundleDir, entryType, desc); err != nil {
			return fmt.Errorf("failed to append log entry: %w", err)
		}
	}

	return nil
}

// RelateConcepts creates a relative markdown link between source and target concepts.
func RelateConcepts(bundleDir, sourceID, targetID, relationDesc, actor string) error {
	b, err := LoadBundle(bundleDir)
	if err != nil {
		return fmt.Errorf("failed to load bundle: %w", err)
	}

	srcConcept, ok := b.Concepts[strings.TrimSuffix(sourceID, ".md")]
	if !ok {
		return fmt.Errorf("source concept '%s' not found", sourceID)
	}

	tgtConcept, ok := b.Concepts[strings.TrimSuffix(targetID, ".md")]
	if !ok {
		return fmt.Errorf("target concept '%s' not found", targetID)
	}

	// Compute relative path from source's directory to target
	srcDir := filepath.Dir(srcConcept.Path)
	relPath, err := filepath.Rel(srcDir, tgtConcept.Path)
	if err != nil {
		return fmt.Errorf("failed to compute relative path: %w", err)
	}
	relPath = filepath.ToSlash(relPath)

	linkText := tgtConcept.Title
	if linkText == "" {
		linkText = filepath.Base(targetID)
	}

	relStatement := fmt.Sprintf("\n- Related to [%s](%s)", linkText, relPath)
	if relationDesc != "" {
		relStatement = fmt.Sprintf("\n- [%s](%s): %s", linkText, relPath, relationDesc)
	}

	if !strings.Contains(srcConcept.Body, "# Related") {
		srcConcept.Body = strings.TrimRight(srcConcept.Body, "\n") + "\n\n# Related Concepts" + relStatement + "\n"
	} else {
		srcConcept.Body = strings.TrimRight(srcConcept.Body, "\n") + relStatement + "\n"
	}

	if err := SaveConcept(bundleDir, srcConcept, false, true, false, actor); err != nil {
		return fmt.Errorf("failed to save related concept: %w", err)
	}

	logDesc := fmt.Sprintf("Linked `%s` to `%s`.", srcConcept.Path, tgtConcept.Path)
	if relationDesc != "" {
		logDesc = fmt.Sprintf("Linked `%s` to `%s` (%s).", srcConcept.Path, tgtConcept.Path, relationDesc)
	}
	return AppendLogEntry(bundleDir, "Update", logDesc)
}
