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

func isYAMLNull(raw string) bool {
	s := strings.TrimSpace(raw)
	return s == "null" || s == "Null" || s == "NULL" || s == "~"
}

func isExplicitEmptyScalar(raw string, blockLines []string) bool {
	if len(blockLines) > 0 {
		return false
	}
	s := strings.TrimSpace(raw)
	return s == "" || s == `""` || s == "''"
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

	addWarn := func(msg string) {
		c.ParseWarnings = append(c.ParseWarnings, msg)
	}
	parseOptionalScalar := func(field, raw string, lines []string) string {
		if isYAMLNull(raw) {
			addWarn(fmt.Sprintf("'%s' has null value; treated as absent", field))
			return ""
		}
		if isExplicitEmptyScalar(raw, lines) {
			addWarn(fmt.Sprintf("'%s' is empty; treated as absent", field))
			return ""
		}
		return unquote(raw)
	}
	parseFaultTolerantList := func(field, raw string, lines []string) []string {
		if isYAMLNull(raw) {
			addWarn(fmt.Sprintf("'%s' has null value; treated as absent", field))
			return nil
		}
		if isExplicitEmptyScalar(raw, lines) {
			addWarn(fmt.Sprintf("'%s' is empty; treated as absent", field))
			return nil
		}
		trimmed := strings.TrimSpace(raw)
		if trimmed != "" && !strings.HasPrefix(trimmed, "[") {
			// OKF v0.3 §11.2: scalar where list expected is a 1-item list; do NOT split at commas.
			val := unquote(trimmed)
			if val == "" {
				return nil
			}
			addWarn(fmt.Sprintf("'%s' is a scalar instead of a list; treated as single-item list", field))
			return []string{val}
		}
		return ParseStringList(raw, lines)
	}
	ensureAttestation := func() *AttestedComputation {
		if c.Attestation == nil {
			c.Attestation = &AttestedComputation{}
		}
		return c.Attestation
	}

	for k, b := range blocks {
		switch k {
		case "type":
			if !isYAMLNull(b.inline) {
				c.Type = unquote(b.inline)
			}
		case "title":
			c.Title = parseOptionalScalar("title", b.inline, b.lines)
		case "description":
			c.Description = parseOptionalScalar("description", b.inline, b.lines)
		case "resource":
			c.Resource = parseOptionalScalar("resource", b.inline, b.lines)
		case "status":
			c.Status = parseOptionalScalar("status", b.inline, b.lines)
		case "superseded_by":
			c.SupersededBy = parseOptionalScalar("superseded_by", b.inline, b.lines)
		case "governance":
			c.Governance = parseOptionalScalar("governance", b.inline, b.lines)
		case "code_refs":
			c.CodeRefs = parseFaultTolerantList("code_refs", b.inline, b.lines)
		case "stale_after":
			if isExplicitEmptyScalar(b.inline, b.lines) {
				addWarn("'stale_after' is empty; treated as absent")
				c.StaleAfter = ""
			} else {
				// Preserve literal "null" for validator warning compatibility (TestStaleAfterNullPreservedForValidation).
				c.StaleAfter = unquote(b.inline)
			}
		case "tags":
			c.Tags = parseFaultTolerantList("tags", b.inline, b.lines)
		case "generated":
			if isYAMLNull(b.inline) {
				addWarn("'generated' has null value; treated as absent")
				continue
			}
			if isExplicitEmptyScalar(b.inline, b.lines) {
				addWarn("'generated' is empty; treated as absent")
				continue
			}
			var m map[string]string
			if b.inline != "" && strings.HasPrefix(b.inline, "{") {
				m = ParseFlowMapping(b.inline)
			} else {
				m = parseBlockMapping(b.lines)
			}
			if len(m) > 0 {
				gen := &Generated{By: m["by"], At: m["at"]}
				for mk, mv := range m {
					if mk == "by" || mk == "at" {
						continue
					}
					if gen.Extra == nil {
						gen.Extra = make(map[string]string)
					}
					gen.Extra[mk] = mv
				}
				c.Generated = gen
			}
		case "verified":
			if isYAMLNull(b.inline) {
				addWarn("'verified' has null value; treated as absent")
				continue
			}
			if isExplicitEmptyScalar(b.inline, b.lines) {
				addWarn("'verified' is empty; treated as absent")
				continue
			}
			var rawItems []map[string]string
			if b.inline != "" && strings.HasPrefix(b.inline, "{") {
				rawItems = append(rawItems, ParseFlowMapping(b.inline))
			} else if b.inline != "" && strings.HasPrefix(b.inline, "[") {
				inner := strings.TrimPrefix(strings.TrimSuffix(b.inline, "]"), "[")
				for _, part := range splitFlowItems(inner) {
					part = strings.TrimSpace(part)
					if part == "" {
						continue
					}
					m := ParseFlowMapping(part)
					if len(m) > 0 {
						rawItems = append(rawItems, m)
					}
				}
			} else if len(b.lines) > 0 {
				rawItems = parseListOfMappingsOrSingle(b.lines, nil)
			}
			for _, item := range rawItems {
				v := Verified{By: item["by"], At: item["at"]}
				for mk, mv := range item {
					if mk == "by" || mk == "at" {
						continue
					}
					if v.Extra == nil {
						v.Extra = make(map[string]string)
					}
					v.Extra[mk] = mv
				}
				if v.By != "" || v.At != "" || len(v.Extra) > 0 {
					c.Verified = append(c.Verified, v)
				}
			}
		case "sources":
			if isYAMLNull(b.inline) {
				addWarn("'sources' has null value; treated as absent")
				continue
			}
			if isExplicitEmptyScalar(b.inline, b.lines) {
				addWarn("'sources' is empty; treated as absent")
				continue
			}
			var items []map[string]string
			if b.inline != "" && strings.HasPrefix(b.inline, "{") {
				addWarn("'sources' is a single mapping instead of a list; treated as single-item list")
				items = []map[string]string{ParseFlowMapping(b.inline)}
			} else if b.inline != "" && strings.HasPrefix(b.inline, "[") {
				inner := strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(b.inline, "]"), "["))
				if inner != "" {
					for _, part := range splitFlowItems(inner) {
						part = strings.TrimSpace(part)
						if part == "" {
							continue
						}
						if strings.HasPrefix(part, "{") {
							items = append(items, ParseFlowMapping(part))
						} else {
							items = append(items, map[string]string{"resource": unquote(part)})
						}
					}
				}
			} else if b.inline != "" {
				addWarn("'sources' is a scalar instead of a list; treated as single-item list")
				items = []map[string]string{{"resource": unquote(b.inline)}}
			} else {
				items = parseListOfMappingsOrSingle(b.lines, func() {
					addWarn("'sources' is a single mapping instead of a list; treated as single-item list")
				})
			}
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
				for key, value := range it {
					switch key {
					case "id", "resource", "title", "author", "last_modified", "usage_count":
						continue
					}
					if src.Extra == nil {
						src.Extra = map[string]string{}
					}
					src.Extra[key] = value
				}
				c.Sources = append(c.Sources, src)
			}
		case "runtime":
			if val := parseOptionalScalar("runtime", b.inline, b.lines); val != "" {
				ensureAttestation().Runtime = val
			}
		case "computation":
			if val := parseOptionalScalar("computation", b.inline, b.lines); val != "" {
				ensureAttestation().Computation = val
			}
		case "executor":
			if exec := parseExecutorBlock(b.inline, b.lines, addWarn); exec != nil {
				ensureAttestation().Executor = exec
			}
		case "attester":
			if att := parseAttesterBlock(b.inline, b.lines, addWarn); att != nil {
				ensureAttestation().Attester = att
			}
		case "parameters":
			if params := parseParametersBlock(b.inline, b.lines, addWarn); len(params) > 0 {
				ensureAttestation().Parameters = params
			}
		default:
			c.setExtra(k, b.inline, b.lines)
		}
	}

	sort.Strings(c.ParseWarnings)
	return c, nil
}

// setExtra stores an unknown frontmatter field. Scalars and lists become typed values; any other
// block structure is kept as verbatim lines so it survives a round trip unchanged.
func (c *Concept) setExtra(key, inline string, blockLines []string) {
	if len(blockLines) == 0 {
		c.Extra[key] = parseExtraValue(inline)
		return
	}
	if items, ok := parseBlockList(blockLines); ok {
		c.Extra[key] = items
		return
	}
	c.Extra[key] = normalizeBlockLines(blockLines)
	c.extraBlocks[key] = true
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
	if items, ok := parseFlowList(s); ok {
		return items
	}
	if m, ok := parseFlowMappingAny(s); ok {
		return m
	}
	return unquote(s)
}

func parseFlowMappingAny(s string) (map[string]any, bool) {
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return nil, false
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	out := make(map[string]any)
	if inner == "" {
		return out, true
	}
	for _, part := range splitFlowItems(inner) {
		before, after, ok := cutYAMLPair(part)
		if !ok {
			return nil, false
		}
		k := unquote(before)
		if k == "" {
			continue
		}
		out[k] = parseExtraValue(after)
	}
	return out, true
}

// parseFlowList parses a YAML flow sequence such as `[a, 'b, c', 3]` into typed items.
func parseFlowList(s string) ([]any, bool) {
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil, false
	}
	items := []any{}
	for _, part := range splitFlowItems(s[1 : len(s)-1]) {
		if part = strings.TrimSpace(part); part != "" {
			items = append(items, parseExtraValue(part))
		}
	}
	return items, true
}

// parseBlockList parses a block sequence of scalar items (`- a`) with uniform indentation into typed
// items. Sequences of mappings, nested sequences, or other structures report false.
func parseBlockList(lines []string) ([]any, bool) {
	items := []any{}
	indent := -1
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		rest, isItem := strings.CutPrefix(trimmed, "-")
		if !isItem {
			return nil, false
		}
		lineIndent := len(line) - len(strings.TrimLeft(line, " \t"))
		if indent != -1 && lineIndent != indent {
			return nil, false
		}
		indent = lineIndent

		item := strings.TrimSpace(rest)
		if item == "" || item == "-" || strings.HasPrefix(item, "- ") || isMappingEntry(item) {
			return nil, false
		}
		items = append(items, parseExtraValue(item))
	}
	return items, len(items) > 0
}

// isMappingEntry reports whether s is a `key: value` pair rather than a plain scalar such as a URL.
func isMappingEntry(s string) bool {
	_, after, ok := cutYAMLPair(s)
	return ok && (after == "" || after[0] == ' ')
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

func parseListOfMappingsOrSingle(lines []string, onSingle func()) []map[string]string {
	hasListDash := false
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "-") {
			hasListDash = true
		}
		break
	}
	if !hasListDash {
		m := parseBlockMapping(lines)
		if len(m) == 0 {
			return nil
		}
		if onSingle != nil {
			onSingle()
		}
		return []map[string]string{m}
	}
	return parseListOfMappings(lines)
}

func parseExecutorBlock(inline string, blockLines []string, warn func(string)) *Executor {
	if isYAMLNull(inline) {
		warn("'executor' has null value; treated as absent")
		return nil
	}
	if isExplicitEmptyScalar(inline, blockLines) {
		warn("'executor' is empty; treated as absent")
		return nil
	}
	exec := &Executor{}
	if inline != "" {
		if strings.HasPrefix(inline, "{") {
			m := ParseFlowMapping(inline)
			for k, v := range m {
				switch k {
				case "resource":
					exec.Resource = v
				case "receipt":
					exec.Receipt = ParseStringList(v, nil)
				default:
					if exec.Extra == nil {
						exec.Extra = make(map[string]string)
					}
					exec.Extra[k] = v
				}
			}
			return exec
		}
		warn("'executor' is a scalar instead of a mapping; treated as executor.resource")
		exec.Resource = unquote(inline)
		return exec
	}

	var curKey string
	var receiptLines []string
	flushReceipt := func() {
		if len(receiptLines) > 0 {
			exec.Receipt = append(exec.Receipt, ParseStringList("", receiptLines)...)
			receiptLines = nil
		}
	}
	for _, l := range blockLines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if curKey == "receipt" && strings.HasPrefix(trimmed, "-") {
			receiptLines = append(receiptLines, l)
			continue
		}
		flushReceipt()
		before, after, ok := cutYAMLPair(trimmed)
		if !ok {
			continue
		}
		k := unquote(before)
		v := strings.TrimSpace(after)
		curKey = k
		switch k {
		case "resource":
			if !isYAMLNull(v) {
				exec.Resource = unquote(v)
			}
		case "receipt":
			if v != "" && !isYAMLNull(v) {
				exec.Receipt = ParseStringList(v, nil)
			}
		default:
			if exec.Extra == nil {
				exec.Extra = make(map[string]string)
			}
			exec.Extra[k] = unquote(v)
		}
	}
	flushReceipt()
	if exec.Resource == "" && len(exec.Receipt) == 0 && len(exec.Extra) == 0 {
		return nil
	}
	return exec
}

func parseAttesterBlock(inline string, blockLines []string, warn func(string)) *Attester {
	if isYAMLNull(inline) {
		warn("'attester' has null value; treated as absent")
		return nil
	}
	if isExplicitEmptyScalar(inline, blockLines) {
		warn("'attester' is empty; treated as absent")
		return nil
	}
	att := &Attester{}
	if inline != "" {
		if strings.HasPrefix(inline, "{") {
			m := ParseFlowMapping(inline)
			for k, v := range m {
				if k == "resource" {
					att.Resource = v
				} else {
					if att.Extra == nil {
						att.Extra = make(map[string]string)
					}
					att.Extra[k] = v
				}
			}
			return att
		}
		warn("'attester' is a scalar instead of a mapping; treated as attester.resource")
		att.Resource = unquote(inline)
		return att
	}
	m := parseBlockMapping(blockLines)
	for k, v := range m {
		if k == "resource" {
			if !isYAMLNull(v) {
				att.Resource = v
			}
		} else {
			if att.Extra == nil {
				att.Extra = make(map[string]string)
			}
			att.Extra[k] = v
		}
	}
	if att.Resource == "" && len(att.Extra) == 0 {
		return nil
	}
	return att
}

func parseParametersBlock(inline string, blockLines []string, warn func(string)) []Parameter {
	if isYAMLNull(inline) {
		warn("'parameters' has null value; treated as absent")
		return nil
	}
	if isExplicitEmptyScalar(inline, blockLines) {
		warn("'parameters' is empty; treated as absent")
		return nil
	}
	var rawItems []map[string]string
	if inline != "" && strings.HasPrefix(inline, "{") {
		warn("'parameters' is a single mapping instead of a list; treated as single-item list")
		rawItems = []map[string]string{ParseFlowMapping(inline)}
	} else if inline != "" && strings.HasPrefix(inline, "[") {
		inner := strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(inline, "]"), "["))
		if inner != "" {
			for _, part := range splitFlowItems(inner) {
				part = strings.TrimSpace(part)
				if strings.HasPrefix(part, "{") {
					rawItems = append(rawItems, ParseFlowMapping(part))
				}
			}
		}
	} else if len(blockLines) > 0 {
		rawItems = parseListOfMappingsOrSingle(blockLines, func() {
			warn("'parameters' is a single mapping instead of a list; treated as single-item list")
		})
	}
	var out []Parameter
	for _, it := range rawItems {
		p := Parameter{
			Name:     it["name"],
			Type:     it["type"],
			Required: strings.EqualFold(strings.TrimSpace(it["required"]), "true"),
		}
		for k, v := range it {
			switch k {
			case "name", "type", "required":
				continue
			}
			if p.Extra == nil {
				p.Extra = make(map[string]string)
			}
			p.Extra[k] = v
		}
		if p.Name != "" || p.Type != "" || len(p.Extra) > 0 {
			out = append(out, p)
		}
	}
	return out
}

func formatFlowPairs(base []string, extra map[string]string) string {
	parts := append([]string{}, base...)
	if len(extra) > 0 {
		keys := make([]string, 0, len(extra))
		for k := range extra {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s: %s", safeYAMLKey(k), safeYAMLString(extra[k])))
		}
	}
	return "{ " + strings.Join(parts, ", ") + " }"
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
		base := []string{
			fmt.Sprintf("by: %s", safeYAMLString(c.Generated.By)),
			fmt.Sprintf("at: %s", safeYAMLString(c.Generated.At)),
		}
		fmt.Fprintf(&sb, "generated: %s\n", formatFlowPairs(base, c.Generated.Extra))
	}
	if len(c.Verified) > 0 {
		if len(c.Verified) == 1 {
			v := c.Verified[0]
			base := []string{
				fmt.Sprintf("by: %s", safeYAMLString(v.By)),
				fmt.Sprintf("at: %s", safeYAMLString(v.At)),
			}
			fmt.Fprintf(&sb, "verified: %s\n", formatFlowPairs(base, v.Extra))
		} else {
			sb.WriteString("verified:\n")
			for _, v := range c.Verified {
				base := []string{
					fmt.Sprintf("by: %s", safeYAMLString(v.By)),
					fmt.Sprintf("at: %s", safeYAMLString(v.At)),
				}
				fmt.Fprintf(&sb, "  - %s\n", formatFlowPairs(base, v.Extra))
			}
		}
	}
	if c.Status != "" {
		fmt.Fprintf(&sb, "status: %s\n", safeYAMLString(c.Status))
	}
	if c.SupersededBy != "" {
		fmt.Fprintf(&sb, "superseded_by: %s\n", safeYAMLString(c.SupersededBy))
	}
	if c.Governance != "" {
		fmt.Fprintf(&sb, "governance: %s\n", safeYAMLString(c.Governance))
	}
	if len(c.CodeRefs) > 0 {
		quotedRefs := make([]string, len(c.CodeRefs))
		for i, r := range c.CodeRefs {
			quotedRefs[i] = safeYAMLString(r)
		}
		fmt.Fprintf(&sb, "code_refs: [%s]\n", strings.Join(quotedRefs, ", "))
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
			keys := make([]string, 0, len(s.Extra))
			for key := range s.Extra {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				fmt.Fprintf(&sb, "    %s: %s\n", safeYAMLKey(key), safeYAMLString(s.Extra[key]))
			}
		}
	}

	if c.Attestation != nil {
		if c.Attestation.Runtime != "" {
			fmt.Fprintf(&sb, "runtime: %s\n", safeYAMLString(c.Attestation.Runtime))
		}
		if c.Attestation.Computation != "" {
			fmt.Fprintf(&sb, "computation: %s\n", safeYAMLString(c.Attestation.Computation))
		}
		if len(c.Attestation.Parameters) > 0 {
			sb.WriteString("parameters:\n")
			for _, p := range c.Attestation.Parameters {
				base := []string{
					fmt.Sprintf("name: %s", safeYAMLString(p.Name)),
				}
				if p.Type != "" {
					base = append(base, fmt.Sprintf("type: %s", safeYAMLString(p.Type)))
				}
				if p.Required {
					base = append(base, "required: true")
				}
				fmt.Fprintf(&sb, "  - %s\n", formatFlowPairs(base, p.Extra))
			}
		}
		if c.Attestation.Executor != nil {
			sb.WriteString("executor:\n")
			if c.Attestation.Executor.Resource != "" {
				fmt.Fprintf(&sb, "  resource: %s\n", safeYAMLString(c.Attestation.Executor.Resource))
			}
			if len(c.Attestation.Executor.Receipt) > 0 {
				quotedReceipt := make([]string, len(c.Attestation.Executor.Receipt))
				for i, r := range c.Attestation.Executor.Receipt {
					quotedReceipt[i] = safeYAMLString(r)
				}
				fmt.Fprintf(&sb, "  receipt: [%s]\n", strings.Join(quotedReceipt, ", "))
			}
			keys := make([]string, 0, len(c.Attestation.Executor.Extra))
			for key := range c.Attestation.Executor.Extra {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				fmt.Fprintf(&sb, "  %s: %s\n", safeYAMLKey(key), safeYAMLString(c.Attestation.Executor.Extra[key]))
			}
		}
		if c.Attestation.Attester != nil {
			sb.WriteString("attester:\n")
			if c.Attestation.Attester.Resource != "" {
				fmt.Fprintf(&sb, "  resource: %s\n", safeYAMLString(c.Attestation.Attester.Resource))
			}
			keys := make([]string, 0, len(c.Attestation.Attester.Extra))
			for key := range c.Attestation.Attester.Extra {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				fmt.Fprintf(&sb, "  %s: %s\n", safeYAMLKey(key), safeYAMLString(c.Attestation.Attester.Extra[key]))
			}
		}
	}

	// Preserve extra unknown fields in canonical key order. String values are
	// always JSON-quoted (valid YAML) so their scalar type and content survive a
	// round trip without allowing frontmatter delimiter injection.
	extraKeys := make([]string, 0, len(c.Extra))
	for k := range c.Extra {
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "verified", "governance":
			// Defense-in-depth: never allow smuggling reserved security keys via Extra.
			continue
		}
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
