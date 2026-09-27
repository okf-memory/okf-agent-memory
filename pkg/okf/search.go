package okf

import (
	"math"
	"path"
	"sort"
	"strings"
	"time"
	"unicode"
)

// SearchResult represents a scored concept match.
type SearchResult struct {
	ConceptID   string   `json:"concept_id"`
	Title       string   `json:"title"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Governance  string   `json:"governance,omitempty"`
	CodeRefs    []string `json:"code_refs,omitempty"`
	Score       float64  `json:"score"`
	MatchedOn   []string `json:"matched_on"`
	Tags        []string `json:"tags,omitempty"`
	Inbound     []string `json:"inbound,omitempty"`
	Outbound    []string `json:"outbound,omitempty"`
}

func tokenize(s string) []string {
	f := func(c rune) bool {
		asciiDelimiter := (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9')
		if !asciiDelimiter {
			return false
		}
		if c <= unicode.MaxASCII {
			return true
		}
		return !unicode.IsLetter(c) && !unicode.IsDigit(c)
	}
	raw := strings.FieldsFunc(strings.ToLower(s), f)
	var out []string
	for _, w := range raw {
		if w != "" {
			out = append(out, w)
		}
	}
	return out
}

// MaxSearchLimit defines the maximum allowable search results returned to prevent resource exhaustion.
const MaxSearchLimit = 100

// MaxQueryLength defines the maximum query string length in runes/bytes evaluated to prevent resource exhaustion.
const MaxQueryLength = 1000

// SearchOptions controls search parameters including query text, path filtering,
// YAML frontmatter metadata filters, and staleness horizons.
type SearchOptions struct {
	Query       string
	TargetPath  string
	Limit       int
	Filter      string
	StaleWithin time.Duration
	BaseTime    time.Time
}

// Search queries the bundle using in-memory BM25/TF-IDF token scoring over frontmatter and body.
func (b *Bundle) Search(query string, limit int) []SearchResult {
	results, _ := b.SearchAdvanced(SearchOptions{
		Query: query,
		Limit: limit,
	})
	return results
}

// SearchAdvanced executes an advanced query combining full-text search, code path references,
// frontmatter filter predicates, and staleness horizon checks.
func (b *Bundle) SearchAdvanced(opts SearchOptions) ([]SearchResult, error) {
	if opts.Filter != "" {
		if _, err := parseFilterClauses(opts.Filter); err != nil {
			return nil, err
		}
	}

	baseTime := opts.BaseTime
	if baseTime.IsZero() {
		baseTime = time.Now().UTC()
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	} else if limit > MaxSearchLimit {
		limit = MaxSearchLimit
	}

	matchesCriteria := func(c *Concept) bool {
		if opts.Filter != "" {
			m, _ := c.MatchesFilter(opts.Filter)
			if !m {
				return false
			}
		}
		if opts.StaleWithin > 0 {
			if !c.IsStaleWithin(baseTime, opts.StaleWithin) {
				return false
			}
		}
		return true
	}

	targetPath := strings.TrimSpace(opts.TargetPath)
	query := strings.TrimSpace(opts.Query)

	// Sub-case A: Search by targetPath via code_refs
	if targetPath != "" {
		type candidate struct {
			concept    *Concept
			exactMatch bool
			matchedRef string
		}
		var matchedCandidates []candidate

		for _, c := range b.Concepts {
			if !matchesCriteria(c) {
				continue
			}
			for _, ref := range c.CodeRefs {
				if matchCodeRef(ref, targetPath) {
					exact := strings.ReplaceAll(strings.TrimPrefix(ref, "./"), "\\", "/") == strings.ReplaceAll(strings.TrimPrefix(targetPath, "./"), "\\", "/")
					matchedCandidates = append(matchedCandidates, candidate{
						concept:    c,
						exactMatch: exact,
						matchedRef: ref,
					})
					break
				}
			}
		}

		if len(matchedCandidates) == 0 {
			return []SearchResult{}, nil
		}

		textScores := make(map[string]SearchResult)
		if query != "" {
			allSearchResults := b.Search(query, len(b.Concepts))
			for _, r := range allSearchResults {
				textScores[r.ConceptID] = r
			}
		}

		var results []SearchResult
		for _, cand := range matchedCandidates {
			c := cand.concept
			gov := c.EffectiveGovernance()

			score := float64(governanceRank(gov) * 10)
			matchedOn := []string{"code_refs"}
			if opts.Filter != "" {
				matchedOn = append(matchedOn, "filter")
			}
			if opts.StaleWithin > 0 {
				matchedOn = append(matchedOn, "stale_within")
			}

			if cand.exactMatch {
				score += 2.0
			}

			if tr, exists := textScores[c.ID]; exists {
				score += tr.Score
				matchedOn = append(matchedOn, tr.MatchedOn...)
			}

			results = append(results, SearchResult{
				ConceptID:   c.ID,
				Title:       c.Title,
				Type:        c.Type,
				Description: c.Description,
				Governance:  gov,
				CodeRefs:    c.CodeRefs,
				Score:       math.Round(score*100) / 100,
				MatchedOn:   matchedOn,
				Tags:        c.Tags,
				Inbound:     b.InboundGraph[c.ID],
				Outbound:    b.Graph[c.ID],
			})
		}

		sort.Slice(results, func(i, j int) bool {
			rankI := governanceRank(results[i].Governance)
			rankJ := governanceRank(results[j].Governance)
			if rankI != rankJ {
				return rankI > rankJ
			}
			if results[i].Score != results[j].Score {
				return results[i].Score > results[j].Score
			}
			return results[i].ConceptID < results[j].ConceptID
		})

		if len(results) > limit {
			results = results[:limit]
		}
		return results, nil
	}

	// Sub-case B: Text query with BM25
	if query != "" {
		if len(query) > MaxQueryLength {
			runes := []rune(query)
			if len(runes) > MaxQueryLength {
				query = string(runes[:MaxQueryLength])
			}
		}

		qTokens := tokenize(query)
		if len(qTokens) == 0 {
			return []SearchResult{}, nil
		}
		if len(qTokens) > 50 {
			qTokens = qTokens[:50]
		}

		N := float64(len(b.Concepts))
		if N == 0 {
			return []SearchResult{}, nil
		}

		// TODO(search-scoring): Fix DF/TF asymmetry and acronym/short-term suppression:
		// 1. DF currently uses strings.Contains on concatenated concept content, causing short
		//    acronyms ("ci", "ui", "id") to match as substrings inside words ("specific", "require",
		//    "decision"), driving df -> N and collapsing IDF to near zero via the smoothing formula.
		//    DF calculation must use the same tokenization and matching logic as TF.
		// 2. TF prefix matching (HasPrefix) causes false positives for short stems ("log" -> "login",
		//    "auth" -> "author"). Require exact token matches for short tokens (< 4 chars) and allow
		//    prefix matching only for tokens >= 4 chars.
		// 3. Add field length normalization or per-field caps on TF (currently only tfBody is capped).
		// Compute Document Frequency for each query term
		df := make(map[string]float64)
		for _, t := range qTokens {
			count := 0.0
			for _, c := range b.Concepts {
				content := strings.ToLower(c.Title + " " + c.Description + " " + strings.Join(c.Tags, " ") + " " + c.ID + " " + c.Body)
				if strings.Contains(content, t) {
					count++
				}
			}
			df[t] = count
		}

		var results []SearchResult

		for id, c := range b.Concepts {
			if !matchesCriteria(c) {
				continue
			}

			score := 0.0
			var matchedOn []string
			titleTokens := tokenize(c.Title)
			tagTokens := tokenize(strings.Join(c.Tags, " "))
			descTokens := tokenize(c.Description)
			idTokens := tokenize(c.ID)
			bodyTokens := tokenize(c.Body)

			countOccurrences := func(tokens []string, term string) float64 {
				cnt := 0.0
				for _, t := range tokens {
					if t == term || strings.HasPrefix(t, term) {
						cnt++
					}
				}
				return cnt
			}

			hasMatch := false

			for _, term := range qTokens {
				docFreq := df[term]
				if docFreq == 0 {
					continue
				}

				idf := math.Log(1.0 + (N-docFreq+0.5)/(docFreq+0.5))

				tfTitle := countOccurrences(titleTokens, term)
				tfTags := countOccurrences(tagTokens, term)
				tfDesc := countOccurrences(descTokens, term)
				tfID := countOccurrences(idTokens, term)
				tfBody := countOccurrences(bodyTokens, term)

				termScore := 0.0
				if tfTitle > 0 {
					termScore += tfTitle * 4.0
					matchedOn = append(matchedOn, "title")
				}
				if tfTags > 0 {
					termScore += tfTags * 3.5
					matchedOn = append(matchedOn, "tags")
				}
				if tfDesc > 0 {
					termScore += tfDesc * 2.5
					matchedOn = append(matchedOn, "description")
				}
				if tfID > 0 {
					termScore += tfID * 2.0
					matchedOn = append(matchedOn, "id")
				}
				if tfBody > 0 {
					termScore += math.Min(tfBody, 5.0) * 1.0
					matchedOn = append(matchedOn, "body")
				}

				if termScore > 0 {
					hasMatch = true
					score += termScore * idf
				}
			}

			if hasMatch && score > 0 {
				dedupMap := make(map[string]bool)
				var dedupMatched []string
				for _, m := range matchedOn {
					if !dedupMap[m] {
						dedupMap[m] = true
						dedupMatched = append(dedupMatched, m)
					}
				}
				if opts.Filter != "" {
					dedupMatched = append(dedupMatched, "filter")
				}
				if opts.StaleWithin > 0 {
					dedupMatched = append(dedupMatched, "stale_within")
				}

				results = append(results, SearchResult{
					ConceptID:   id,
					Title:       c.Title,
					Type:        c.Type,
					Description: c.Description,
					Governance:  c.EffectiveGovernance(),
					CodeRefs:    c.CodeRefs,
					Score:       math.Round(score*100) / 100,
					MatchedOn:   dedupMatched,
					Tags:        c.Tags,
					Inbound:     b.InboundGraph[id],
					Outbound:    b.Graph[id],
				})
			}
		}

		sort.Slice(results, func(i, j int) bool {
			if results[i].Score == results[j].Score {
				return results[i].ConceptID < results[j].ConceptID
			}
			return results[i].Score > results[j].Score
		})

		if len(results) > limit {
			results = results[:limit]
		}
		return results, nil
	}

	// Sub-case C: Pure Filter / Staleness search (query == "" && targetPath == "")
	var results []SearchResult
	for id, c := range b.Concepts {
		if !matchesCriteria(c) {
			continue
		}
		gov := c.EffectiveGovernance()
		score := float64(governanceRank(gov) * 10)
		var matchedOn []string
		if opts.Filter != "" {
			matchedOn = append(matchedOn, "filter")
		}
		if opts.StaleWithin > 0 {
			matchedOn = append(matchedOn, "stale_within")
		}

		results = append(results, SearchResult{
			ConceptID:   id,
			Title:       c.Title,
			Type:        c.Type,
			Description: c.Description,
			Governance:  gov,
			CodeRefs:    c.CodeRefs,
			Score:       score,
			MatchedOn:   matchedOn,
			Tags:        c.Tags,
			Inbound:     b.InboundGraph[id],
			Outbound:    b.Graph[id],
		})
	}

	sort.Slice(results, func(i, j int) bool {
		rankI := governanceRank(results[i].Governance)
		rankJ := governanceRank(results[j].Governance)
		if rankI != rankJ {
			return rankI > rankJ
		}
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].ConceptID < results[j].ConceptID
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// matchCodeRef tests whether target matches a code reference pattern.
// Supports exact paths, directory prefixes, standard globs (path.Match), and recursive ** wildcards.
// It also seamlessly handles absolute paths (e.g. /workspace/pkg/okf/types.go).
func matchCodeRef(ref, target string) bool {
	ref = strings.ReplaceAll(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(ref), "."), "/"), "\\", "/")
	target = strings.ReplaceAll(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(target), "."), "/"), "\\", "/")
	if ref == "" || target == "" {
		return false
	}
	if ref == target || strings.HasSuffix(target, "/"+ref) {
		return true
	}
	// Directory prefix: e.g. "pkg/okf" matches "pkg/okf/types.go" or "/app/pkg/okf/types.go"
	cleanRef := strings.TrimSuffix(ref, "/")
	if strings.HasPrefix(target, cleanRef+"/") || strings.Contains(target, "/"+cleanRef+"/") {
		return true
	}
	// Standard path.Match glob
	if matched, err := path.Match(ref, target); err == nil && matched {
		return true
	}
	// Recursive glob: e.g. "pkg/**/*.go" or "**/*.go"
	if strings.Contains(ref, "**") {
		parts := strings.Split(ref, "**")
		if len(parts) == 2 {
			prefix := strings.TrimSuffix(parts[0], "/")
			suffix := strings.TrimPrefix(parts[1], "/")
			hasPrefixMatch := prefix == "" || strings.HasPrefix(target, prefix+"/") || target == prefix || strings.Contains(target, "/"+prefix+"/")
			if !hasPrefixMatch {
				return false
			}
			if suffix == "" {
				return true
			}
			if strings.Contains(suffix, "*") {
				matched, err := path.Match(suffix, path.Base(target))
				return err == nil && matched
			}
			return strings.HasSuffix(target, "/"+suffix) || target == suffix
		}
	}
	return false
}

func governanceRank(gov string) int {
	switch strings.ToLower(gov) {
	case GovernanceHold:
		return 3
	case GovernanceConstraint:
		return 2
	default:
		return 1
	}
}

// SearchForPath finds concepts governing targetPath via code_refs.
// Matches are ranked first by governance authority (hold > constraint > context),
// then by query relevance score (or match specificity if query is empty), and finally by ConceptID.
func (b *Bundle) SearchForPath(targetPath, query string, limit int) []SearchResult {
	results, _ := b.SearchAdvanced(SearchOptions{
		TargetPath: targetPath,
		Query:      query,
		Limit:      limit,
	})
	return results
}
