package okf

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	isoDateRegex  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	validStatuses = map[string]bool{"draft": true, "stable": true, "deprecated": true}
)

// ValidationResult contains all validation diagnostics.
type ValidationResult struct {
	BundlePath   string       `json:"bundle_path"`
	DeclaredVer  string       `json:"declared_version,omitempty"`
	ConceptCount int          `json:"concept_count"`
	Errors       []string     `json:"errors"`
	Warnings     []string     `json:"warnings"`
	GateFindings []string     `json:"gate_findings"`
	BrokenLinks  []BrokenLink `json:"broken_links"`
	Orphans      []string     `json:"orphans"`
	StaleCount   int          `json:"stale_count"`
	IsConformant bool         `json:"is_conformant"`
	GatePassed   bool         `json:"gate_passed"`
}

// ValidateOptions controls validation severity and checks.
type ValidateOptions struct {
	Strict bool
	Drift  bool
}

// Validate performs full OKF v0.2 conformance, connectivity, and drift validation on a loaded bundle.
func Validate(b *Bundle, opts ValidateOptions) *ValidationResult {
	res := &ValidationResult{
		BundlePath:   b.RootPath,
		DeclaredVer:  b.DeclaredVer,
		ConceptCount: len(b.Concepts),
		Errors:       make([]string, 0),
		Warnings:     make([]string, 0),
		GateFindings: make([]string, 0),
		BrokenLinks:  b.BrokenLinks,
		Orphans:      b.Orphans,
	}

	today := time.Now().UTC().Format("2006-01-02")
	isV2 := b.DeclaredVer == "0.2"

	// 1. Validate sub-indexes have no frontmatter
	for relPath, content := range b.Indexes {
		if relPath == "index.md" {
			continue
		}
		_, _, hasFM := ExtractFrontmatter(content)
		if hasFM {
			res.Warnings = append(res.Warnings, fmt.Sprintf("%s: index.md should carry no frontmatter (only bundle-root may)", relPath))
		}
	}

	// 2. Validate log.md date headings
	if b.LogContent != "" {
		for _, line := range strings.Split(b.LogContent, "\n") {
			if after, ok := strings.CutPrefix(line, "## "); ok {
				dateStr := strings.TrimSpace(after)
				if !isoDateRegex.MatchString(dateStr) {
					res.Warnings = append(res.Warnings, fmt.Sprintf("log.md: log heading '%s' is not ISO 8601 YYYY-MM-DD", dateStr))
				}
			}
		}
	}

	// 3. Validate each concept
	for _, c := range b.Concepts {
		at := c.Path

		if c.RawContent != "" && c.Type == "" && c.Body == "" {
			// Failed parsing frontmatter entirely
			if filepath.Base(c.Path) == "README.md" {
				res.Errors = append(res.Errors, fmt.Sprintf("%s: README.md inside a bundle is a concept to a consumer, and has no frontmatter", at))
			} else {
				res.Errors = append(res.Errors, fmt.Sprintf("%s: missing YAML frontmatter block", at))
			}
			continue
		}

		if c.Type == "" {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: 'type' field is missing or empty", at))
		}

		// v0.1 leftovers check on v0.2 declared bundle
		if isV2 {
			if strings.Contains(c.RawContent, "timestamp:") {
				res.GateFindings = append(res.GateFindings, fmt.Sprintf("%s: legacy 'timestamp' (v0.2 records it as 'generated: { by, at }')", at))
			}
			if regexp.MustCompile(`(?m)^#\s+Citations\s*$`).MatchString(c.RawContent) {
				res.GateFindings = append(res.GateFindings, fmt.Sprintf("%s: legacy '# Citations' body section (v0.2 uses 'sources' frontmatter)", at))
			}
		}

		// Sources validation
		sourceIDs := make(map[string]bool)
		for i, s := range c.Sources {
			if s.Resource == "" {
				res.GateFindings = append(res.GateFindings, fmt.Sprintf("%s: sources[%d] has no 'resource'", at, i))
			}
			if s.ID != "" {
				sourceIDs[s.ID] = true
			}
			if s.Author != "" {
				if !IsValidActor(s.Author) {
					res.Warnings = append(res.Warnings, fmt.Sprintf("%s: sources[%d].author '%s' is not a valid actor", at, i, s.Author))
				} else if p := GetNonStandardPrefix(s.Author); p != "" {
					res.Warnings = append(res.Warnings, fmt.Sprintf("%s: sources[%d].author '%s' uses non-standard prefix '%s:'", at, i, s.Author, p))
				}
			}
			if s.LastModified != "" && !isoDateRegex.MatchString(s.LastModified) {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: sources[%d].last_modified '%s' is not YYYY-MM-DD", at, i, s.LastModified))
			}
		}

		// Keyed footnotes validation
		if len(sourceIDs) > 0 {
			bodyWithoutFences := StripFences(c.Body)
			fnMatches := regexp.MustCompile(`\[\^([^\]]+)\]`).FindAllStringSubmatch(bodyWithoutFences, -1)
			for _, m := range fnMatches {
				fnKey := m[1]
				if !sourceIDs[fnKey] {
					res.Warnings = append(res.Warnings, fmt.Sprintf("%s: footnote [^%s] matches no 'sources' entry id", at, fnKey))
				}
			}
		}

		// Trust validation: generated
		if c.Generated != nil {
			if c.Generated.By == "" {
				res.GateFindings = append(res.GateFindings, fmt.Sprintf("%s: 'generated' has no 'by' field", at))
			} else if !IsValidActor(c.Generated.By) {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: generated.by '%s' is not a valid actor", at, c.Generated.By))
			} else if p := GetNonStandardPrefix(c.Generated.By); p != "" {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: generated.by '%s' uses non-standard prefix '%s:'", at, c.Generated.By, p))
			}
			if c.Generated.At == "" {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: 'generated' has no 'at' timestamp", at))
			}
		}

		// Trust validation: verified
		for i, v := range c.Verified {
			if v.By == "" {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: verified[%d] has no 'by'", at, i))
			} else if !IsValidActor(v.By) {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: verified[%d].by '%s' is not a valid actor", at, i, v.By))
			} else if p := GetNonStandardPrefix(v.By); p != "" {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: verified[%d].by '%s' uses non-standard prefix '%s:'", at, i, v.By, p))
			}
			if v.At == "" {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: verified[%d] has no 'at' timestamp", at, i))
			}
		}

		// Lifecycle validation
		if c.Status != "" && !validStatuses[c.Status] {
			res.GateFindings = append(res.GateFindings, fmt.Sprintf("%s: status '%s' is not draft|stable|deprecated", at, c.Status))
		}
		if c.StaleAfter != "" {
			if !isoDateRegex.MatchString(c.StaleAfter) {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: stale_after '%s' is not YYYY-MM-DD", at, c.StaleAfter))
			} else if today >= c.StaleAfter {
				res.StaleCount++
			}
		}
	}

	// 4. Drift check: compare index descriptions against concept descriptions
	if opts.Drift {
		normText := func(s string) string {
			s = strings.ToLower(s)
			s = strings.ReplaceAll(s, "\\", "")
			s = strings.ReplaceAll(s, "\"", "")
			s = strings.ReplaceAll(s, "'", "")
			s = strings.ReplaceAll(s, "`", "")
			s = strings.ReplaceAll(s, "*", "")
			s = strings.ReplaceAll(s, "_", "")
			return strings.TrimSpace(strings.TrimSuffix(strings.Join(strings.Fields(s), " "), "."))
		}

		entryRegex := regexp.MustCompile(`(?m)^[*-]\s+\[[^\]]+\]\(([^)\s]+\.md)\)\s*[-–:]\s+(.+)$`)
		for idxPath, idxContent := range b.Indexes {
			matches := entryRegex.FindAllStringSubmatch(StripFences(idxContent), -1)
			for _, m := range matches {
				href := m[1]
				listingDesc := m[2]
				targetID := b.ResolveLink(idxPath, href)
				if concept, ok := b.Concepts[targetID]; ok && concept.Description != "" {
					if !strings.Contains(normText(listingDesc), normText(concept.Description)) {
						res.Warnings = append(res.Warnings, fmt.Sprintf("%s: listing for %s.md differs from concept description", idxPath, targetID))
					}
				}
			}
		}
	}

	res.IsConformant = len(res.Errors) == 0
	gateFailure := (opts.Strict && (len(b.BrokenLinks) > 0 || len(b.Orphans) > 0)) ||
		(opts.Strict && isV2 && len(res.GateFindings) > 0)
	res.GatePassed = res.IsConformant && !gateFailure

	return res
}
