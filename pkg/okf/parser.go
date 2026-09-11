package okf

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
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
		if s[0] == '"' && s[len(s)-1] == '"' {
			var unquoted string
			if err := json.Unmarshal([]byte(s), &unquoted); err == nil {
				return unquoted
			}
			return strings.TrimSpace(s[1 : len(s)-1])
		}
		if s[0] == '\'' && s[len(s)-1] == '\'' {
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
	parts := splitFlowItems(s)
	for _, part := range parts {
		before, after, ok := cutYAMLPair(part)
		if !ok {
			continue
		}
		k := unquote(before)
		v := unquote(after)
		if k != "" {
			out[k] = v
		}
	}
	return out
}

func cutYAMLPair(s string) (string, string, bool) {
	var quote rune
	escaped := false
	for i, r := range s {
		if quote != 0 {
			if quote == '"' && escaped {
				escaped = false
				continue
			}
			if quote == '"' && r == '\\' {
				escaped = true
				continue
			}
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == ':' {
			return s[:i], s[i+1:], true
		}
	}
	return s, "", false
}

// ParseStringList parses `[a, b, c]` or block list items.
func ParseStringList(inline string, blockLines []string) []string {
	var out []string
	if inline != "" {
		s := strings.TrimSpace(inline)
		if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
			s = s[1 : len(s)-1]
			for _, item := range splitFlowItems(s) {
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

// splitFlowItems separates comma-delimited YAML flow values while preserving
// commas inside quoted strings and nested flow collections.
func splitFlowItems(s string) []string {
	var parts []string
	start := 0
	depth := 0
	var quote rune
	escaped := false

	for i, r := range s {
		if quote != 0 {
			if quote == '"' && escaped {
				escaped = false
				continue
			}
			if quote == '"' && r == '\\' {
				escaped = true
				continue
			}
			if r == quote {
				quote = 0
			}
			continue
		}

		switch r {
		case '\'', '"':
			quote = r
		case '{', '[':
			depth++
		case '}', ']':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// ParseConcept parses a concept's raw text and relative path into a Concept struct.
func ParseConcept(relPath, content string) (*Concept, error) {
	id := strings.TrimSuffix(relPath, ".md")
	fmText, body, hasFM := ExtractFrontmatter(content)
	if !hasFM {
		return nil, fmt.Errorf("%s: missing YAML frontmatter block", relPath)
	}

	c := &Concept{
		ID:          id,
		Path:        relPath,
		Body:        body,
		RawContent:  content,
		Extra:       make(map[string]any),
		extraBlocks: make(map[string]bool),
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

		before, after, ok := cutYAMLPair(line)
		if !ok {
			continue
		}

		key := unquote(before)
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
				for _, part := range splitFlowItems(inner) {
					part = strings.TrimSpace(part)
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
				c.Extra[k] = parseExtraValue(b.inline)
			} else {
				c.Extra[k] = normalizeBlockLines(b.lines)
				c.extraBlocks[k] = true
			}
		}
	}

	return c, nil
}

func normalizeBlockLines(lines []string) []string {
	minIndent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
	}
	if minIndent < 0 {
		minIndent = 0
	}

	normalized := make([]string, len(lines))
	for i, line := range lines {
		if len(line) >= minIndent {
			line = line[minIndent:]
		}
		normalized[i] = line
	}
	return normalized
}

func parseExtraValue(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var value any
	if json.Unmarshal([]byte(s), &value) == nil {
		return value
	}
	return unquote(s)
}

func parseBlockMapping(lines []string) map[string]string {
	out := make(map[string]string)
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		trimmed = strings.TrimPrefix(trimmed, "-")
		before, after, ok := cutYAMLPair(trimmed)
		if ok {
			k := unquote(before)
			v := unquote(after)
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

		before, after, ok := cutYAMLPair(trimmed)
		if ok && cur != nil {
			k := unquote(before)
			v := unquote(after)
			cur[k] = v
		}
	}
	return out
}

// safeYAMLString sanitizes a string scalar for inclusion in YAML frontmatter.
// If the string contains newlines, quotes, colons, or YAML special characters,
// it is JSON-quoted to prevent frontmatter injection and syntax errors.
func safeYAMLString(s string) string {
	if s == "" {
		return ""
	}
	needsQuote := strings.ContainsAny(s, "\n\r\":{}[]#&*!|>'%@`,?") ||
		strings.HasPrefix(s, "-") ||
		strings.HasPrefix(s, " ") ||
		strings.HasSuffix(s, " ")

	if needsQuote {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(s); err == nil {
			return strings.TrimSpace(buf.String())
		}
	}
	return s
}

func safeYAMLKey(s string) string {
	plain := s != ""
	for i, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || (i > 0 && r >= '0' && r <= '9') || (i > 0 && (r == '-' || r == '.')) {
			continue
		}
		plain = false
		break
	}
	if plain {
		switch strings.ToLower(s) {
		case "null", "true", "false", "yes", "no", "on", "off":
			plain = false
		}
	}
	if plain {
		return s
	}
	encoded, _ := json.Marshal(s)
	return string(encoded)
}

// SerializeConcept converts a Concept into standard OKF Markdown with YAML frontmatter.
func SerializeConcept(c *Concept) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "type: %s\n", safeYAMLString(c.Type))

	if c.Title != "" {
		fmt.Fprintf(&sb, "title: %s\n", safeYAMLString(c.Title))
	}
	if c.Description != "" {
		fmt.Fprintf(&sb, "description: %s\n", safeYAMLString(c.Description))
	}
	if c.Resource != "" {
		fmt.Fprintf(&sb, "resource: %s\n", safeYAMLString(c.Resource))
	}
	if len(c.Tags) > 0 {
		quotedTags := make([]string, len(c.Tags))
		for i, t := range c.Tags {
			quotedTags[i] = safeYAMLString(t)
		}
		fmt.Fprintf(&sb, "tags: [%s]\n", strings.Join(quotedTags, ", "))
	}
	if c.Generated != nil {
		fmt.Fprintf(&sb, "generated: { by: %s, at: %s }\n", safeYAMLString(c.Generated.By), safeYAMLString(c.Generated.At))
	}
	if len(c.Verified) > 0 {
		if len(c.Verified) == 1 {
			fmt.Fprintf(&sb, "verified: { by: %s, at: %s }\n", safeYAMLString(c.Verified[0].By), safeYAMLString(c.Verified[0].At))
		} else {
			sb.WriteString("verified:\n")
			for _, v := range c.Verified {
				fmt.Fprintf(&sb, "  - { by: %s, at: %s }\n", safeYAMLString(v.By), safeYAMLString(v.At))
			}
		}
	}
	if c.Status != "" {
		fmt.Fprintf(&sb, "status: %s\n", safeYAMLString(c.Status))
	}
	if c.StaleAfter != "" {
		fmt.Fprintf(&sb, "stale_after: %s\n", safeYAMLString(c.StaleAfter))
	}
	if len(c.Sources) > 0 {
		sb.WriteString("sources:\n")
		for _, s := range c.Sources {
			fmt.Fprintf(&sb, "  - resource: %s\n", safeYAMLString(s.Resource))
			if s.ID != "" {
				fmt.Fprintf(&sb, "    id: %s\n", safeYAMLString(s.ID))
			}
			if s.Title != "" {
				fmt.Fprintf(&sb, "    title: %s\n", safeYAMLString(s.Title))
			}
			if s.Author != "" {
				fmt.Fprintf(&sb, "    author: %s\n", safeYAMLString(s.Author))
			}
			if s.LastModified != "" {
				fmt.Fprintf(&sb, "    last_modified: %s\n", safeYAMLString(s.LastModified))
			}
			if s.UsageCount > 0 {
				fmt.Fprintf(&sb, "    usage_count: %d\n", s.UsageCount)
			}
		}
	}

	// Preserve extra unknown fields in canonical key order. String values are
	// always JSON-quoted (valid YAML) so their scalar type and content survive a
	// round trip without allowing frontmatter delimiter injection.
	extraKeys := make([]string, 0, len(c.Extra))
	for k := range c.Extra {
		extraKeys = append(extraKeys, k)
	}
	sort.Strings(extraKeys)
	for _, k := range extraKeys {
		v := c.Extra[k]
		safeKey := safeYAMLKey(k)
		if c.extraBlocks[k] {
			if lines, ok := v.([]string); ok {
				fmt.Fprintf(&sb, "%s:\n", safeKey)
				for _, line := range lines {
					line = strings.ReplaceAll(strings.ReplaceAll(line, "\r", " "), "\n", " ")
					fmt.Fprintf(&sb, "  %s\n", line)
				}
				continue
			}
		}
		switch val := v.(type) {
		case string:
			encoded, _ := json.Marshal(val)
			fmt.Fprintf(&sb, "%s: %s\n", safeKey, encoded)
		default:
			encoded, err := json.Marshal(val)
			if err != nil {
				encoded, _ = json.Marshal(fmt.Sprint(val))
			}
			fmt.Fprintf(&sb, "%s: %s\n", safeKey, encoded)
		}
	}

	sb.WriteString("---\n\n")
	sb.WriteString(strings.TrimSpace(c.Body))
	sb.WriteString("\n")

	return sb.String()
}
