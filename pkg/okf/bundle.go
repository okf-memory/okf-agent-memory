package okf

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// StripFences removes ```...``` code blocks so illustrative links are ignored.
var fenceRegex = regexp.MustCompile("(?s)```.*?```")

func StripFences(text string) string {
	return fenceRegex.ReplaceAllString(text, "")
}

// LinkRegex matches markdown links `[label](href)`
var linkRegex = regexp.MustCompile(`\]\(([^)\s]+\.md)(?:#[^)]*)?\)`)

// Bundle represents an in-memory OKF bundle.
type Bundle struct {
	RootPath     string              `json:"root_path"`
	DeclaredVer  string              `json:"declared_version,omitempty"` // e.g. "0.2" from root index.md
	Scopes       []Scope             `json:"scopes,omitempty"`           // optional allowed memory scopes from root index.md
	Concepts     map[string]*Concept `json:"concepts"`                   // concept ID -> Concept
	Indexes      map[string]string   `json:"indexes"`                    // relPath -> raw content
	LogContent   string              `json:"log_content,omitempty"`
	Graph        map[string][]string `json:"graph"`         // concept ID -> outbound concept IDs
	InboundGraph map[string][]string `json:"inbound_graph"` // concept ID -> inbound concept IDs
	BrokenLinks  []BrokenLink        `json:"broken_links,omitempty"`
	Orphans      []string            `json:"orphans,omitempty"`
}

// AllowedScope reports whether the given scope is permitted by this bundle's configuration.
// If no scopes were explicitly configured in root index.md, all scopes are allowed.
func (b *Bundle) AllowedScope(s Scope) bool {
	if b == nil || len(b.Scopes) == 0 {
		return true
	}
	for _, allowed := range b.Scopes {
		if allowed == s {
			return true
		}
	}
	return false
}

// BrokenLink records a link from a concept to a non-existent target or reserved file.
type BrokenLink struct {
	SourceConcept string `json:"source_concept"`
	TargetHref    string `json:"target_href"`
	Reason        string `json:"reason"`
}

// ensureWithinRoot verifies that targetPath (resolving all symlinks) stays strictly
// within the canonical root directory. It returns the resolved absolute path or an error.
func ensureWithinRoot(rootDir, targetPath string) (string, error) {
	// Absolutise BEFORE resolving symlinks. EvalSymlinks on a relative path returns a
	// relative path, so a relative root under a symlinked cwd (macOS /tmp -> /private/tmp)
	// stayed unresolved while the target was resolved, and every legitimate write looked
	// like an escape.
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path of bundle root: %w", err)
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", fmt.Errorf("failed to resolve bundle root: %w", err)
	}

	cleanTarget := strings.ReplaceAll(targetPath, "\\", "/")
	if IsAbsPath(targetPath) {
		cleanTarget = filepath.Clean(cleanTarget)
	} else {
		cleanTarget = path.Join(filepath.ToSlash(realRoot), cleanTarget)
	}

	absTarget, err := filepath.Abs(filepath.FromSlash(cleanTarget))
	if err != nil {
		return "", err
	}

	// Walk up to find the closest ancestor that exists, and evaluate its symlinks
	curr := absTarget
	var missingParts []string
	for {
		_, lstatErr := os.Lstat(curr)
		if lstatErr == nil {
			break
		}
		missingParts = append([]string{filepath.Base(curr)}, missingParts...)
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}

	realCurr, err := filepath.EvalSymlinks(curr)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path %q: %w", curr, err)
	}
	realCurr, err = filepath.Abs(realCurr)
	if err != nil {
		return "", err
	}

	parts := append([]string{realCurr}, missingParts...)
	realTarget := filepath.Join(parts...)

	rel, err := filepath.Rel(realRoot, realTarget)
	relSlash := filepath.ToSlash(rel)
	if err != nil || relSlash == ".." || strings.HasPrefix(relSlash, "../") {
		return "", fmt.Errorf("path traversal denied: %q escapes bundle directory", targetPath)
	}

	return realTarget, nil
}

// LoadBundle loads all concepts, indexes, and logs from a bundle directory and builds the relationship graph.
// A bundle root that is a symlink must resolve to a location inside the directory containing the link.
func LoadBundle(root string) (*Bundle, error) {
	return loadBundle(root, false)
}

// loadTrustedBundle loads a bundle whose root the user configured explicitly (user and system scope).
// Such a root may be a symlink to any location; symlinks inside the bundle remain confined to it.
func loadTrustedBundle(root string) (*Bundle, error) {
	return loadBundle(root, true)
}

// resolveBundleRoot returns the canonical directory to walk and the root path to report for the bundle at root.
func resolveBundleRoot(root string, trustedRoot bool) (walkRoot, bundlePath string, err error) {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", fmt.Errorf("bundle directory does not exist: %w", err)
	}
	realRoot, err = filepath.Abs(realRoot)
	if err != nil {
		return "", "", fmt.Errorf("failed to get absolute path of bundle root: %w", err)
	}

	if !trustedRoot {
		if err := ensureRootWithinParent(root, realRoot); err != nil {
			return "", "", err
		}
	}

	info, err := os.Stat(realRoot)
	if err != nil {
		return "", "", fmt.Errorf("bundle directory does not exist: %w", err)
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("path is not a directory: %s", root)
	}

	// If root itself does not contain index.md, but contains a knowledge/ subdirectory,
	// resolve to the nested knowledge/ bundle directory (e.g. project root with DMAA layout).
	if _, err := os.Stat(filepath.Join(realRoot, "index.md")); !os.IsNotExist(err) {
		return realRoot, root, nil
	}
	if kInfo, kErr := os.Stat(filepath.Join(realRoot, "knowledge")); kErr != nil || !kInfo.IsDir() {
		return realRoot, root, nil
	}
	knowledgeRoot, err := ensureWithinRoot(realRoot, "knowledge")
	if err != nil {
		return "", "", err
	}
	return knowledgeRoot, filepath.Join(root, "knowledge"), nil
}

// ensureRootWithinParent rejects a bundle root whose symlink target lies outside the directory containing the link.
func ensureRootWithinParent(root, realRoot string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of bundle root: %w", err)
	}
	if _, err := ensureWithinRoot(filepath.Dir(absRoot), realRoot); err != nil {
		return fmt.Errorf("bundle root %q resolves outside its parent directory: %w", root, err)
	}
	return nil
}

// parseRootIndexFrontmatter extracts okf_version and optional scopes configuration from root index.md.
// If 'scopes' is present, it must be a non-empty list containing only canonical scopes (project, vendor, user, system).
func parseRootIndexFrontmatter(fmText string) (string, []Scope, error) {
	type fmBlock struct {
		inline string
		lines  []string
	}
	blocks := make(map[string]*fmBlock)
	var curBlock *fmBlock

	for _, line := range strings.Split(fmText, "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") || (curBlock != nil && strings.HasPrefix(line, "-")) {
			if curBlock != nil {
				curBlock.lines = append(curBlock.lines, line)
			}
			continue
		}
		before, after, ok := cutYAMLPair(line)
		if !ok {
			continue
		}
		key := unquote(before)
		val := strings.TrimSpace(after)
		curBlock = &fmBlock{inline: val}
		blocks[key] = curBlock
	}

	var declaredVer string
	if verBlock, ok := blocks["okf_version"]; ok {
		declaredVer = unquote(verBlock.inline)
	}

	scopesBlock, hasScopes := blocks["scopes"]
	if !hasScopes {
		return declaredVer, nil, nil
	}

	rawItems, err := parseStrictScopesList(scopesBlock.inline, scopesBlock.lines)
	if err != nil {
		return "", nil, err
	}
	if len(rawItems) == 0 {
		return "", nil, fmt.Errorf("invalid scopes in index.md: scopes list cannot be empty")
	}

	var scopes []Scope
	seen := make(map[Scope]bool)
	for _, raw := range rawItems {
		norm := Scope(strings.ToLower(strings.TrimSpace(raw)))
		switch norm {
		case ScopeProject, ScopeVendor, ScopeUser, ScopeSystem:
			if !seen[norm] {
				seen[norm] = true
				scopes = append(scopes, norm)
			}
		default:
			return "", nil, fmt.Errorf("invalid scope %q in index.md (allowed: project, vendor, user, system)", raw)
		}
	}
	return declaredVer, scopes, nil
}

func parseStrictScopesList(inline string, blockLines []string) ([]string, error) {
	var out []string
	s := strings.TrimSpace(inline)
	if s != "" {
		if strings.HasPrefix(s, "[") {
			if !strings.HasSuffix(s, "]") {
				return nil, fmt.Errorf("invalid scopes in index.md: malformed flow list %q", inline)
			}
			inner := strings.TrimSpace(s[1 : len(s)-1])
			if inner == "" {
				return nil, nil
			}
			for _, item := range splitFlowItems(inner) {
				it := unquote(item)
				if it == "" {
					return nil, fmt.Errorf("invalid scopes in index.md: empty item in scopes list")
				}
				out = append(out, it)
			}
			return out, nil
		}
		return []string{unquote(s)}, nil
	}

	for _, line := range blockLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		after, ok := strings.CutPrefix(trimmed, "-")
		if !ok {
			return nil, fmt.Errorf("invalid scopes in index.md: unexpected block line %q", trimmed)
		}
		val := unquote(strings.TrimSpace(after))
		if val == "" {
			return nil, fmt.Errorf("invalid scopes in index.md: empty item in scopes list")
		}
		out = append(out, val)
	}
	return out, nil
}

func loadBundle(root string, trustedRoot bool) (*Bundle, error) {
	walkRoot, bundlePath, err := resolveBundleRoot(root, trustedRoot)
	if err != nil {
		return nil, err
	}

	b := &Bundle{
		RootPath:     bundlePath,
		Concepts:     make(map[string]*Concept),
		Indexes:      make(map[string]string),
		Graph:        make(map[string][]string),
		InboundGraph: make(map[string][]string),
	}

	// 1. Walk directory and collect files
	err = filepath.WalkDir(walkRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path == walkRoot {
				return nil
			}
			rel, err := filepath.Rel(walkRoot, path)
			if err == nil && (rel == "." || rel == "") {
				return nil
			}
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		name := d.Name()
		if strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".md") {
			return nil
		}

		rel, err := filepath.Rel(walkRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		// Root AGENTS.md is the agent governance layer (DMAA L1), not an OKF concept
		if strings.EqualFold(rel, "AGENTS.md") {
			return nil
		}

		// Security: prevent symlink following outside bundle directory
		if d.Type()&fs.ModeSymlink != 0 {
			realTarget, err := ensureWithinRoot(walkRoot, path)
			if err != nil {
				return err
			}
			fi, err := os.Stat(realTarget)
			if err != nil {
				return fmt.Errorf("cannot stat symlink target %q: %w", rel, err)
			}
			if fi.IsDir() {
				return fmt.Errorf("symlink %q points to a directory, not a markdown file", rel)
			}
			if !strings.HasSuffix(strings.ToLower(realTarget), ".md") {
				return fmt.Errorf("symlink %q must point to a markdown (.md) file", rel)
			}
		}

		// #nosec G122,G304 -- path is strictly within bundle root and symlinks are resolved via ensureWithinRoot
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(data)

		if name == "index.md" {
			b.Indexes[rel] = content
			if rel == "index.md" {
				fm, _, hasFM := ExtractFrontmatter(content)
				if hasFM {
					ver, scopes, err := parseRootIndexFrontmatter(fm)
					if err != nil {
						return err
					}
					b.DeclaredVer = ver
					b.Scopes = scopes
				}
			}
			return nil
		}

		if name == "log.md" {
			if rel == "log.md" {
				b.LogContent = content
			}
			return nil
		}

		// Concept document
		c, parseErr := ParseConcept(rel, content)
		if parseErr != nil {
			// Save stub concept with raw content for validator reporting
			id := strings.TrimSuffix(rel, ".md")
			b.Concepts[id] = &Concept{
				ID:         id,
				Path:       rel,
				RawContent: content,
			}
		} else {
			b.Concepts[c.ID] = c
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 2. Build link graph & check connectivity
	b.buildGraph()
	return b, nil
}

// ResolveLink converts a link href from a source concept into a target concept ID.
func (b *Bundle) ResolveLink(sourceRelPath, href string) string {
	normSource := strings.ReplaceAll(sourceRelPath, "\\", "/")
	normHref := strings.ReplaceAll(href, "\\", "/")

	cleanHref, _, _ := strings.Cut(strings.Split(normHref, "#")[0], "?")
	if cleanHref == "" {
		return ""
	}

	var base string
	if after, ok := strings.CutPrefix(cleanHref, "/"); ok {
		base = after
	} else {
		srcDir := path.Dir(normSource)
		if srcDir == "." {
			base = cleanHref
		} else {
			base = path.Join(srcDir, cleanHref)
		}
	}

	cleanPath := path.Clean(base)
	cleanPath = strings.TrimPrefix(cleanPath, "/")
	return strings.TrimSuffix(cleanPath, ".md")
}

func (b *Bundle) buildGraph() {
	linkedNodes := make(map[string]bool)

	for id := range b.Concepts {
		b.Graph[id] = make([]string, 0)
		if _, ok := b.InboundGraph[id]; !ok {
			b.InboundGraph[id] = make([]string, 0)
		}
	}

	for id, concept := range b.Concepts {
		body := StripFences(concept.Body)
		matches := linkRegex.FindAllStringSubmatch(body, -1)

		for _, match := range matches {
			href := match[1]
			if IsExternalLink(href) {
				linkedNodes[id] = true // Outbound external reference prevents false orphan flag
				continue               // External URL or Qualified Scope URI: hermetic CI ignores remote target
			}

			targetID := b.ResolveLink(concept.Path, href)
			targetRel := targetID + ".md"
			targetBase := path.Base(targetRel)

			if strings.EqualFold(targetBase, "index.md") || strings.EqualFold(targetRel, "log.md") || strings.EqualFold(targetRel, "AGENTS.md") {
				b.BrokenLinks = append(b.BrokenLinks, BrokenLink{
					SourceConcept: concept.Path,
					TargetHref:    href,
					Reason:        "reserved index.md/log.md/AGENTS.md is navigation, not a concept",
				})
				continue
			}

			if _, exists := b.Concepts[targetID]; exists {
				if targetID != id {
					b.Graph[id] = append(b.Graph[id], targetID)
					b.InboundGraph[targetID] = append(b.InboundGraph[targetID], id)
					linkedNodes[id] = true
					linkedNodes[targetID] = true
				}
			} else {
				b.BrokenLinks = append(b.BrokenLinks, BrokenLink{
					SourceConcept: concept.Path,
					TargetHref:    href,
					Reason:        "target concept does not exist",
				})
			}
		}
	}

	// Also check for broken links inside directory indexes (index.md)
	for idxPath, idxContent := range b.Indexes {
		body := StripFences(idxContent)
		matches := linkRegex.FindAllStringSubmatch(body, -1)

		for _, match := range matches {
			href := match[1]
			if IsExternalLink(href) {
				continue // External URL or Qualified Scope URI: hermetic CI ignores remote target
			}

			targetID := b.ResolveLink(idxPath, href)
			targetRel := targetID + ".md"
			targetBase := path.Base(targetRel)

			if strings.EqualFold(targetBase, "index.md") || strings.EqualFold(targetRel, "log.md") || strings.EqualFold(targetRel, "AGENTS.md") {
				continue // Reserved files in indexes are standard navigation
			}

			if _, exists := b.Concepts[targetID]; !exists {
				b.BrokenLinks = append(b.BrokenLinks, BrokenLink{
					SourceConcept: idxPath,
					TargetHref:    href,
					Reason:        "target concept does not exist",
				})
			}
		}
	}

	// Compute orphans (degree 0 in concept graph when bundle has > 1 concept)
	if len(b.Concepts) > 1 {
		for id := range b.Concepts {
			if !linkedNodes[id] {
				b.Orphans = append(b.Orphans, id)
			}
		}
	}
}
