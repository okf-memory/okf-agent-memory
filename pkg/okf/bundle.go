package okf

import (
	"fmt"
	"io/fs"
	"os"
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

// LoadBundle loads all concepts, indexes, and logs from a bundle directory and builds the relationship graph.
func LoadBundle(root string) (*Bundle, error) {
	info, err := os.Stat(root)
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
			name := d.Name()
			if (strings.HasPrefix(name, ".") && name != ".") || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		name := filepath.Base(rel)

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
			b.LogContent = content
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
	cleanHref, _, _ := strings.Cut(strings.Split(href, "#")[0], "?")
	if cleanHref == "" {
		return ""
	}

	var base string
	if after, ok := strings.CutPrefix(cleanHref, "/"); ok {
		base = after
	} else {
		srcDir := filepath.Dir(sourceRelPath)
		if srcDir == "." {
			base = cleanHref
		} else {
			base = filepath.Join(srcDir, cleanHref)
		}
	}

	cleanPath := filepath.Clean(filepath.ToSlash(base))
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
			targetBase := filepath.Base(targetRel)

			if targetBase == "index.md" || targetBase == "log.md" {
				b.BrokenLinks = append(b.BrokenLinks, BrokenLink{
					SourceConcept: concept.Path,
					TargetHref:    href,
					Reason:        "reserved index.md/log.md is navigation, not a concept",
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
