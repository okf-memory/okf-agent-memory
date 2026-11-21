package okf

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
)

var newlineReplacer = strings.NewReplacer("\r", " ", "\n", " ")

// frontmatterSmuggleRegex matches reserved frontmatter keys in a body line, including quoted keys
// and whitespace before the colon, which a plain prefix check would miss.
var frontmatterSmuggleRegex = regexp.MustCompile(`(?i)^\s*["']?(verified|governance|generated|type|status|superseded_by|code_refs|stale_after|x-okf-memory-protected)["']?\s*:`)

func titleCase(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// atomicWriteFile writes data to a temporary file in the same directory as targetPath,
// flushes it to disk, and atomically replaces targetPath using os.Rename.
func atomicWriteFile(targetPath string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", targetPath, err)
	}

	tmpFile, err := os.CreateTemp(dir, ".tmp-okf-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write to temporary file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to sync temporary file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("failed to set permissions on temporary file: %w", err)
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("failed to atomically replace %s: %w", targetPath, err)
	}
	return nil
}

// InitBundle initializes a new OKF v0.2 bundle with root index.md and log.md.
func InitBundle(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return WithBundleLock(dir, func() error {
		rootIndex := filepath.Join(dir, "index.md")
		if _, err := os.Stat(rootIndex); os.IsNotExist(err) {
			indexContent := "---\nokf_version: \"0.2\"\n---\n\n# Knowledge Base\n\n"
			if err := atomicWriteFile(rootIndex, []byte(indexContent), 0o644); err != nil {
				return fmt.Errorf("failed to write root index.md: %w", err)
			}
		}

		logFile := filepath.Join(dir, "log.md")
		if _, err := os.Stat(logFile); os.IsNotExist(err) {
			today := time.Now().UTC().Format("2006-01-02")
			logContent := fmt.Sprintf("## %s\n* **Creation**: Initialized OKF v0.2 knowledge bundle.\n", today)
			if err := atomicWriteFile(logFile, []byte(logContent), 0o644); err != nil {
				return fmt.Errorf("failed to write log.md: %w", err)
			}
		}

		return nil
	})
}

// ResolveStaleAfter parses a stale_after input string into a canonical YYYY-MM-DD UTC date.
// An empty string "" returns ("", "", nil) to indicate clearing the key.
// Supported formats: ISO date (YYYY-MM-DD), ISO 8601 timestamp (normalized to UTC date),
// or relative duration matching ParseRelativeDuration (e.g. "14d", "2w", "3m", "90d").
// Dates strictly before today (UTC) succeed and return a non-empty warning string.
func ResolveStaleAfter(raw string, now time.Time) (string, string, error) {
	if raw == "" {
		return "", "", nil
	}
	if strings.ContainsAny(raw, "\r\n") {
		return "", "", fmt.Errorf("invalid stale_after %q: cannot contain newlines", raw)
	}
	if strings.Contains(raw, "---") {
		return "", "", fmt.Errorf("invalid stale_after %q: cannot contain frontmatter delimiter '---'", raw)
	}

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", fmt.Errorf("invalid stale_after %q: whitespace-only value is not allowed (use empty string \"\" to clear)", raw)
	}

	nowUTC := now.UTC()
	var resolved string

	if t, err := time.Parse("2006-01-02", trimmed); err == nil {
		resolved = t.Format("2006-01-02")
	} else if t, ok := parseTimestamp(trimmed); ok {
		resolved = t.UTC().Format("2006-01-02")
	} else if d, err := ParseRelativeDuration(trimmed); err == nil {
		resolved = nowUTC.Add(d).Format("2006-01-02")
	} else {
		return "", "", fmt.Errorf("invalid stale_after %q: expected YYYY-MM-DD, ISO 8601 timestamp, or relative duration (e.g. 14d, 2w, 3m)", raw)
	}

	today := nowUTC.Format("2006-01-02")
	var warning string
	if resolved < today {
		warning = fmt.Sprintf("stale_after date %s is in the past (today is %s)", resolved, today)
	}
	return resolved, warning, nil
}

// AppendLogEntry prepends a new dated change entry to log.md.
func AppendLogEntry(bundleDir, entryType, description string) error {
	logPath := filepath.Join(bundleDir, "log.md")
	if _, err := ensureWithinRoot(bundleDir, logPath); err != nil {
		return err
	}
	return WithBundleLock(bundleDir, func() error {
		return appendLogEntryLocked(bundleDir, entryType, description)
	})
}

func appendLogEntryLocked(bundleDir, entryType, description string) error {
	entryType = strings.TrimSpace(newlineReplacer.Replace(entryType))
	description = strings.TrimSpace(newlineReplacer.Replace(description))
	today := time.Now().UTC().Format("2006-01-02")
	newEntry := fmt.Sprintf("* **%s**: %s\n", entryType, description)
	return insertLogLineLocked(bundleDir, today, newEntry)
}

func appendTimestampedLogEntryLocked(bundleDir string, at time.Time, entryType, description, changeNote string) error {
	entryType = strings.TrimSpace(newlineReplacer.Replace(entryType))
	description = strings.TrimSpace(newlineReplacer.Replace(description))
	changeNote = strings.TrimSpace(newlineReplacer.Replace(changeNote))

	atUTC := at.UTC()
	today := atUTC.Format("2006-01-02")
	ts := atUTC.Format(time.RFC3339)

	var newEntry string
	if changeNote != "" {
		newEntry = fmt.Sprintf("* **%s** [%s]: %s — %s\n", entryType, ts, description, changeNote)
	} else {
		newEntry = fmt.Sprintf("* **%s** [%s]: %s\n", entryType, ts, description)
	}
	return insertLogLineLocked(bundleDir, today, newEntry)
}

func insertLogLineLocked(bundleDir, dayHeading, newEntry string) error {
	logPath := filepath.Join(bundleDir, "log.md")
	if _, err := ensureWithinRoot(bundleDir, logPath); err != nil {
		return err
	}

	existingContent := ""
	// #nosec G304 -- logPath is guaranteed within bundleDir via ensureWithinRoot
	if data, err := os.ReadFile(logPath); err == nil {
		existingContent = string(data)
	}

	heading := fmt.Sprintf("## %s\n", dayHeading)
	if idx := strings.Index(existingContent, heading); idx != -1 {
		afterHeading := existingContent[idx+len(heading):]
		nextHeadingIdx := strings.Index(afterHeading, "\n## ")
		todaySection := afterHeading
		if nextHeadingIdx != -1 {
			todaySection = afterHeading[:nextHeadingIdx]
		}

		targetEntry := strings.TrimSpace(newEntry)
		for _, line := range strings.Split(todaySection, "\n") {
			if strings.TrimSpace(line) == targetEntry {
				// Exact entry already recorded today; avoid redundant duplicate lines
				return nil
			}
		}

		existingContent = existingContent[:idx+len(heading)] + newEntry + existingContent[idx+len(heading):]
	} else {
		// Prepend today heading
		existingContent = heading + newEntry + "\n" + strings.TrimLeft(existingContent, "\n")
	}

	// #nosec G703 -- logPath is validated and contained within bundle root
	return atomicWriteFile(logPath, []byte(existingContent), 0o644)
}

// ValidateConceptID verifies that a concept ID conforms to OKF naming conventions
// and does not attempt path traversal or target reserved bundle files.
func ValidateConceptID(id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return fmt.Errorf("concept ID cannot be empty")
	}

	for _, r := range trimmed {
		if unicode.IsControl(r) {
			return fmt.Errorf("concept ID %q contains forbidden control character U+%04X", id, r)
		}
		if unicode.In(r, unicode.Cf) {
			return fmt.Errorf("concept ID %q contains forbidden format/invisible character U+%04X", id, r)
		}
		if (r >= 0x202A && r <= 0x202E) || (r >= 0x2066 && r <= 0x2069) {
			return fmt.Errorf("concept ID %q contains forbidden bidirectional override character U+%04X", id, r)
		}
		if !unicode.IsPrint(r) {
			return fmt.Errorf("concept ID %q contains non-printable character U+%04X", id, r)
		}
	}

	cleanID := strings.TrimSuffix(trimmed, ".md")
	if cleanID == "" || cleanID == "." || cleanID == ".." {
		return fmt.Errorf("invalid concept ID %q", id)
	}

	if strings.HasPrefix(cleanID, "-") {
		return fmt.Errorf("concept ID %q cannot start with a hyphen -", id)
	}

	if IsAbsPath(cleanID) {
		return fmt.Errorf("concept ID %q must be a relative path", id)
	}

	// Split by '/' or '\' and inspect each path component
	parts := strings.FieldsFunc(cleanID, func(r rune) bool {
		return r == '/' || r == '\\'
	})

	if slices.Contains(parts, "..") {
		return fmt.Errorf("concept ID %q contains forbidden '..' traversal", id)
	}

	if len(parts) > MaxConceptDirectoryDepth {
		return fmt.Errorf("concept ID %q exceeds maximum directory depth of %d", id, MaxConceptDirectoryDepth)
	}

	for _, part := range parts {
		if part != "." && strings.HasPrefix(part, ".") {
			return fmt.Errorf("concept ID %q cannot contain hidden directory or dot-file component %q", id, part)
		}
	}

	// Clean path and ensure it does not escape
	cleaned := filepath.Clean(strings.ReplaceAll(cleanID, "\\", "/"))
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return fmt.Errorf("concept ID %q escapes bundle directory", id)
	}

	// Check for reserved filenames (index.md anywhere, root log.md, root AGENTS.md)
	base := filepath.Base(cleaned)
	normClean := strings.ReplaceAll(cleaned, "\\", "/")
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

	normConceptPath := strings.ReplaceAll(c.Path, "\\", "/")
	dir := path.Dir(normConceptPath)
	indexRelPath := "index.md"
	if dir != "." {
		indexRelPath = filepath.Join(filepath.FromSlash(dir), "index.md")
	}

	indexPath := filepath.Join(absBundle, filepath.Clean(indexRelPath))
	rel, err := filepath.Rel(absBundle, indexPath)
	relSlash := strings.ReplaceAll(rel, "\\", "/")
	if err != nil || relSlash == ".." || strings.HasPrefix(relSlash, "../") {
		return fmt.Errorf("path traversal denied: parent index %q escapes bundle directory", indexRelPath)
	}
	if _, err := ensureWithinRoot(bundleDir, indexPath); err != nil {
		return err
	}

	return WithBundleLock(bundleDir, func() error {
		return updateParentIndexLocked(bundleDir, c)
	})
}

// splitRawFrontmatter splits content into its exact raw frontmatter prefix
// (including both '---' delimiter lines and the trailing newline) and the remaining body.
// If content has no frontmatter block, prefix is empty and body is content unchanged.
func splitRawFrontmatter(content string) (prefix, body string) {
	offset := 0
	if strings.HasPrefix(content, "\ufeff") {
		offset = len("\ufeff")
	}
	rest := content[offset:]
	if !strings.HasPrefix(rest, "---") {
		return "", content
	}
	nlIdx := strings.IndexByte(rest, '\n')
	if nlIdx == -1 {
		return "", content
	}
	firstLine := strings.TrimRight(rest[:nlIdx], "\r")
	if firstLine != "---" {
		return "", content
	}

	pos := offset + nlIdx + 1
	for pos < len(content) {
		nextNL := strings.IndexByte(content[pos:], '\n')
		var lineEnd, afterLine int
		if nextNL == -1 {
			lineEnd = len(content)
			afterLine = len(content)
		} else {
			lineEnd = pos + nextNL
			afterLine = lineEnd + 1
		}
		if strings.TrimRight(content[pos:lineEnd], "\r") == "---" {
			return content[:afterLine], content[afterLine:]
		}
		pos = afterLine
	}
	return "", content
}

func updateParentIndexLocked(bundleDir string, c *Concept) error {
	absBundle, err := filepath.Abs(bundleDir)
	if err != nil {
		return fmt.Errorf("invalid bundle directory: %w", err)
	}

	normConceptPath := strings.ReplaceAll(c.Path, "\\", "/")
	dir := path.Dir(normConceptPath)
	indexRelPath := "index.md"
	if dir != "." {
		indexRelPath = filepath.Join(filepath.FromSlash(dir), "index.md")
	}

	indexPath := filepath.Join(absBundle, filepath.Clean(indexRelPath))
	rel, err := filepath.Rel(absBundle, indexPath)
	relSlash := strings.ReplaceAll(rel, "\\", "/")
	if err != nil || relSlash == ".." || strings.HasPrefix(relSlash, "../") {
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

	// Preserve any existing frontmatter verbatim (OKF v0.3 §8.1); only search/mutate body.
	fmPrefix, bodyPart := splitRawFrontmatter(existingContent)

	// Check if already listed in body
	linkTarget := fmt.Sprintf("(%s)", targetFilename)
	if strings.Contains(bodyPart, linkTarget) {
		// Update existing line
		lines := strings.Split(bodyPart, "\n")
		for i, l := range lines {
			if strings.Contains(l, linkTarget) {
				lines[i] = newListing
				break
			}
		}
		bodyPart = strings.Join(lines, "\n")
	} else {
		// Append listing
		if bodyPart == "" {
			if fmPrefix != "" && !strings.HasSuffix(fmPrefix, "\n") {
				bodyPart = "\n\n" + newListing + "\n"
			} else {
				bodyPart = "\n" + newListing + "\n"
			}
		} else {
			bodyPart = strings.TrimRight(bodyPart, "\n") + "\n" + newListing + "\n"
		}
	}
	existingContent = fmPrefix + bodyPart

	// #nosec G703 -- indexPath is verified within bundleDir
	return atomicWriteFile(indexPath, []byte(existingContent), 0o644)
}

// resolveInBundle joins relPath onto bundleDir and refuses any result that
// resolves outside the bundle directory or targets reserved root files.
func resolveInBundle(bundleDir, relPath string) (string, error) {
	absBundle, err := filepath.Abs(bundleDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve bundle directory: %w", err)
	}

	if IsAbsPath(relPath) {
		return "", fmt.Errorf("concept path %q must be a relative path", relPath)
	}

	// Normalize backslashes to forward slashes before calling filepath.Clean
	// to prevent Windows-style backslash traversal vectors (e.g. "..\..\file") on POSIX OS.
	normRel := strings.ReplaceAll(relPath, "\\", "/")
	cleanRel := filepath.Clean(normRel)
	full := filepath.Join(absBundle, cleanRel)
	rel, err := filepath.Rel(absBundle, full)
	if err != nil {
		return "", fmt.Errorf("failed to resolve concept path %q: %w", relPath, err)
	}
	relSlash := strings.ReplaceAll(rel, "\\", "/")
	if relSlash == ".." || strings.HasPrefix(relSlash, "../") {
		return "", fmt.Errorf("path traversal denied: concept path %q escapes bundle directory", relPath)
	}
	// Check reserved filenames on relative path (index.md anywhere, root log.md, root AGENTS.md)
	relBase := filepath.Base(cleanRel)
	normRel = filepath.ToSlash(rel)
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
		{"stale_after", c.StaleAfter},
		{"superseded_by", c.SupersededBy},
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

	// Security: prevent frontmatter smuggling in body (e.g. forging verified/governance via nested --- blocks)
	if c.Body != "" {
		lines := strings.Split(c.Body, "\n")
		inDelimiter := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "---" {
				inDelimiter = !inDelimiter
				continue
			}
			if inDelimiter {
				if trimmed == "" {
					continue
				}
				if match := frontmatterSmuggleRegex.FindStringSubmatch(trimmed); match != nil {
					return fmt.Errorf("concept body cannot smuggle frontmatter block containing %q", match[1])
				}
			}
		}
	}

	return nil
}

// SaveOptions controls bookkeeping and authoring metadata when persisting a concept.
type SaveOptions struct {
	IsNew          bool
	AutoLog        bool
	AutoIndex      bool
	Actor          string
	ChangeNote     string
	LogDetail      string
	LogEntryType   string
	AllowProtected bool
}

// SaveConcept writes a concept file to disk and optionally executes automatic bookkeeping.
// The entire mutation (concept file write + parent index update + log entry) executes
// inside a cross-process bundle write lock, and performs an optimistic concurrency check
// when persisting an existing concept loaded via LoadBundle.
func SaveConcept(bundleDir string, c *Concept, opts SaveOptions) error {
	if c.ID == "" && c.Path != "" {
		c.ID = strings.TrimSuffix(c.Path, ".md")
	}
	if err := ValidateConceptID(c.ID); err != nil {
		return fmt.Errorf("invalid concept ID: %w", err)
	}

	if _, err := resolveInBundle(bundleDir, c.Path); err != nil {
		return err
	}

	return WithBundleLock(bundleDir, func() error {
		return saveConceptLocked(bundleDir, c, opts)
	})
}

func saveConceptLocked(bundleDir string, c *Concept, opts SaveOptions) error {
	if c.ID == "" && c.Path != "" {
		c.ID = strings.TrimSuffix(c.Path, ".md")
	}
	if err := ValidateConceptID(c.ID); err != nil {
		return fmt.Errorf("invalid concept ID: %w", err)
	}

	fullPath, err := resolveInBundle(bundleDir, c.Path)
	if err != nil {
		return err
	}

	// Update generated timestamp & actor
	actor := strings.TrimSpace(opts.Actor)
	if actor == "" {
		actor = "agent/okf-tool"
	}
	if err := ensureWriteAllowed(fullPath, c, actor, opts.AllowProtected); err != nil {
		return err
	}
	if err := ensureNoForgedHumanVerification(fullPath, c, actor, opts.IsNew); err != nil {
		return err
	}

	// Stage 2b: Optimistic Concurrency Check (CAS) when updating a concept loaded from disk
	if !opts.IsNew && c.sourceHash != "" {
		// #nosec G304 -- fullPath is validated and confined to bundleDir by resolveInBundle
		if diskData, readErr := os.ReadFile(fullPath); readErr == nil {
			if hashConceptContent(string(diskData)) != c.sourceHash {
				return fmt.Errorf("%w: concept %q was modified on disk since it was loaded", ErrConcurrentModification, c.Path)
			}
		}
	}

	now := time.Now().UTC()
	var genExtra map[string]string
	if c.Generated != nil && len(c.Generated.Extra) > 0 {
		genExtra = c.Generated.Extra
	}
	c.Generated = &Generated{
		By:    actor,
		At:    now.Format(time.RFC3339),
		Extra: genExtra,
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
	if err := atomicWriteFile(fullPath, []byte(raw), 0o644); err != nil {
		return fmt.Errorf("failed to write concept: %w", err)
	}
	c.RawContent = raw
	if c.sourceHash != "" {
		c.sourceHash = hashConceptContent(raw)
	}

	// Automated Bookkeeping
	if opts.AutoIndex {
		if err := updateParentIndexLocked(bundleDir, c); err != nil {
			return fmt.Errorf("failed to update parent index: %w", err)
		}
	}

	if opts.AutoLog {
		entryType := opts.LogEntryType
		detail := strings.TrimSpace(opts.LogDetail)
		var desc string
		if opts.IsNew {
			if entryType == "" {
				entryType = "Creation"
			}
			if detail != "" {
				desc = fmt.Sprintf("Documented concept `%s` (%s); %s.", c.Path, c.Title, detail)
			} else {
				desc = fmt.Sprintf("Documented concept `%s` (%s).", c.Path, c.Title)
			}
		} else if strings.EqualFold(entryType, "Deprecation") {
			entryType = "Deprecation"
			if detail != "" {
				desc = fmt.Sprintf("Deprecated concept `%s` (%s).", c.Path, detail)
			} else {
				desc = fmt.Sprintf("Deprecated concept `%s`.", c.Path)
			}
		} else {
			if entryType == "" {
				entryType = "Update"
			}
			if detail != "" {
				desc = fmt.Sprintf("Updated concept `%s` (%s).", c.Path, detail)
			} else {
				desc = fmt.Sprintf("Updated concept `%s`.", c.Path)
			}
		}
		if err := appendTimestampedLogEntryLocked(bundleDir, now, entryType, desc, opts.ChangeNote); err != nil {
			return fmt.Errorf("failed to append log entry: %w", err)
		}
	}

	return nil
}

// UpdateConcept loads the latest concept state from bundleDir, applies mutateFn,
// and saves the updated concept (including optional index and log bookkeeping)
// atomically inside a single cross-process bundle write lock.
func UpdateConcept(bundleDir, conceptID string, opts SaveOptions, mutateFn func(c *Concept) error) (*Concept, error) {
	cleanID := strings.TrimSpace(strings.TrimSuffix(conceptID, ".md"))
	if err := ValidateConceptID(cleanID); err != nil {
		return nil, fmt.Errorf("invalid concept ID: %w", err)
	}

	var result *Concept
	err := WithBundleLock(bundleDir, func() error {
		b, err := LoadBundle(bundleDir)
		if err != nil {
			return fmt.Errorf("failed to load bundle: %w", err)
		}

		c, ok := b.Concepts[cleanID]
		if !ok {
			return fmt.Errorf("%w: '%s' in %s", ErrConceptNotFound, cleanID, bundleDir)
		}

		prevStatus := c.Status
		updated := *c
		if err := mutateFn(&updated); err != nil {
			return err
		}

		if opts.LogEntryType == "" && !strings.EqualFold(prevStatus, "deprecated") && strings.EqualFold(updated.Status, "deprecated") {
			opts.LogEntryType = "Deprecation"
		}

		opts.IsNew = false
		if err := saveConceptLocked(bundleDir, &updated, opts); err != nil {
			return err
		}
		result = &updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// RelateOptions controls bookkeeping and provenance when relating two concepts.
type RelateOptions struct {
	Actor      string
	AutoLog    bool
	ChangeNote string
}

// RelateConcepts creates a relative markdown link between source and target concepts.
func RelateConcepts(bundleDir, sourceID, targetID, relationDesc, actor string) error {
	return RelateConceptsWithOptions(bundleDir, sourceID, targetID, relationDesc, RelateOptions{
		Actor:   actor,
		AutoLog: true,
	})
}

// RelateConceptsWithOptions creates a relative markdown link between source and target concepts
// with optional log suppression and change note bookkeeping.
func RelateConceptsWithOptions(bundleDir, sourceID, targetID, relationDesc string, opts RelateOptions) error {
	relationDesc = strings.TrimSpace(newlineReplacer.Replace(relationDesc))

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

	return WithBundleLock(bundleDir, func() error {
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

		if err := saveConceptLocked(bundleDir, srcConcept, SaveOptions{
			IsNew:     false,
			AutoLog:   false,
			AutoIndex: false,
			Actor:     opts.Actor,
		}); err != nil {
			return fmt.Errorf("failed to save related concept: %w", err)
		}

		if !opts.AutoLog {
			return nil
		}

		logDesc := fmt.Sprintf("Linked `%s` to `%s`.", srcConcept.Path, tgtConcept.Path)
		if relationDesc != "" {
			logDesc = fmt.Sprintf("Linked `%s` to `%s` (%s).", srcConcept.Path, tgtConcept.Path, relationDesc)
		}
		return appendTimestampedLogEntryLocked(bundleDir, time.Now().UTC(), "Relate", logDesc, opts.ChangeNote)
	})
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
		clean := trimmed
		if strings.HasPrefix(clean, "- ") || strings.HasPrefix(clean, "* ") {
			clean = clean[2:]
		}
		if relationDesc == "" {
			if (strings.HasPrefix(trimmed, "- Related to [") || strings.HasPrefix(trimmed, "* Related to [") || strings.HasPrefix(clean, "Related to [") || strings.HasPrefix(clean, "[")) && strings.Contains(clean, "]("+relPath+")") {
				return true
			}
			continue
		}
		if (strings.HasPrefix(clean, "[") || strings.HasPrefix(trimmed, "- [") || strings.HasPrefix(trimmed, "* [")) && (strings.HasSuffix(trimmed, "]("+relPath+"): "+relationDesc) || strings.HasSuffix(clean, "]("+relPath+"): "+relationDesc)) {
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

func parseHeading(line string) (int, string) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "#") {
		return 0, ""
	}
	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level > 6 || level == len(trimmed) || trimmed[level] != ' ' {
		return 0, ""
	}
	title := strings.TrimSpace(trimmed[level:])
	return level, title
}

func isRelatedHeading(title string) bool {
	norm := strings.TrimSpace(strings.ToLower(title))
	for len(norm) > 0 {
		r := rune(norm[0])
		if (r >= '0' && r <= '9') || r == '.' || r == '-' || r == ')' || r == ' ' || r == ':' {
			norm = strings.TrimSpace(norm[1:])
		} else {
			break
		}
	}
	return norm == "related concepts" || norm == "related" || strings.HasPrefix(norm, "related concepts")
}

func relatedSectionBounds(lines []string) (int, int, bool) {
	inFence := false
	start := -1
	startLevel := 0
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
			if level, title := parseHeading(line); level > 0 && isRelatedHeading(title) {
				start = i + 1
				startLevel = level
			}
			continue
		}
		if level, _ := parseHeading(line); level > 0 && level <= startLevel {
			return start, i, true
		}
	}
	if start != -1 {
		return start, len(lines), true
	}
	return 0, 0, false
}
