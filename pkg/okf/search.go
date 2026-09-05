package okf

import (
	"math"
	"sort"
	"strings"
)

// SearchResult represents a scored concept match.
type SearchResult struct {
	ConceptID   string   `json:"concept_id"`
	Title       string   `json:"title"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Score       float64  `json:"score"`
	MatchedOn   []string `json:"matched_on"`
	Tags        []string `json:"tags,omitempty"`
	Inbound     []string `json:"inbound,omitempty"`
	Outbound    []string `json:"outbound,omitempty"`
}

func tokenize(s string) []string {
	f := func(c rune) bool {
		return (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9')
	}
	raw := strings.FieldsFunc(strings.ToLower(s), f)
	var out []string
	for _, w := range raw {
		if len(w) > 1 {
			out = append(out, w)
		}
	}
	return out
}

// Search queries the bundle using in-memory BM25/TF-IDF token scoring over frontmatter and body.
func (b *Bundle) Search(query string, limit int) []SearchResult {
	qTokens := tokenize(query)
	if len(qTokens) == 0 {
		return nil
	}

	if limit <= 0 {
		limit = 10
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
				Score:       math.Round(score*100) / 100,
				MatchedOn:   dedupMatched,
				Tags:        c.Tags,
				Inbound:     b.InboundGraph[id],
				Outbound:    b.Graph[id],
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results
}
