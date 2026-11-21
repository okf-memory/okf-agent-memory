package okf

import (
	"math"
	"path"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// SearchResult represents a scored concept match.
type SearchResult struct {
	ConceptID      string         `json:"concept_id"`
	Title          string         `json:"title"`
	Type           string         `json:"type"`
	Description    string         `json:"description"`
	Governance     string         `json:"governance,omitempty"`
	CodeRefs       []string       `json:"code_refs,omitempty"`
	Score          float64        `json:"score"`
	MatchedOn      []string       `json:"matched_on"`
	Tags           []string       `json:"tags,omitempty"`
	Inbound        []string       `json:"inbound,omitempty"`
	Outbound       []string       `json:"outbound,omitempty"`
	Scope          Scope          `json:"scope,omitempty"`
	Priority       int            `json:"priority,omitempty"`
	Origin         string         `json:"origin,omitempty"`
	SupersededFrom string         `json:"superseded_from,omitempty"`
	TrustTier      string         `json:"trust_tier,omitempty"`
	Staleness      *StalenessInfo `json:"staleness,omitempty"`
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
// YAML frontmatter metadata filters, staleness horizons, and optional Git freshness.
type SearchOptions struct {
	Query            string
	TargetPath       string
	Limit            int
	Filter           string
	StaleWithin      time.Duration
	BaseTime         time.Time
	WithGitFreshness bool
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

		results = b.forwardSupersededResults(results, opts)

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
		b.enrichSearchResults(results, opts.WithGitFreshness)
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

		type parsedConceptTokens struct {
			id          string
			concept     *Concept
			titleTokens []string
			tagTokens   []string
			descTokens  []string
			idTokens    []string
			bodyTokens  []string
		}

		conceptTokenList := make([]parsedConceptTokens, 0, len(b.Concepts))
		for id, c := range b.Concepts {
			conceptTokenList = append(conceptTokenList, parsedConceptTokens{
				id:          id,
				concept:     c,
				titleTokens: tokenize(c.Title),
				tagTokens:   tokenize(strings.Join(c.Tags, " ")),
				descTokens:  tokenize(c.Description),
				idTokens:    tokenize(c.ID),
				bodyTokens:  tokenize(c.Body),
			})
		}

		tokenMatches := func(token, term string) bool {
			if utf8.RuneCountInString(term) < 4 {
				return token == term
			}
			return token == term || strings.HasPrefix(token, term)
		}

		countOccurrences := func(tokens []string, term string) float64 {
			cnt := 0.0
			for _, t := range tokens {
				if tokenMatches(t, term) {
					cnt++
				}
			}
			return cnt
		}

		// Compute Document Frequency for each query term using identical tokenization and matching logic
		df := make(map[string]float64, len(qTokens))
		for _, term := range qTokens {
			if _, exists := df[term]; exists {
				continue
			}
			count := 0.0
			for i := range conceptTokenList {
				tc := &conceptTokenList[i]
				if countOccurrences(tc.titleTokens, term) > 0 ||
					countOccurrences(tc.tagTokens, term) > 0 ||
					countOccurrences(tc.descTokens, term) > 0 ||
					countOccurrences(tc.idTokens, term) > 0 ||
					countOccurrences(tc.bodyTokens, term) > 0 {
					count++
				}
			}
			df[term] = count
		}

		var results []SearchResult

		for i := range conceptTokenList {
			tc := &conceptTokenList[i]
			c := tc.concept
			if !matchesCriteria(c) {
				continue
			}

			score := 0.0
			var matchedOn []string
			hasMatch := false

			for _, term := range qTokens {
				docFreq := df[term]
				if docFreq == 0 {
					continue
				}

				idf := math.Log(1.0 + (N-docFreq+0.5)/(docFreq+0.5))

				tfTitle := countOccurrences(tc.titleTokens, term)
				tfTags := countOccurrences(tc.tagTokens, term)
				tfDesc := countOccurrences(tc.descTokens, term)
				tfID := countOccurrences(tc.idTokens, term)
				tfBody := countOccurrences(tc.bodyTokens, term)

				termScore := 0.0
				if tfTitle > 0 {
					termScore += math.Min(tfTitle, 2.0) * 4.0
					matchedOn = append(matchedOn, "title")
				}
				if tfTags > 0 {
					termScore += math.Min(tfTags, 2.0) * 3.5
					matchedOn = append(matchedOn, "tags")
				}
				if tfDesc > 0 {
					termScore += math.Min(tfDesc, 3.0) * 2.5
					matchedOn = append(matchedOn, "description")
				}
				if tfID > 0 {
					termScore += math.Min(tfID, 2.0) * 2.0
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
					ConceptID:   tc.id,
					Title:       c.Title,
					Type:        c.Type,
					Description: c.Description,
					Governance:  c.EffectiveGovernance(),
					CodeRefs:    c.CodeRefs,
					Score:       math.Round(score*100) / 100,
					MatchedOn:   dedupMatched,
					Tags:        c.Tags,
					Inbound:     b.InboundGraph[tc.id],
					Outbound:    b.Graph[tc.id],
				})
			}
		}

		results = b.forwardSupersededResults(results, opts)

		sort.Slice(results, func(i, j int) bool {
			if results[i].Score == results[j].Score {
				return results[i].ConceptID < results[j].ConceptID
			}
			return results[i].Score > results[j].Score
		})

		if len(results) > limit {
			results = results[:limit]
		}
		b.enrichSearchResults(results, opts.WithGitFreshness)
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

	results = b.forwardSupersededResults(results, opts)

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

	b.enrichSearchResults(results, opts.WithGitFreshness)
	return results, nil
}

func (b *Bundle) enrichSearchResults(results []SearchResult, withGitFreshness bool) {
	if len(results) == 0 {
		return
	}
	var gitStaleness map[string]*StalenessInfo
	if withGitFreshness {
		gitStaleness = ResolveBundleStaleness(b, true)
	}
	for i := range results {
		c := b.Concepts[results[i].ConceptID]
		if c == nil {
			continue
		}
		results[i].TrustTier = TrustTier(c)
		if withGitFreshness {
			results[i].Staleness = gitStaleness[results[i].ConceptID]
		} else {
			results[i].Staleness = FrontmatterStaleness(c, b.RootPath)
		}
	}
}

func filterRequestsDeprecated(filterExpr string) bool {
	if strings.TrimSpace(filterExpr) == "" {
		return false
	}
	preds, err := parseFilterClauses(filterExpr)
	if err != nil {
		return false
	}
	for _, p := range preds {
		if p.key == "status" && p.operator == "=" && strings.EqualFold(p.value, "deprecated") {
			return true
		}
		if p.key == "superseded_by" {
			return true
		}
	}
	return false
}

// forwardSupersededResults follows superseded_by on deprecated concepts in search results
// so consumers querying current knowledge receive the replacement concept (OKF v0.3 §5.4).
// If the filter explicitly asks for deprecated concepts or superseded_by, results are left untouched.
func (b *Bundle) forwardSupersededResults(results []SearchResult, opts SearchOptions) []SearchResult {
	if len(results) == 0 || filterRequestsDeprecated(opts.Filter) {
		return results
	}

	byID := make(map[string]SearchResult, len(results))
	order := make([]string, 0, len(results))

	upsert := func(r SearchResult) {
		if existing, exists := byID[r.ConceptID]; exists {
			// Prefer a direct match over a forwarded one when scores tie, otherwise keep higher score.
			if r.Score > existing.Score || (r.Score == existing.Score && existing.SupersededFrom != "" && r.SupersededFrom == "") {
				byID[r.ConceptID] = r
			}
			return
		}
		byID[r.ConceptID] = r
		order = append(order, r.ConceptID)
	}

	for _, r := range results {
		c := b.Concepts[r.ConceptID]
		if c != nil && strings.EqualFold(c.Status, "deprecated") && strings.TrimSpace(c.SupersededBy) != "" {
			if sr, err := b.ResolveSuperseded(c.ID); err == nil && sr != nil && sr.Forwarded && sr.Target != nil {
				target := sr.Target
				forwarded := SearchResult{
					ConceptID:      target.ID,
					Title:          target.Title,
					Type:           target.Type,
					Description:    target.Description,
					Governance:     target.EffectiveGovernance(),
					CodeRefs:       target.CodeRefs,
					Score:          r.Score,
					MatchedOn:      r.MatchedOn,
					Tags:           target.Tags,
					Inbound:        b.InboundGraph[target.ID],
					Outbound:       b.Graph[target.ID],
					Scope:          r.Scope,
					Priority:       r.Priority,
					Origin:         r.Origin,
					SupersededFrom: c.ID,
				}
				upsert(forwarded)
				continue
			}
		}
		upsert(r)
	}

	out := make([]SearchResult, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out
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
