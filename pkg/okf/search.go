package okf

import (
	"math"
	"path"
	"path/filepath"
	"sort"
	"strings"
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

// Search queries the bundle using in-memory BM25/TF-IDF token scoring over frontmatter and body.
func (b *Bundle) Search(query string, limit int) []SearchResult {
	if len(query) > MaxQueryLength {
		runes := []rune(query)
		if len(runes) > MaxQueryLength {
			query = string(runes[:MaxQueryLength])
		}
	}

	qTokens := tokenize(query)
	if len(qTokens) == 0 {
		return nil
	}
	if len(qTokens) > 50 {
		qTokens = qTokens[:50]
	}

	if limit <= 0 {
		limit = 10
	} else if limit > MaxSearchLimit {
		limit = MaxSearchLimit
	}

	N := float64(len(b.Concepts))
	if N == 0 {
		return nil
	}

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

	// Compute scores for each concept
	var results []SearchResult

	for id, c := range b.Concepts {
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

			// Standard IDF formula with smoothing
			idf := math.Log(1.0 + (N-docFreq+0.5)/(docFreq+0.5))

			// Weighted term frequencies
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
			// Deduplicate matchedOn list
			dedupMap := make(map[string]bool)
			var dedupMatched []string
			for _, m := range matchedOn {
				if !dedupMap[m] {
					dedupMap[m] = true
					dedupMatched = append(dedupMatched, m)
				}
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

	return results
}

// matchCodeRef tests whether target matches a code reference pattern.
// Supports exact paths, directory prefixes, standard globs (path.Match), and recursive ** wildcards.
// It also seamlessly handles absolute paths (e.g. /workspace/pkg/okf/types.go).
func matchCodeRef(ref, target string) bool {
	ref = filepath.ToSlash(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(ref), "."), "/"))
	target = filepath.ToSlash(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(target), "."), "/"))
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
	if limit <= 0 {
		limit = 10
	} else if limit > MaxSearchLimit {
		limit = MaxSearchLimit
	}

	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		if query != "" {
			return b.Search(query, limit)
		}
		return nil
	}

	// First, identify all concepts matching targetPath via code_refs
	type candidate struct {
		concept    *Concept
		exactMatch bool
		matchedRef string
	}
	var matchedCandidates []candidate

	for _, c := range b.Concepts {
		for _, ref := range c.CodeRefs {
			if matchCodeRef(ref, targetPath) {
				exact := filepath.ToSlash(strings.TrimPrefix(ref, "./")) == filepath.ToSlash(strings.TrimPrefix(targetPath, "./"))
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
		return nil
	}

	// If query is also provided, compute text search scores
	textScores := make(map[string]SearchResult)
	if strings.TrimSpace(query) != "" {
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

	return results
}
