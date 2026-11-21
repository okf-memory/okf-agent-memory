package okf

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	isoDateRegex     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	validStatuses    = map[string]bool{"draft": true, "stable": true, "deprecated": true}
	validGovernances = map[string]bool{
		GovernanceConstraint: true,
		GovernanceHold:       true,
		GovernanceContext:    true,
	}
)

// IsValidConceptStatus reports whether status is an OKF v0.2 lifecycle value.
func IsValidConceptStatus(status string) bool {
	return validStatuses[status]
}

// ValidationResult contains all validation diagnostics.
type ValidationResult struct {
	BundlePath   string           `json:"bundle_path"`
	DeclaredVer  string           `json:"declared_version,omitempty"`
	ConceptCount int              `json:"concept_count"`
	Errors       []string         `json:"errors"`
	Warnings     []string         `json:"warnings"`
	GateFindings []string         `json:"gate_findings"`
	BrokenLinks  []BrokenLink     `json:"broken_links"`
	Orphans      []string         `json:"orphans"`
	StaleCount   int              `json:"stale_count"`
	IsConformant bool             `json:"is_conformant"`
	GatePassed   bool             `json:"gate_passed"`
	Scorecard    *ScorecardResult `json:"scorecard,omitempty"`
}

// ValidateOptions controls validation severity and checks.
type ValidateOptions struct {
	Strict      bool
	Drift       bool
	DriftGit    bool
	Stale       bool
	StaleWithin time.Duration
	Scorecard   bool
	MinScore    int
}

func parseTimestamp(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
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
	isV2OrLater := b.DeclaredVer == "0.2" || !b.IsV02OrEarlier()

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
			res.Errors = append(res.Errors, fmt.Sprintf("%s: missing YAML frontmatter block", at))
			continue
		}

		for _, w := range c.ParseWarnings {
			res.Warnings = append(res.Warnings, fmt.Sprintf("%s: %s", at, w))
		}

		if strings.TrimSpace(c.Type) == "" {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: 'type' field is missing or empty", at))
		}
		if c.Title != "" && strings.TrimSpace(c.Title) == "" {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: 'title' field cannot be whitespace", at))
		}

		bodyWithoutFences := StripFences(c.Body)

		// v0.1 leftovers check on v0.2+ declared bundle
		if isV2OrLater {
			fm, _, hasFM := ExtractFrontmatter(c.RawContent)
			if hasFM && (strings.Contains(fm, "timestamp:") || (c.Extra != nil && c.Extra["timestamp"] != nil)) {
				res.GateFindings = append(res.GateFindings, fmt.Sprintf("%s: legacy 'timestamp' (v0.2 records it as 'generated: { by, at }')", at))
			}
			if regexp.MustCompile(`(?m)^#\s+Citations\s*$`).MatchString(bodyWithoutFences) {
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
				}
			}
			if s.LastModified != "" && !isoDateRegex.MatchString(s.LastModified) {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: sources[%d].last_modified '%s' is not YYYY-MM-DD", at, i, s.LastModified))
			}
		}

		// Keyed footnotes validation
		if len(sourceIDs) > 0 {
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
			}
			if c.Generated.At == "" {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: 'generated' has no 'at' timestamp", at))
			} else if _, ok := parseTimestamp(c.Generated.At); !ok {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: generated.at '%s' is not a valid ISO 8601 timestamp", at, c.Generated.At))
			}
		}

		// Trust validation: verified
		// Per OKF v0.3 §5.3, historical verifications (verified.at < generated.at)
		// remain in the file as audit history and are excluded from the active trust tier
		// without failing validation or emitting gate findings.
		for i, v := range c.Verified {
			if v.By == "" {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: verified[%d] has no 'by'", at, i))
			} else if !IsValidActor(v.By) {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: verified[%d].by '%s' is not a valid actor", at, i, v.By))
			}
			if v.At == "" {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: verified[%d] has no 'at' timestamp", at, i))
			} else if _, okV := parseTimestamp(v.At); !okV {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: verified[%d].at '%s' is not a valid ISO 8601 timestamp", at, i, v.At))
			}
		}

		// Governance validation
		if c.Governance != "" && !validGovernances[strings.ToLower(c.Governance)] {
			res.GateFindings = append(res.GateFindings, fmt.Sprintf("%s: governance '%s' is not constraint|hold|context", at, c.Governance))
		}

		// Protected concept provenance validation
		if c.IsProtected() && !HasHumanProvenance(c) {
			msg := fmt.Sprintf("%s: protected concept (%s: true) lacks human provenance (requires generated.by or current verified.by from a human actor)", at, ProtectedExtensionKey)
			res.Warnings = append(res.Warnings, msg)
			res.GateFindings = append(res.GateFindings, msg)
		}

		// CodeRefs traversal & boundary validation (CWE-22 prevention)
		for _, ref := range c.CodeRefs {
			refTrimmed := strings.TrimSpace(ref)
			if refTrimmed == "" {
				continue
			}
			normRef := strings.ReplaceAll(refTrimmed, "\\", "/")
			cleanRef := filepath.Clean(normRef)
			if IsAbsPath(cleanRef) {
				res.GateFindings = append(res.GateFindings, fmt.Sprintf("%s: code_refs '%s' must be a relative path", at, refTrimmed))
			} else if cleanRef == ".." || strings.HasPrefix(cleanRef, ".."+string(filepath.Separator)) || strings.HasPrefix(cleanRef, "../") {
				res.GateFindings = append(res.GateFindings, fmt.Sprintf("%s: code_refs '%s' contains forbidden '..' traversal", at, refTrimmed))
			}
		}

		// Lifecycle validation
		if c.Status != "" && !IsValidConceptStatus(c.Status) {
			msg := fmt.Sprintf("%s: status '%s' is not draft|stable|deprecated", at, c.Status)
			res.GateFindings = append(res.GateFindings, msg)
			res.Warnings = append(res.Warnings, msg)
		}
		if c.SupersededBy != "" {
			if !strings.EqualFold(c.Status, "deprecated") {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: 'superseded_by' only applies when status is 'deprecated'", at))
			}
			rp := b.ResolvePath(c.Path, c.SupersededBy)
			switch rp.Kind {
			case PathKindEscapes:
				res.Errors = append(res.Errors, fmt.Sprintf("%s: superseded_by %q escapes bundle root (path traversal denied)", at, c.SupersededBy))
			case PathKindExternal:
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: superseded_by %q is an external URL, not an in-bundle concept", at, c.SupersededBy))
			case PathKindConcept:
				if cycle := detectSupersededCycle(b, c.ID); len(cycle) > 0 {
					res.Errors = append(res.Errors, fmt.Sprintf("%s: cyclic superseded_by chain detected (%s)", at, strings.Join(cycle, " -> ")))
				}
			default:
				res.Errors = append(res.Errors, fmt.Sprintf("%s: superseded_by target %q does not exist as a concept", at, c.SupersededBy))
			}
		}

		// Unified path-traversal confinement checks for frontmatter path fields (OKF v0.3 §6.2, CWE-22)
		checkFrontmatterPath := func(fieldName, val string) {
			if strings.TrimSpace(val) == "" {
				return
			}
			rp := b.ResolvePath(c.Path, val)
			if rp.Kind == PathKindEscapes {
				res.Errors = append(res.Errors, fmt.Sprintf("%s: %s %q escapes bundle root (path traversal denied)", at, fieldName, val))
			}
		}
		checkFrontmatterPath("resource", c.Resource)
		if !b.IsV02OrEarlier() {
			for i, s := range c.Sources {
				if isPathLikeSourceResource(s.Resource) {
					checkFrontmatterPath(fmt.Sprintf("sources[%d].resource", i), s.Resource)
				}
			}
		}
		if c.Attestation != nil {
			checkFrontmatterPath("computation", c.Attestation.Computation)
			if c.Attestation.Executor != nil {
				checkFrontmatterPath("executor.resource", c.Attestation.Executor.Resource)
			}
			if c.Attestation.Attester != nil {
				checkFrontmatterPath("attester.resource", c.Attestation.Attester.Resource)
			}
		}
		if c.StaleAfter != "" {
			if !isoDateRegex.MatchString(c.StaleAfter) {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: stale_after '%s' is not YYYY-MM-DD", at, c.StaleAfter))
			} else if today >= c.StaleAfter {
				res.StaleCount++
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: concept is stale (stale_after %s <= %s)", at, c.StaleAfter, today))
			} else if opts.StaleWithin > 0 && c.IsStaleWithin(time.Now().UTC(), opts.StaleWithin) {
				res.StaleCount++
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: concept will become stale soon (stale_after %s within %v)", at, c.StaleAfter, opts.StaleWithin))
			}
		}
	}

	// Report CWE-22 path traversal in Markdown body/index links as conformance errors.
	for _, bl := range b.BrokenLinks {
		if bl.Reason == "path traversal denied: target escapes bundle root" {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: link %q escapes bundle root (path traversal denied)", bl.SourceConcept, bl.TargetHref))
		}
	}

	// 4. Drift check: compare index descriptions against concept descriptions & verify code_refs
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
				rp := b.ResolvePath(idxPath, href)
				if rp.Kind != PathKindConcept {
					continue
				}
				listingDesc := m[2]
				if concept, ok := b.Concepts[rp.ConceptID]; ok && concept.Description != "" {
					if !strings.Contains(normText(listingDesc), normText(concept.Description)) {
						res.Warnings = append(res.Warnings, fmt.Sprintf("%s: listing for %s.md differs from concept description", idxPath, rp.ConceptID))
					}
				}
			}
		}

		// Check that each concept is listed in its immediate parent index.md
		if len(b.Indexes) > 0 {
			indexTargets := make(map[string]map[string]bool, len(b.Indexes))
			for idxPath, idxContent := range b.Indexes {
				body := StripFences(idxContent)
				matches := linkRegex.FindAllStringSubmatch(body, -1)
				targets := make(map[string]bool, len(matches))
				for _, match := range matches {
					rp := b.ResolvePath(idxPath, match[1])
					if rp.Kind == PathKindConcept && rp.ConceptID != "" {
						targets[rp.ConceptID] = true
					}
				}
				indexTargets[idxPath] = targets
			}

			for _, c := range b.Concepts {
				normConceptPath := strings.ReplaceAll(c.Path, "\\", "/")
				dir := path.Dir(normConceptPath)
				indexRel := "index.md"
				if dir != "." {
					indexRel = path.Join(dir, "index.md")
				}
				targets, ok := indexTargets[indexRel]
				if !ok {
					res.Warnings = append(res.Warnings, fmt.Sprintf("%s: parent index %s does not exist", c.Path, indexRel))
					continue
				}

				if !targets[c.ID] {
					res.Warnings = append(res.Warnings, fmt.Sprintf("%s: concept is not listed in parent index %s", c.Path, indexRel))
				}
			}
		}

		// Check code_refs drift (verify non-glob references exist in repository or bundle)
		bundleAbs, _ := filepath.Abs(b.RootPath)
		projectRoot := filepath.Dir(bundleAbs)
		for _, c := range b.Concepts {
			at := c.Path
			for _, ref := range c.CodeRefs {
				refTrimmed := strings.TrimSpace(ref)
				if refTrimmed == "" || strings.ContainsAny(refTrimmed, "*?[{") {
					continue
				}
				cleanRef := filepath.Clean(refTrimmed)
				if cleanRef == ".." || strings.HasPrefix(cleanRef, ".."+string(filepath.Separator)) || IsAbsPath(cleanRef) {
					continue
				}
				pathInProj := filepath.Join(projectRoot, cleanRef)
				relProj, errRel := filepath.Rel(projectRoot, pathInProj)
				if errRel != nil || relProj == ".." || strings.HasPrefix(relProj, ".."+string(filepath.Separator)) {
					continue
				}
				pathInBundle := filepath.Join(bundleAbs, cleanRef)
				_, errProj := os.Stat(pathInProj)
				_, errBundle := os.Stat(pathInBundle)
				if errProj != nil && errBundle != nil {
					res.Warnings = append(res.Warnings, fmt.Sprintf("%s: code_refs '%s' points to non-existent path", at, refTrimmed))
				}
			}
		}
	}

	if opts.DriftGit {
		checkGitCodeDrift(b, res)
	}

	if opts.Scorecard || opts.MinScore > 0 {
		res.Scorecard = ComputeScorecard(b)
	}

	if opts.MinScore > 0 && res.Scorecard != nil && res.Scorecard.Score < opts.MinScore {
		res.GateFindings = append(res.GateFindings, fmt.Sprintf("quality score %d is below required minimum %d (Grade: %s)", res.Scorecard.Score, opts.MinScore, res.Scorecard.Grade))
	}

	res.IsConformant = len(res.Errors) == 0
	gateFailure := (opts.Strict && (len(b.BrokenLinks) > 0 || len(b.Orphans) > 0)) ||
		(opts.Strict && isV2OrLater && len(res.GateFindings) > 0) ||
		(opts.MinScore > 0 && res.Scorecard != nil && res.Scorecard.Score < opts.MinScore) ||
		((opts.Stale || opts.StaleWithin > 0) && res.StaleCount > 0)
	res.GatePassed = res.IsConformant && !gateFailure

	return res
}

// detectSupersededCycle walks superseded_by references starting at startID
// (regardless of status) and returns the cyclic chain if a cycle is reached.
func detectSupersededCycle(b *Bundle, startID string) []string {
	visited := map[string]bool{startID: true}
	chain := []string{startID}
	curr, ok := b.Concepts[startID]
	for ok && strings.TrimSpace(curr.SupersededBy) != "" {
		rp := b.ResolvePath(curr.Path, curr.SupersededBy)
		if rp.Kind != PathKindConcept {
			return nil
		}
		if visited[rp.ConceptID] {
			return append(chain, rp.ConceptID)
		}
		visited[rp.ConceptID] = true
		chain = append(chain, rp.ConceptID)
		curr, ok = b.Concepts[rp.ConceptID]
	}
	return nil
}
