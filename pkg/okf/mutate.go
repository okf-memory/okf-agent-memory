package okf

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
	entryType = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(entryType, "\r", " "), "\n", " "))
	description = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(description, "\r", " "), "\n", " "))

	logPath := filepath.Join(bundleDir, "log.md")
	if _, err := ensureWithinRoot(bundleDir, logPath); err != nil {
		return err
	}
	today := time.Now().UTC().Format("2006-01-02")
	newEntry := fmt.Sprintf("* **%s**: %s\n", entryType, description)

	existingContent := ""
	// #nosec G304 -- logPath is guaranteed within bundleDir via ensureWithinRoot
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

	// #nosec G703 -- logPath is validated and contained within bundle root
	return os.WriteFile(logPath, []byte(existingContent), 0o644)
}

// ValidateConceptID verifies that a concept ID conforms to OKF naming conventions
// and does not attempt path traversal or target reserved bundle files.
func ValidateConceptID(id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return fmt.Errorf("concept ID cannot be empty")
	}

	if strings.ContainsAny(trimmed, "\x00\r\n\t") {
		return fmt.Errorf("concept ID %q contains forbidden control characters", id)
	}

	cleanID := strings.TrimSuffix(trimmed, ".md")
	if cleanID == "" || cleanID == "." || cleanID == ".." {
		return fmt.Errorf("invalid concept ID %q", id)
	}

	if filepath.IsAbs(cleanID) || strings.HasPrefix(cleanID, "/") || strings.HasPrefix(cleanID, "\\") ||
		(len(cleanID) >= 2 && cleanID[1] == ':' && ((cleanID[0] >= 'a' && cleanID[0] <= 'z') || (cleanID[0] >= 'A' && cleanID[0] <= 'Z'))) {
		return fmt.Errorf("concept ID %q must be a relative path", id)
	}

	// Split by '/' or '\' and inspect each path component
	parts := strings.FieldsFunc(cleanID, func(r rune) bool {
		return r == '/' || r == '\\'
	})

	if slices.Contains(parts, "..") {
		return fmt.Errorf("concept ID %q contains forbidden '..' traversal", id)
	}

	// Clean path and ensure it does not escape
	cleaned := filepath.Clean(cleanID)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return fmt.Errorf("concept ID %q escapes bundle directory", id)
	}

	// Check for reserved filenames (index.md anywhere, root log.md, root AGENTS.md)
	base := filepath.Base(cleaned)
	normClean := filepath.ToSlash(cleaned)
	if strings.EqualFold(base, "index") || strings.EqualFold(base, "index.md") ||
		strings.EqualFold(normClean, "log") || strings.EqualFold(normClean, "log.md") ||
		strings.EqualFold(normClean, "AGENTS") || strings.EqualFold(normClean, "AGENTS.md") {
		return fmt.Errorf("concept ID %q is a reserved bundle document", id)
	}

	return nil
}

// UpdateParentIndex ensures the concept is listed in its immediate directory index.md.
func UpdateParentIndex(bundleDir string, c *Concept) error {
	absBundle, err := filepath.Abs(bundleDir)
	if err != nil {
		return fmt.Errorf("invalid bundle directory: %w", err)
	}

	dir := filepath.Dir(c.Path)
	indexRelPath := "index.md"
	if dir != "." {
		indexRelPath = filepath.Join(dir, "index.md")
	}

	indexPath := filepath.Join(absBundle, filepath.Clean(indexRelPath))
	rel, err := filepath.Rel(absBundle, indexPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path traversal denied: parent index %q escapes bundle directory", indexRelPath)
	}
	if _, err := ensureWithinRoot(bundleDir, indexPath); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory for parent index %q: %w", indexRelPath, err)
	}

	targetFilename := filepath.Base(c.Path)
	targetTitle := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(c.Title, "\r", " "), "\n", " "))
	if targetTitle == "" {
		targetTitle = targetFilename
	}
	targetDesc := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(c.Description, "\r", " "), "\n", " "))

	newListing := fmt.Sprintf("* [%s](%s) - %s", targetTitle, targetFilename, targetDesc)
	if targetDesc == "" {
		newListing = fmt.Sprintf("* [%s](%s)", targetTitle, targetFilename)
	}

	existingContent := ""
	// #nosec G304 -- indexPath is validated within bundleDir via ensureWithinRoot
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

	// #nosec G703 -- indexPath is verified within bundleDir
	return os.WriteFile(indexPath, []byte(existingContent), 0o644)
}

// resolveInBundle joins relPath onto bundleDir and refuses any result that
// resolves outside the bundle directory or targets reserved root files.
func resolveInBundle(bundleDir, relPath string) (string, error) {
	absBundle, err := filepath.Abs(bundleDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve bundle directory: %w", err)
	}

	cleanRel := filepath.Clean(relPath)
	full := filepath.Join(absBundle, cleanRel)
	rel, err := filepath.Rel(absBundle, full)
	if err != nil {
		return "", fmt.Errorf("failed to resolve concept path %q: %w", relPath, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal denied: concept path %q escapes bundle directory", relPath)
	}
	// Check reserved filenames on relative path (index.md anywhere, root log.md, root AGENTS.md)
	relBase := filepath.Base(cleanRel)
	normRel := filepath.ToSlash(rel)
	if rel == "." || cleanRel == "." ||
		strings.EqualFold(relBase, "index") || strings.EqualFold(relBase, "index.md") ||
		strings.EqualFold(normRel, "log.md") || strings.EqualFold(normRel, "AGENTS.md") {
		return "", fmt.Errorf("cannot write concept to reserved bundle file %q", relPath)
	}

	// Security: prevent symlink-based path traversal and arbitrary file overwrite
	realTarget, err := ensureWithinRoot(bundleDir, full)
	if err != nil {
		return "", err
	}

	// Check reserved filenames and file type on symlink-resolved target
	targetBase := filepath.Base(realTarget)
	realRoot, err := filepath.EvalSymlinks(bundleDir)
	if err != nil {
		realRoot, _ = filepath.Abs(bundleDir)
	} else {
		realRoot, _ = filepath.Abs(realRoot)
	}
	targetRel, _ := filepath.Rel(realRoot, realTarget)
	targetRel = filepath.ToSlash(targetRel)

	if strings.EqualFold(targetBase, "index.md") || strings.EqualFold(targetRel, "log.md") || strings.EqualFold(targetRel, "AGENTS.md") {
		return "", fmt.Errorf("cannot write concept to reserved bundle file %q", relPath)
	}
	if !strings.HasSuffix(strings.ToLower(realTarget), ".md") {
		return "", fmt.Errorf("concept target %q must be a markdown (.md) file", relPath)
	}

	return realTarget, nil
}

// sanitizeConceptMetadata validates that concept metadata fields do not contain
// newlines or frontmatter delimiters that could lead to YAML injection or delimiter smuggling.
func sanitizeConceptMetadata(c *Concept) error {
	if strings.TrimSpace(c.Type) == "" {
		return fmt.Errorf("concept type cannot be empty or whitespace")
	}
	if strings.TrimSpace(c.Title) == "" {
		return fmt.Errorf("concept title cannot be empty or whitespace")
	}
	if c.Description != "" && strings.TrimSpace(c.Description) == "" {
		return fmt.Errorf("concept description cannot be empty or whitespace")
	}

	fields := []struct {
		name  string
		value string
	}{
		{"type", c.Type},
		{"title", c.Title},
		{"description", c.Description},
	}
	if c.Generated != nil {
		fields = append(fields, struct {
			name  string
			value string
		}{"actor", c.Generated.By})
	}

	for _, f := range fields {
		if strings.ContainsAny(f.value, "\r\n") {
			return fmt.Errorf("concept %s cannot contain newlines", f.name)
		}
		if strings.Contains(f.value, "---") {
			return fmt.Errorf("concept %s cannot contain frontmatter delimiter '---'", f.name)
		}
	}
	return nil
}

// SaveConcept writes a concept file to disk and optionally executes automatic bookkeeping.
func SaveConcept(bundleDir string, c *Concept, isNew, autoLog, autoIndex bool, actor string) error {
	fullPath, err := resolveInBundle(bundleDir, c.Path)
	if err != nil {
		return err
	}

	// Update generated timestamp & actor
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "agent/okf-tool"
	}
	c.Generated = &Generated{
		By: actor,
		At: time.Now().UTC().Format(time.RFC3339),
	}

	if err := sanitizeConceptMetadata(c); err != nil {
		return err
	}

	c.Type = strings.TrimSpace(c.Type)
	c.Title = strings.TrimSpace(c.Title)
	if c.Description != "" {
		c.Description = strings.TrimSpace(c.Description)
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
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
	relationDesc = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(relationDesc, "\r", " "), "\n", " "))

	sourceID = strings.TrimSpace(strings.TrimSuffix(sourceID, ".md"))
	targetID = strings.TrimSpace(strings.TrimSuffix(targetID, ".md"))

	if err := ValidateConceptID(sourceID); err != nil {
		return fmt.Errorf("invalid source concept ID: %w", err)
	}
	if err := ValidateConceptID(targetID); err != nil {
		return fmt.Errorf("invalid target concept ID: %w", err)
	}
	if sourceID == targetID {
		return fmt.Errorf("cannot relate concept to itself (%s)", sourceID)
	}

	b, err := LoadBundle(bundleDir)
	if err != nil {
		return fmt.Errorf("failed to load bundle: %w", err)
	}

	srcConcept, ok := b.Concepts[sourceID]
	if !ok {
		return fmt.Errorf("source concept '%s' not found", sourceID)
	}

	tgtConcept, ok := b.Concepts[targetID]
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
	if hasRelationship(srcConcept.Body, relPath, relationDesc) {
		return nil
	}

	srcConcept.Body = insertRelationship(srcConcept.Body, strings.TrimPrefix(relStatement, "\n"))

	if err := SaveConcept(bundleDir, srcConcept, false, false, false, actor); err != nil {
		return fmt.Errorf("failed to save related concept: %w", err)
	}

	logDesc := fmt.Sprintf("Linked `%s` to `%s`.", srcConcept.Path, tgtConcept.Path)
	if relationDesc != "" {
		logDesc = fmt.Sprintf("Linked `%s` to `%s` (%s).", srcConcept.Path, tgtConcept.Path, relationDesc)
	}
	return AppendLogEntry(bundleDir, "Update", logDesc)
}

func hasRelationship(body, relPath, relationDesc string) bool {
	lines := strings.Split(body, "\n")
	start, end, ok := relatedSectionBounds(lines)
	if !ok {
		return false
	}
	inFence := false
	for _, line := range lines[start:end] {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if relationDesc == "" {
			if strings.HasPrefix(trimmed, "- Related to [") && strings.HasSuffix(trimmed, "]("+relPath+")") {
				return true
			}
			continue
		}
		if strings.HasPrefix(trimmed, "- [") && strings.HasSuffix(trimmed, "]("+relPath+"): "+relationDesc) {
			return true
		}
	}
	return false
}

func insertRelationship(body, relLine string) string {
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	_, end, ok := relatedSectionBounds(lines)
	if !ok {
		return strings.TrimRight(body, "\n") + "\n\n# Related Concepts\n" + relLine + "\n"
	}

	before := strings.TrimRight(strings.Join(lines[:end], "\n"), "\n")
	after := strings.TrimLeft(strings.Join(lines[end:], "\n"), "\n")
	if after == "" {
		return before + "\n" + relLine + "\n"
	}
	return before + "\n" + relLine + "\n\n" + after + "\n"
}

func relatedSectionBounds(lines []string) (int, int, bool) {
	inFence := false
	start := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if start == -1 {
			if trimmed == "# Related Concepts" || trimmed == "# Related" {
				start = i + 1
			}
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			return start, i, true
		}
	}
	if start != -1 {
		return start, len(lines), true
	}
	return 0, 0, false
}
