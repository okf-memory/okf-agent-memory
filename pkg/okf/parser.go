package okf

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// ExtractFrontmatter splits a markdown file into its raw frontmatter string and body.
func ExtractFrontmatter(content string) (frontmatter, body string, hasFM bool) {
	// Strip BOM if present
	if after, ok := strings.CutPrefix(content, "\ufeff"); ok {
		content = after
	}

	if !strings.HasPrefix(content, "---") {
		return "", content, false
	}

	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		return "", content, false
	}

	firstLine := strings.TrimRight(lines[0], "\r")
	if firstLine != "---" {
		return "", content, false
	}

	fmEndIdx := -1
	for i := 1; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		if line == "---" {
			fmEndIdx = i
			break
		}
	}

	if fmEndIdx == -1 {
		return "", content, false
	}

	fmText := strings.Join(lines[1:fmEndIdx], "\n")
	bodyText := strings.Join(lines[fmEndIdx+1:], "\n")
	return fmText, bodyText, true
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return strings.TrimSpace(s[1 : len(s)-1])
		}
	}
	return s
}

// ParseFlowMapping parses a `{ by: "foo", at: "bar" }` string.
func ParseFlowMapping(s string) map[string]string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
		s = s[1 : len(s)-1]
	}
	out := make(map[string]string)
	parts := strings.Split(s, ",")
	for _, part := range parts {
		before, after, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		k := strings.TrimSpace(before)
		v := unquote(after)
		if k != "" {
			out[k] = v
		}
	}
	return out
}

// ParseStringList parses `[a, b, c]` or block list items.
func ParseStringList(inline string, blockLines []string) []string {
	var out []string
	if inline != "" {
		s := strings.TrimSpace(inline)
		if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
			s = s[1 : len(s)-1]
			for _, item := range strings.Split(s, ",") {
				it := unquote(item)
				if it != "" {
					out = append(out, it)
				}
			}
			return out
		}
		if s != "" {
			out = append(out, unquote(s))
			return out
		}
	}

	for _, line := range blockLines {
		trimmed := strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(trimmed, "-"); ok {
			val := strings.TrimSpace(after)
			val = unquote(val)
			if val != "" {
				out = append(out, val)
			}
		}
	}
	return out
}

// ParseConcept parses a concept's raw text and relative path into a Concept struct.
func ParseConcept(relPath, content string) (*Concept, error) {
	id := strings.TrimSuffix(relPath, ".md")
	fmText, body, hasFM := ExtractFrontmatter(content)
	if !hasFM {
		return nil, fmt.Errorf("%s: missing YAML frontmatter block", relPath)
	}

	c := &Concept{
		ID:         id,
		Path:       relPath,
		Body:       body,
		RawContent: content,
		Extra:      make(map[string]any),
	}

	// Parse top-level frontmatter blocks
	type fmBlock struct {
		inline string
		lines  []string
	}
	blocks := make(map[string]*fmBlock)
	var curBlock *fmBlock

	scanner := bufio.NewScanner(strings.NewReader(fmText))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") || (curBlock != nil && strings.HasPrefix(line, "-")) {
			if curBlock != nil {
				curBlock.lines = append(curBlock.lines, line)
			}
			continue
		}

		before, after, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}

		key := strings.TrimSpace(before)
		val := strings.TrimSpace(after)
		curBlock = &fmBlock{inline: val}
		blocks[key] = curBlock
	}

	for k, b := range blocks {
		switch k {
		case "type":
			c.Type = unquote(b.inline)
		case "title":
			c.Title = unquote(b.inline)
		case "description":
			c.Description = unquote(b.inline)
		case "resource":
			c.Resource = unquote(b.inline)
		case "status":
			c.Status = unquote(b.inline)
		case "stale_after":
			c.StaleAfter = unquote(b.inline)
		case "tags":
			c.Tags = ParseStringList(b.inline, b.lines)
		case "generated":
			if b.inline != "" && strings.HasPrefix(b.inline, "{") {
				m := ParseFlowMapping(b.inline)
				c.Generated = &Generated{By: m["by"], At: m["at"]}
			} else {
				m := parseBlockMapping(b.lines)
				if len(m) > 0 {
					c.Generated = &Generated{By: m["by"], At: m["at"]}
				}
			}
		case "verified":
			if b.inline != "" && strings.HasPrefix(b.inline, "{") {
				m := ParseFlowMapping(b.inline)
				c.Verified = append(c.Verified, Verified{By: m["by"], At: m["at"]})
			} else if b.inline != "" && strings.HasPrefix(b.inline, "[") {
				// List of flow pairs
				inner := strings.TrimPrefix(strings.TrimSuffix(b.inline, "]"), "[")
				for _, part := range strings.Split(inner, "},") {
					part = strings.TrimSpace(part)
					if !strings.HasSuffix(part, "}") {
						part += "}"
					}
					m := ParseFlowMapping(part)
					if m["by"] != "" || m["at"] != "" {
						c.Verified = append(c.Verified, Verified{By: m["by"], At: m["at"]})
					}
				}
			} else {
				items := parseListOfMappings(b.lines)
				for _, item := range items {
					c.Verified = append(c.Verified, Verified{By: item["by"], At: item["at"]})
				}
			}
		case "sources":
			items := parseListOfMappings(b.lines)
			for _, it := range items {
				src := Source{
					ID:           it["id"],
					Resource:     it["resource"],
					Title:        it["title"],
					Author:       it["author"],
					LastModified: it["last_modified"],
				}
				if countStr, ok := it["usage_count"]; ok {
					if cnt, err := strconv.Atoi(countStr); err == nil {
						src.UsageCount = cnt
					}
				}
				c.Sources = append(c.Sources, src)
			}
		default:
			if len(b.lines) == 0 {
				c.Extra[k] = unquote(b.inline)
			} else {
				c.Extra[k] = b.lines
			}
		}
	}

	return c, nil
}

func parseBlockMapping(lines []string) map[string]string {
	out := make(map[string]string)
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		trimmed = strings.TrimPrefix(trimmed, "-")
		idx := strings.Index(trimmed, ":")
		if idx != -1 {
			k := strings.TrimSpace(trimmed[:idx])
			v := unquote(trimmed[idx+1:])
			out[k] = v
		}
	}
	return out
}

func parseListOfMappings(lines []string) []map[string]string {
	var out []map[string]string
	var cur map[string]string

	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "-") {
			cur = make(map[string]string)
			out = append(out, cur)
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			if strings.HasPrefix(trimmed, "{") {
				fm := ParseFlowMapping(trimmed)
				for k, v := range fm {
					cur[k] = v
				}
				continue
			}
		}

		idx := strings.Index(trimmed, ":")
		if idx != -1 && cur != nil {
			k := strings.TrimSpace(trimmed[:idx])
			v := unquote(trimmed[idx+1:])
			cur[k] = v
		}
	}
	return out
}

// SerializeConcept converts a Concept into standard OKF Markdown with YAML frontmatter.
func SerializeConcept(c *Concept) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "type: %s\n", c.Type)

	if c.Title != "" {
		fmt.Fprintf(&sb, "title: %s\n", c.Title)
	}
	if c.Description != "" {
		fmt.Fprintf(&sb, "description: %s\n", c.Description)
	}
	if c.Resource != "" {
		fmt.Fprintf(&sb, "resource: %s\n", c.Resource)
	}
	if len(c.Tags) > 0 {
		fmt.Fprintf(&sb, "tags: [%s]\n", strings.Join(c.Tags, ", "))
	}
	if c.Generated != nil {
		fmt.Fprintf(&sb, "generated: { by: %s, at: %s }\n", c.Generated.By, c.Generated.At)
	}
	if len(c.Verified) > 0 {
		if len(c.Verified) == 1 {
			fmt.Fprintf(&sb, "verified: { by: %s, at: %s }\n", c.Verified[0].By, c.Verified[0].At)
		} else {
			sb.WriteString("verified:\n")
			for _, v := range c.Verified {
				fmt.Fprintf(&sb, "  - { by: %s, at: %s }\n", v.By, v.At)
			}
		}
	}
	if c.Status != "" {
		fmt.Fprintf(&sb, "status: %s\n", c.Status)
	}
	if c.StaleAfter != "" {
		fmt.Fprintf(&sb, "stale_after: %s\n", c.StaleAfter)
	}
	if len(c.Sources) > 0 {
		sb.WriteString("sources:\n")
		for _, s := range c.Sources {
			fmt.Fprintf(&sb, "  - resource: %s\n", s.Resource)
			if s.ID != "" {
				fmt.Fprintf(&sb, "    id: %s\n", s.ID)
			}
			if s.Title != "" {
				fmt.Fprintf(&sb, "    title: %s\n", s.Title)
			}
			if s.Author != "" {
				fmt.Fprintf(&sb, "    author: %s\n", s.Author)
			}
			if s.LastModified != "" {
				fmt.Fprintf(&sb, "    last_modified: %s\n", s.LastModified)
			}
			if s.UsageCount > 0 {
				fmt.Fprintf(&sb, "    usage_count: %d\n", s.UsageCount)
			}
		}
	}

	// Preserve extra unknown fields
	for k, v := range c.Extra {
		switch val := v.(type) {
		case string:
			fmt.Fprintf(&sb, "%s: %s\n", k, val)
		case []string:
			fmt.Fprintf(&sb, "%s:\n", k)
			for _, l := range val {
				fmt.Fprintf(&sb, "  %s\n", l)
			}
		default:
			fmt.Fprintf(&sb, "%s: %v\n", k, val)
		}
	}

	sb.WriteString("---\n\n")
	sb.WriteString(strings.TrimSpace(c.Body))
	sb.WriteString("\n")

	return sb.String()
}
