package okf

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ChangeOperation represents one recorded mutation on a concept.
type ChangeOperation struct {
	Op     string `json:"op"`
	At     string `json:"at"`
	Note   string `json:"note,omitempty"`
	Target string `json:"target,omitempty"`
}

// ConceptChange summarizes all changes to a single concept within a queried time range.
type ConceptChange struct {
	ConceptID   string            `json:"concept_id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Status      string            `json:"status,omitempty"`
	Deleted     bool              `json:"deleted,omitempty"`
	GeneratedAt string            `json:"generated_at,omitempty"`
	GeneratedBy string            `json:"generated_by,omitempty"`
	Operations  []ChangeOperation `json:"operations"`
}

// ChangesResult is the structured response for QueryChanges (CLI --json and MCP okf_changes).
type ChangesResult struct {
	Since   string          `json:"since"`
	Until   string          `json:"until"`
	Total   int             `json:"total"`
	Changes []ConceptChange `json:"changes"`
}

// ChangesOptions configures a time-range query over bundle concept changes.
type ChangesOptions struct {
	Since string
	Until string
	Limit int
	Now   time.Time
}

const (
	defaultChangesLimit = 50
	maxChangesLimit     = 500
)

var (
	// Matches: * **EntryType** [2026-11-04T12:20:00Z]: description
	// or:      * **EntryType**: description
	logBulletRegex   = regexp.MustCompile(`^[*-]\s+\*\*([^*]+)\*\*(?:\s+\[([^\]]+)\])?\s*:\s*(.+)$`)
	backtickMDRegex  = regexp.MustCompile("`([^`]+\\.md)`")
	docTitleRegex    = regexp.MustCompile("`[^`]+\\.md`\\s*\\(([^)]+)\\)")
	relateDescPrefix = regexp.MustCompile(`(?i)^linked\s+` + "`([^`]+\\.md)`" + `\s+to\s+` + "`([^`]+\\.md)`")
)

// ParseTimeBound parses a time boundary for change queries.
// Accepts YYYY-MM-DD, ISO 8601 timestamps, or relative durations (e.g. 1d, 2w, 3m, 48h).
// For date-only YYYY-MM-DD inputs, isUntil=false resolves to 00:00:00Z and isUntil=true
// resolves to 23:59:59.999999999Z so date ranges are inclusive.
func ParseTimeBound(raw string, now time.Time, isUntil bool) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("empty time boundary")
	}

	nowUTC := now.UTC()

	// 1. Date-only YYYY-MM-DD
	if t, err := time.Parse("2006-01-02", trimmed); err == nil {
		if isUntil {
			return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC), nil
		}
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
	}

	// 2. Full ISO 8601 timestamp
	if t, ok := parseTimestamp(trimmed); ok {
		return t.UTC(), nil
	}

	// 3. Relative duration (e.g. 1d, 2w, 3m, 48h)
	if d, err := ParseRelativeDuration(trimmed); err == nil {
		return nowUTC.Add(-d), nil
	}

	return time.Time{}, fmt.Errorf("invalid time value %q: expected YYYY-MM-DD, ISO 8601 timestamp, or relative duration (e.g. 1d, 2w, 3m)", raw)
}

type parsedLogOp struct {
	conceptID string
	op        ChangeOperation
	sortTime  time.Time
	seq       int
}

// QueryChanges returns concepts created, updated, related, deprecated, or deleted
// within [since, until], combining log.md history with bundle concept metadata.
func QueryChanges(b *Bundle, opts ChangesOptions) (*ChangesResult, error) {
	if b == nil {
		return nil, fmt.Errorf("bundle is nil")
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	if strings.TrimSpace(opts.Since) == "" {
		return nil, fmt.Errorf("'since' is required")
	}
	sinceTime, err := ParseTimeBound(opts.Since, now, false)
	if err != nil {
		return nil, fmt.Errorf("invalid since: %w", err)
	}

	untilTime := now
	if strings.TrimSpace(opts.Until) != "" {
		untilTime, err = ParseTimeBound(opts.Until, now, true)
		if err != nil {
			return nil, fmt.Errorf("invalid until: %w", err)
		}
	}

	if sinceTime.After(untilTime) {
		return nil, fmt.Errorf("since (%s) cannot be after until (%s)", sinceTime.Format(time.RFC3339), untilTime.Format(time.RFC3339))
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = defaultChangesLimit
	}
	if limit > maxChangesLimit {
		limit = maxChangesLimit
	}

	// 1. Parse log.md entries across the full log (capturing last known titles for deleted concepts)
	lastKnownTitle := make(map[string]string)
	byConcept := make(map[string][]parsedLogOp)

	parseLogForChanges(b.LogContent, sinceTime, untilTime, lastKnownTitle, byConcept)

	// 2. Include concepts from the bundle whose generated.at falls within [since, until]
	// even if they have no log.md entry in that range (e.g. --no-log or out-of-band edit).
	for id, c := range b.Concepts {
		if _, hasLog := byConcept[id]; hasLog {
			continue
		}
		if c.Generated == nil || strings.TrimSpace(c.Generated.At) == "" {
			continue
		}
		genT, ok := parseTimestamp(c.Generated.At)
		if !ok {
			continue
		}
		genUTC := genT.UTC()
		if genUTC.Before(sinceTime) || genUTC.After(untilTime) {
			continue
		}
		opName := "update"
		if strings.EqualFold(c.Status, "deprecated") {
			opName = "deprecate"
		}
		byConcept[id] = append(byConcept[id], parsedLogOp{
			conceptID: id,
			op: ChangeOperation{
				Op: opName,
				At: genUTC.Format(time.RFC3339),
			},
			sortTime: genUTC,
		})
	}

	// 3. Assemble one ConceptChange per concept, enriched with bundle metadata
	type rankedChange struct {
		change     ConceptChange
		latestTime time.Time
	}

	ranked := make([]rankedChange, 0, len(byConcept))
	for id, ops := range byConcept {
		// Sort operations chronologically (oldest -> newest); log.md is newest-first so higher seq is older
		sort.SliceStable(ops, func(i, j int) bool {
			if !ops[i].sortTime.Equal(ops[j].sortTime) {
				return ops[i].sortTime.Before(ops[j].sortTime)
			}
			return ops[i].seq > ops[j].seq
		})

		outOps := make([]ChangeOperation, len(ops))
		var latest time.Time
		for i, o := range ops {
			outOps[i] = o.op
			if o.sortTime.After(latest) {
				latest = o.sortTime
			}
		}

		cc := ConceptChange{
			ConceptID:  id,
			Operations: outOps,
		}

		if c, ok := b.Concepts[id]; ok {
			cc.Title = c.Title
			cc.Description = c.Description
			cc.Status = c.Status
			if c.Generated != nil {
				cc.GeneratedAt = c.Generated.At
				cc.GeneratedBy = c.Generated.By
			}
		} else {
			cc.Deleted = true
			if title, ok := lastKnownTitle[id]; ok {
				cc.Title = title
			} else {
				cc.Title = path.Base(id)
			}
		}

		ranked = append(ranked, rankedChange{
			change:     cc,
			latestTime: latest,
		})
	}

	// Sort concepts newest-first by latest operation time, tie-breaking by concept_id ascending
	sort.Slice(ranked, func(i, j int) bool {
		if !ranked[i].latestTime.Equal(ranked[j].latestTime) {
			return ranked[i].latestTime.After(ranked[j].latestTime)
		}
		return ranked[i].change.ConceptID < ranked[j].change.ConceptID
	})

	total := len(ranked)
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}

	changes := make([]ConceptChange, len(ranked))
	for i, r := range ranked {
		changes[i] = r.change
	}

	return &ChangesResult{
		Since:   sinceTime.Format(time.RFC3339),
		Until:   untilTime.Format(time.RFC3339),
		Total:   total,
		Changes: changes,
	}, nil
}

func parseLogForChanges(logContent string, sinceTime, untilTime time.Time, lastKnownTitle map[string]string, byConcept map[string][]parsedLogOp) {
	if strings.TrimSpace(logContent) == "" {
		return
	}

	var currentDay time.Time
	var currentDayStr string
	hasDay := false
	seq := 0

	for _, rawLine := range strings.Split(logContent, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		if after, ok := strings.CutPrefix(line, "## "); ok {
			dateStr := strings.TrimSpace(after)
			if t, err := time.Parse("2006-01-02", dateStr); err == nil {
				currentDay = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
				currentDayStr = dateStr
				hasDay = true
			} else {
				hasDay = false
				currentDayStr = ""
			}
			continue
		}

		if !hasDay {
			continue
		}

		m := logBulletRegex.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		seq++

		entryType := strings.TrimSpace(m[1])
		inlineTS := strings.TrimSpace(m[2])
		body := strings.TrimSpace(m[3])

		// Extract optional change note separated by ". — " or " — "
		descPart, note := splitChangeNote(body)

		// Capture last known title for any concept mentioned with "(Title)" (e.g. Documented concept `id.md` (Title))
		// Note: log.md is newest-first, so the first title seen is the most recent.
		if titleMatch := docTitleRegex.FindStringSubmatch(descPart); titleMatch != nil {
			if mdMatches := backtickMDRegex.FindStringSubmatch(descPart); mdMatches != nil {
				cid := normalizeLoggedConceptID(mdMatches[1])
				candidate := strings.TrimSpace(titleMatch[1])
				if cid != "" && candidate != "" && !strings.HasPrefix(candidate, "set stale_after ") && candidate != "cleared stale_after" {
					if _, exists := lastKnownTitle[cid]; !exists {
						lastKnownTitle[cid] = candidate
					}
				}
			}
		}

		// Determine timestamp & range match
		var atStr string
		var sortTime time.Time
		inRange := false

		if inlineTS != "" {
			if t, ok := parseTimestamp(inlineTS); ok {
				tUTC := t.UTC()
				atStr = tUTC.Format(time.RFC3339)
				sortTime = tUTC
				inRange = !tUTC.Before(sinceTime) && !tUTC.After(untilTime)
			}
		}
		if atStr == "" {
			// Legacy date-only entry: matches if the UTC day overlaps [sinceTime, untilTime]
			dayStart := currentDay
			dayEnd := time.Date(currentDay.Year(), currentDay.Month(), currentDay.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
			inRange = !dayEnd.Before(sinceTime) && !dayStart.After(untilTime)
			atStr = currentDayStr
			sortTime = dayStart
		}

		if !inRange {
			continue
		}

		opName := canonicalLogOp(entryType, descPart)

		// Handle "Linked `src.md` to `tgt.md`" (relate)
		if relMatch := relateDescPrefix.FindStringSubmatch(descPart); relMatch != nil {
			srcID := normalizeLoggedConceptID(relMatch[1])
			tgtID := normalizeLoggedConceptID(relMatch[2])
			if srcID != "" {
				byConcept[srcID] = append(byConcept[srcID], parsedLogOp{
					conceptID: srcID,
					op: ChangeOperation{
						Op:     "relate",
						At:     atStr,
						Note:   note,
						Target: tgtID,
					},
					sortTime: sortTime,
					seq:      seq,
				})
			}
			continue
		}

		// General entry: extract all backtick-quoted .md concept references
		mdMatches := backtickMDRegex.FindAllStringSubmatch(descPart, -1)
		if len(mdMatches) == 0 {
			continue
		}

		seenInLine := make(map[string]bool)
		for _, mm := range mdMatches {
			cid := normalizeLoggedConceptID(mm[1])
			if cid == "" || seenInLine[cid] {
				continue
			}
			seenInLine[cid] = true
			byConcept[cid] = append(byConcept[cid], parsedLogOp{
				conceptID: cid,
				op: ChangeOperation{
					Op:   opName,
					At:   atStr,
					Note: note,
				},
				sortTime: sortTime,
				seq:      seq,
			})
		}
	}
}

func splitChangeNote(body string) (string, string) {
	if idx := strings.Index(body, ". — "); idx != -1 {
		return strings.TrimSpace(body[:idx+1]), strings.TrimSpace(body[idx+len(". — "):])
	}
	if idx := strings.Index(body, " — "); idx != -1 {
		return strings.TrimSpace(body[:idx]), strings.TrimSpace(body[idx+len(" — "):])
	}
	return body, ""
}

func normalizeLoggedConceptID(rawPath string) string {
	clean := strings.TrimSpace(strings.ReplaceAll(rawPath, "\\", "/"))
	clean = strings.TrimPrefix(clean, "./")
	clean = strings.TrimSuffix(clean, ".md")
	if clean == "" || strings.EqualFold(path.Base(clean), "index") ||
		strings.EqualFold(clean, "log") || strings.EqualFold(clean, "AGENTS") {
		return ""
	}
	return clean
}

func canonicalLogOp(entryType, desc string) string {
	lower := strings.ToLower(strings.TrimSpace(entryType))
	switch lower {
	case "creation", "created", "create", "add", "added":
		return "create"
	case "relate", "related", "link", "linked":
		return "relate"
	case "deletion", "delete", "deleted", "remove", "removed":
		return "delete"
	case "deprecation", "deprecate", "deprecated":
		return "deprecate"
	case "update", "updated", "edit", "edited", "modify", "modified":
		if relateDescPrefix.MatchString(desc) {
			return "relate"
		}
		if strings.Contains(strings.ToLower(desc), "deprecated concept ") {
			return "deprecate"
		}
		return "update"
	default:
		if lower == "" {
			return "update"
		}
		return lower
	}
}
