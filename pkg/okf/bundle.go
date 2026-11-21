package okf

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// StripFences removes ```...``` code blocks so illustrative links are ignored.
var fenceRegex = regexp.MustCompile("(?s)```.*?```")

func StripFences(text string) string {
	return fenceRegex.ReplaceAllString(text, "")
}

// LinkRegex matches markdown links `[label](href.md)`
var linkRegex = regexp.MustCompile(`\]\(([^)\s]+\.md)(?:#[^)]*)?\)`)

// allLinkRegex matches any standard markdown link or image `[label](href)` / `![alt](href)`.
var allLinkRegex = regexp.MustCompile(`\]\(([^)\s]+)\)`)

// ErrSupersededCycle is returned when following superseded_by links encounters a cycle.
var ErrSupersededCycle = errors.New("cyclic superseded_by chain detected")

// SupersededResolution describes the outcome of following a concept's superseded_by chain.
type SupersededResolution struct {
	Target    *Concept // Final resolved concept (may be the starting concept if not superseded)
	Chain     []string // Concept IDs traversed, starting with the original ID and ending with Target.ID
	Forwarded bool     // True if Target.ID != original ID
}

// Bundle represents an in-memory OKF bundle.
type Bundle struct {
	RootPath        string              `json:"root_path"`
	DeclaredVer     string              `json:"declared_version,omitempty"` // e.g. "0.3" from root index.md okf_version
	Title           string              `json:"title,omitempty"`            // optional bundle title from root index.md (§8.1)
	Description     string              `json:"description,omitempty"`      // optional bundle description from root index.md (§8.1)
	License         string              `json:"license,omitempty"`          // optional SPDX or path license from root index.md (§8.1)
	Extra           map[string]any      `json:"extra,omitempty"`            // optional extension keys from root index.md (§8.1)
	Scopes          []Scope             `json:"scopes,omitempty"`           // optional allowed memory scopes from root index.md
	Concepts        map[string]*Concept `json:"concepts"`                   // concept ID -> Concept
	Indexes         map[string]string   `json:"indexes"`                    // relPath -> raw content
	Assets          []string            `json:"assets,omitempty"`           // non-Markdown bundle-relative asset paths (§3.2)
	SkippedMarkdown []string            `json:"skipped_markdown,omitempty"` // .md files without frontmatter or type (§3.2)
	LogContent      string              `json:"log_content,omitempty"`
	Graph           map[string][]string `json:"graph"`         // concept ID -> outbound concept IDs
	InboundGraph    map[string][]string `json:"inbound_graph"` // concept ID -> inbound concept IDs
	BrokenLinks     []BrokenLink        `json:"broken_links,omitempty"`
	Orphans         []string            `json:"orphans,omitempty"`
}

// IsV02OrEarlier reports whether this bundle declares OKF version 0.2 or earlier,
// or declares no version at all (OKF v0.3 §12.1 compatibility fallback trigger).
func (b *Bundle) IsV02OrEarlier() bool {
	if b == nil {
		return true
	}
	v := strings.TrimSpace(b.DeclaredVer)
	if v == "" {
		return true
	}
	v = strings.TrimPrefix(strings.TrimPrefix(v, "v"), "V")
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		major, err := strconv.Atoi(parts[0])
		if err != nil {
			return false
		}
		return major == 0
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return false
	}
	return major == 0 && minor <= 2
}

// HasAsset reports whether relPath (bundle-relative, with or without leading '/')
// refers to a non-Markdown asset file present in the bundle.
func (b *Bundle) HasAsset(relPath string) bool {
	if b == nil || len(b.Assets) == 0 {
		return false
	}
	norm := strings.TrimPrefix(path.Clean(strings.ReplaceAll(relPath, "\\", "/")), "/")
	return slices.Contains(b.Assets, norm)
}

func (b *Bundle) isSkippedMarkdown(relPath string) bool {
	if b == nil || len(b.SkippedMarkdown) == 0 {
		return false
	}
	norm := strings.TrimPrefix(path.Clean(strings.ReplaceAll(relPath, "\\", "/")), "/")
	return slices.Contains(b.SkippedMarkdown, norm)
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

type rootIndexMeta struct {
	declaredVer string
	title       string
	description string
	license     string
	scopes      []Scope
	extra       map[string]any
}

// parseRootIndexFrontmatter extracts OKF bundle metadata (okf_version, title, description, license),
// optional scopes configuration, and extension keys from root index.md (§8.1).
// If 'scopes' is present, it must be a non-empty list containing only canonical scopes (project, vendor, user, system).
func parseRootIndexFrontmatter(fmText string) (rootIndexMeta, error) {
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

	var meta rootIndexMeta
	parseOptionalScalar := func(raw string) string {
		if isYAMLNull(raw) {
			return ""
		}
		return unquote(raw)
	}

	for key, blk := range blocks {
		switch key {
		case "okf_version":
			meta.declaredVer = parseOptionalScalar(blk.inline)
		case "title":
			meta.title = parseOptionalScalar(blk.inline)
		case "description":
			meta.description = parseOptionalScalar(blk.inline)
		case "license":
			meta.license = parseOptionalScalar(blk.inline)
		case "scopes":
			// Handled below with strict validation.
		default:
			if meta.extra == nil {
				meta.extra = make(map[string]any)
			}
			if len(blk.lines) == 0 {
				meta.extra[key] = parseExtraValue(blk.inline)
			} else if items, ok := parseBlockList(blk.lines); ok {
				meta.extra[key] = items
			} else {
				meta.extra[key] = normalizeBlockLines(blk.lines)
			}
		}
	}

	scopesBlock, hasScopes := blocks["scopes"]
	if !hasScopes {
		return meta, nil
	}

	rawItems, err := parseStrictScopesList(scopesBlock.inline, scopesBlock.lines)
	if err != nil {
		return rootIndexMeta{}, err
	}
	if len(rawItems) == 0 {
		return rootIndexMeta{}, fmt.Errorf("invalid scopes in index.md: scopes list cannot be empty")
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
			return rootIndexMeta{}, fmt.Errorf("invalid scope %q in index.md (allowed: project, vendor, user, system)", raw)
		}
	}
	meta.scopes = scopes
	return meta, nil
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
		if strings.HasPrefix(name, ".") {
			return nil
		}

		rel, err := filepath.Rel(walkRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		// Non-Markdown files are linkable assets, not concepts (OKF v0.3 §3.2).
		if !strings.HasSuffix(name, ".md") {
			if d.Type()&fs.ModeSymlink != 0 {
				if _, err := ensureWithinRoot(walkRoot, path); err != nil {
					return nil // Do not follow symlinks outside bundle root (§3.2 rule 5)
				}
			}
			b.Assets = append(b.Assets, rel)
			return nil
		}

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
					meta, err := parseRootIndexFrontmatter(fm)
					if err != nil {
						return err
					}
					b.DeclaredVer = meta.declaredVer
					b.Title = meta.title
					b.Description = meta.description
					b.License = meta.license
					b.Extra = meta.extra
					b.Scopes = meta.scopes
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

		// OKF v0.3 §3.2: A .md file without frontmatter or without a non-empty 'type'
		// is not a concept (e.g. README.md, AGENTS.md, CONTRIBUTING.md, SKILL.md).
		// Consumers skip it cleanly without treating it as a concept or orphan.
		if _, _, hasFM := ExtractFrontmatter(content); !hasFM {
			b.SkippedMarkdown = append(b.SkippedMarkdown, rel)
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
				sourceHash: hashConceptContent(content),
			}
		} else if strings.TrimSpace(c.Type) == "" {
			b.SkippedMarkdown = append(b.SkippedMarkdown, rel)
		} else {
			c.sourceHash = hashConceptContent(content)
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

// ResolveSuperseded follows superseded_by references from startID to the final active
// replacement concept (OKF v0.3 §5.4). It only follows superseded_by on deprecated
// concepts, detects cycles without infinite looping, and reports broken or escaping targets.
func (b *Bundle) ResolveSuperseded(startID string) (*SupersededResolution, error) {
	if b == nil {
		return nil, fmt.Errorf("%w: %q", ErrConceptNotFound, startID)
	}
	curr, ok := b.Concepts[startID]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrConceptNotFound, startID)
	}

	visited := map[string]bool{startID: true}
	chain := []string{startID}

	for strings.EqualFold(curr.Status, "deprecated") && strings.TrimSpace(curr.SupersededBy) != "" {
		rp := b.ResolvePath(curr.Path, curr.SupersededBy)
		if rp.Kind == PathKindEscapes {
			return &SupersededResolution{
				Target:    curr,
				Chain:     chain,
				Forwarded: len(chain) > 1,
			}, rp.Err
		}
		if rp.Kind != PathKindConcept {
			return &SupersededResolution{
				Target:    curr,
				Chain:     chain,
				Forwarded: len(chain) > 1,
			}, fmt.Errorf("superseded_by target %q from %q does not resolve to an in-bundle concept", curr.SupersededBy, curr.ID)
		}
		if visited[rp.ConceptID] {
			cycleChain := append(append([]string{}, chain...), rp.ConceptID)
			return &SupersededResolution{
				Target:    curr,
				Chain:     cycleChain,
				Forwarded: len(chain) > 1,
			}, fmt.Errorf("%w: %s", ErrSupersededCycle, strings.Join(cycleChain, " -> "))
		}

		next := b.Concepts[rp.ConceptID]
		visited[next.ID] = true
		chain = append(chain, next.ID)
		curr = next
	}

	return &SupersededResolution{
		Target:    curr,
		Chain:     chain,
		Forwarded: len(chain) > 1,
	}, nil
}

func (b *Bundle) buildGraph() {
	linkedNodes := make(map[string]bool)
	edgeSeen := make(map[string]map[string]bool)

	for id := range b.Concepts {
		b.Graph[id] = make([]string, 0)
		if _, ok := b.InboundGraph[id]; !ok {
			b.InboundGraph[id] = make([]string, 0)
		}
		edgeSeen[id] = make(map[string]bool)
	}

	addEdge := func(fromID, toID string) {
		if fromID == toID {
			return
		}
		if !edgeSeen[fromID][toID] {
			edgeSeen[fromID][toID] = true
			b.Graph[fromID] = append(b.Graph[fromID], toID)
			b.InboundGraph[toID] = append(b.InboundGraph[toID], fromID)
		}
		linkedNodes[fromID] = true
		linkedNodes[toID] = true
	}

	for id, concept := range b.Concepts {
		// 1. Body Markdown links (§6.1)
		body := StripFences(concept.Body)
		matches := allLinkRegex.FindAllStringSubmatch(body, -1)

		for _, match := range matches {
			href := match[1]
			rp := b.ResolvePath(concept.Path, href)

			switch rp.Kind {
			case PathKindEmpty, PathKindFragment:
				continue
			case PathKindExternal:
				linkedNodes[id] = true // Outbound external reference prevents false orphan flag
				continue
			case PathKindEscapes:
				b.BrokenLinks = append(b.BrokenLinks, BrokenLink{
					SourceConcept: concept.Path,
					TargetHref:    href,
					Reason:        "path traversal denied: target escapes bundle root",
				})
			case PathKindReserved:
				b.BrokenLinks = append(b.BrokenLinks, BrokenLink{
					SourceConcept: concept.Path,
					TargetHref:    href,
					Reason:        "reserved index.md/log.md/AGENTS.md is navigation, not a concept",
				})
			case PathKindConcept:
				addEdge(id, rp.ConceptID)
			case PathKindAsset, PathKindSkippedMD, PathKindDirectory:
				// Valid link to an asset, non-concept Markdown file, or directory (§6.1); not a concept edge.
				continue
			case PathKindMissing:
				if strings.HasSuffix(strings.ToLower(rp.RelPath), ".md") {
					b.BrokenLinks = append(b.BrokenLinks, BrokenLink{
						SourceConcept: concept.Path,
						TargetHref:    href,
						Reason:        "target concept does not exist",
					})
				} else if strings.Contains(rp.RelPath, "/") || strings.Contains(rp.RelPath, ".") {
					b.BrokenLinks = append(b.BrokenLinks, BrokenLink{
						SourceConcept: concept.Path,
						TargetHref:    href,
						Reason:        "target asset does not exist",
					})
				}
			}
		}

		// 2. Frontmatter path fields (§6.2 & §6.4): directed concept edges & confinement checks
		if concept.SupersededBy != "" {
			rp := b.ResolvePath(concept.Path, concept.SupersededBy)
			switch rp.Kind {
			case PathKindConcept:
				addEdge(id, rp.ConceptID)
			case PathKindEscapes:
				b.BrokenLinks = append(b.BrokenLinks, BrokenLink{
					SourceConcept: concept.Path,
					TargetHref:    concept.SupersededBy,
					Reason:        "path traversal denied: superseded_by escapes bundle root",
				})
			case PathKindExternal, PathKindEmpty, PathKindFragment:
				// Non-local target; validated separately if needed.
			default:
				b.BrokenLinks = append(b.BrokenLinks, BrokenLink{
					SourceConcept: concept.Path,
					TargetHref:    concept.SupersededBy,
					Reason:        "superseded_by target concept does not exist",
				})
			}
		}

		type frontmatterPathField struct {
			name string
			val  string
		}
		var pathFields []frontmatterPathField
		if concept.Resource != "" {
			pathFields = append(pathFields, frontmatterPathField{"resource", concept.Resource})
		}
		for _, src := range concept.Sources {
			if isPathLikeSourceResource(src.Resource) {
				pathFields = append(pathFields, frontmatterPathField{"sources.resource", src.Resource})
			}
		}
		if concept.Attestation != nil {
			if concept.Attestation.Computation != "" {
				pathFields = append(pathFields, frontmatterPathField{"computation", concept.Attestation.Computation})
			}
			if concept.Attestation.Executor != nil && concept.Attestation.Executor.Resource != "" {
				pathFields = append(pathFields, frontmatterPathField{"executor.resource", concept.Attestation.Executor.Resource})
			}
			if concept.Attestation.Attester != nil && concept.Attestation.Attester.Resource != "" {
				pathFields = append(pathFields, frontmatterPathField{"attester.resource", concept.Attestation.Attester.Resource})
			}
		}

		for _, pf := range pathFields {
			rp := b.ResolvePath(concept.Path, pf.val)
			switch rp.Kind {
			case PathKindConcept:
				addEdge(id, rp.ConceptID)
			case PathKindEscapes:
				// In v0.2 bundles, sources[].resource commonly cites parent-repo docs (e.g. ../../docs/...);
				// treat those as external provenance (never read locally) rather than broken bundle links.
				if pf.name == "sources.resource" && b.IsV02OrEarlier() {
					continue
				}
				b.BrokenLinks = append(b.BrokenLinks, BrokenLink{
					SourceConcept: concept.Path,
					TargetHref:    pf.val,
					Reason:        fmt.Sprintf("path traversal denied: %s escapes bundle root", pf.name),
				})
			}
		}
	}

	// Also check for broken links inside directory indexes (index.md)
	for idxPath, idxContent := range b.Indexes {
		body := StripFences(idxContent)
		matches := allLinkRegex.FindAllStringSubmatch(body, -1)

		for _, match := range matches {
			href := match[1]
			rp := b.ResolvePath(idxPath, href)

			switch rp.Kind {
			case PathKindEmpty, PathKindFragment, PathKindExternal, PathKindReserved,
				PathKindConcept, PathKindAsset, PathKindSkippedMD, PathKindDirectory:
				continue
			case PathKindEscapes:
				b.BrokenLinks = append(b.BrokenLinks, BrokenLink{
					SourceConcept: idxPath,
					TargetHref:    href,
					Reason:        "path traversal denied: target escapes bundle root",
				})
			case PathKindMissing:
				if strings.HasSuffix(strings.ToLower(rp.RelPath), ".md") {
					b.BrokenLinks = append(b.BrokenLinks, BrokenLink{
						SourceConcept: idxPath,
						TargetHref:    href,
						Reason:        "target concept does not exist",
					})
				}
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
