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
	Concepts     map[string]*Concept `json:"concepts"`                   // concept ID -> Concept
	Indexes      map[string]string   `json:"indexes"`                    // relPath -> raw content
	LogContent   string              `json:"log_content,omitempty"`
	Graph        map[string][]string `json:"graph"`         // concept ID -> outbound concept IDs
	InboundGraph map[string][]string `json:"inbound_graph"` // concept ID -> inbound concept IDs
	BrokenLinks  []BrokenLink        `json:"broken_links,omitempty"`
	Orphans      []string            `json:"orphans,omitempty"`
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
	realRoot, err := filepath.EvalSymlinks(rootDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve bundle root: %w", err)
	}
	realRoot, err = filepath.Abs(realRoot)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path of bundle root: %w", err)
	}

	absTarget, err := filepath.Abs(targetPath)
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
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal denied: %q escapes bundle directory", targetPath)
	}

	return realTarget, nil
}

// LoadBundle loads all concepts, indexes, and logs from a bundle directory and builds the relationship graph.
func LoadBundle(root string) (*Bundle, error) {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("bundle directory does not exist: %w", err)
	}
	realRoot, err = filepath.Abs(realRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path of bundle root: %w", err)
	}

	info, err := os.Stat(realRoot)
	if err != nil {
		return nil, fmt.Errorf("bundle directory does not exist: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", root)
	}

	b := &Bundle{
		RootPath:     root,
		Concepts:     make(map[string]*Concept),
		Indexes:      make(map[string]string),
		Graph:        make(map[string][]string),
		InboundGraph: make(map[string][]string),
	}

	// 1. Walk directory and collect files
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path == root {
				return nil
			}
			rel, err := filepath.Rel(root, path)
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

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		// Security: prevent symlink following outside bundle directory
		if d.Type()&fs.ModeSymlink != 0 {
			realTarget, err := ensureWithinRoot(root, path)
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
					for _, line := range strings.Split(fm, "\n") {
						idx := strings.Index(line, ":")
						if idx != -1 && strings.TrimSpace(line[:idx]) == "okf_version" {
							b.DeclaredVer = unquote(line[idx+1:])
						}
					}
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
			if strings.Contains(href, "://") {
				continue // External URL
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

	// Compute orphans (degree 0 in concept graph when bundle has > 1 concept)
	if len(b.Concepts) > 1 {
		for id := range b.Concepts {
			if !linkedNodes[id] {
				b.Orphans = append(b.Orphans, id)
			}
		}
	}
}
