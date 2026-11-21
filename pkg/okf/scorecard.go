package okf

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// ScorecardResult represents an OpenSSF-style quality evaluation of an OKF bundle across 3 pillars.
type ScorecardResult struct {
	Score       int      `json:"score"`       // Overall score 0-100
	Grade       string   `json:"grade"`       // Grade "A", "B", "C", "D", "F"
	Maintenance int      `json:"maintenance"` // Epistemic Maintenance & Relevance (0-100, weight: 35%)
	Stewardship int      `json:"stewardship"` // Human Stewardship & Provenance (0-100, weight: 35%)
	Integrity   int      `json:"integrity"`   // Graph & Conformance Integrity (0-100, weight: 30%)
	Signals     []string `json:"signals"`     // Key diagnostics and rationale signals
	Summary     string   `json:"summary"`     // One-line human-readable summary
}

// ComputeScorecard evaluates the quality scorecard for a loaded OKF bundle.
//
// Scoring Dimensions:
// 1. Epistemic Maintenance (35%):
//   - Invariant/durable concepts without stale_after are treated as 100% valid by default (anti-chore).
//   - Lapsed stale_after dates incur a hard deduction (0 points for that concept).
//   - Deprecated concepts receive 0 points; draft concepts receive 70%.
//   - If the root bundle index is marked deprecated, maintenance is capped at 0.
//
// 2. Human Stewardship & Provenance (35%):
//   - Only current human verifications (verified.at >= generated.at, or all if generated.at is absent) count toward human stewardship.
//   - Concepts with a current human review (TrustTier == "human-reviewed") or human authors receive 100 points.
//   - Unreviewed synthetic agent concepts (generated.by: agent/*) or machine-confirmed concepts receive 20 points.
//   - Unattributed concepts receive 0 points.
//
// 3. Graph & Conformance Integrity (30%):
//   - Strict OKF v0.2 validation starting at 100 base points.
//   - Errors: -25 pts each, broken links: -15 pts each, orphans: -10 pts each, warnings: -5 pts each.
func ComputeScorecard(b *Bundle) *ScorecardResult {
	if b == nil || len(b.Concepts) == 0 {
		return &ScorecardResult{
			Score:       0,
			Grade:       "F",
			Maintenance: 0,
			Stewardship: 0,
			Integrity:   0,
			Signals:     []string{"empty bundle (0 concepts)"},
			Summary:     "Empty bundle: Grade F (Score: 0/100)",
		}
	}

	today := time.Now().UTC().Format("2006-01-02")
	totalConcepts := len(b.Concepts)
	var signals []string

	// ------------------------------------------------------------------------
	// Pillar 1: Epistemic Maintenance & Relevance (35%)
	// ------------------------------------------------------------------------
	var sumMaintenance float64
	lapsedCount := 0
	durableCount := 0
	deprecatedCount := 0

	// Check if root bundle is deprecated
	rootDeprecated := false
	if rootIndex, ok := b.Indexes["index.md"]; ok {
		fmRaw, _, hasFM := ExtractFrontmatter(rootIndex)
		if hasFM {
			for _, line := range strings.Split(fmRaw, "\n") {
				line = strings.TrimSpace(line)
				if after, found := strings.CutPrefix(line, "status:"); found {
					val := strings.ToLower(strings.Trim(strings.TrimSpace(after), `"'`))
					if val == "deprecated" {
						rootDeprecated = true
					}
				}
			}
		}
	}

	for _, c := range b.Concepts {
		status := strings.ToLower(strings.TrimSpace(c.Status))
		if status == "deprecated" {
			deprecatedCount++
			continue
		}

		conceptScore := 100.0
		if c.StaleAfter != "" {
			if isoDateRegex.MatchString(c.StaleAfter) {
				if c.StaleAfter < today {
					conceptScore = 0.0
					lapsedCount++
				}
			} else {
				conceptScore = 0.0
				lapsedCount++
			}
		} else {
			durableCount++
		}

		if status == "draft" {
			conceptScore *= 0.7
		}

		sumMaintenance += conceptScore
	}

	maintenanceScore := int(math.Round(sumMaintenance / float64(totalConcepts)))
	if rootDeprecated {
		maintenanceScore = 0
		signals = append(signals, "bundle root marked as deprecated (maintenance zeroed)")
	} else {
		if lapsedCount > 0 {
			signals = append(signals, fmt.Sprintf("%d/%d concept(s) have lapsed review deadlines (stale_after)", lapsedCount, totalConcepts))
		}
		if deprecatedCount > 0 {
			signals = append(signals, fmt.Sprintf("%d/%d concept(s) marked as deprecated", deprecatedCount, totalConcepts))
		}
		if lapsedCount == 0 && deprecatedCount == 0 {
			signals = append(signals, fmt.Sprintf("all %d concept(s) active and current (%d durable invariants)", totalConcepts, durableCount))
		}
	}
	maintenanceScore = clamp(maintenanceScore, 0, 100)

	// ------------------------------------------------------------------------
	// Pillar 2: Human Stewardship & Provenance (35%)
	// ------------------------------------------------------------------------
	var sumStewardship float64
	humanCount := 0
	syntheticCount := 0

	for _, c := range b.Concepts {
		hasCurrentHumanVerifier := TrustTier(c) == TrustTierHumanReviewed
		isHumanAuthor := c.Generated != nil && IsHumanIdentity(c.Generated.By)

		if hasCurrentHumanVerifier || isHumanAuthor {
			sumStewardship += 100.0
			humanCount++
		} else if c.Generated != nil && strings.TrimSpace(c.Generated.By) != "" {
			sumStewardship += 20.0
			syntheticCount++
		} else {
			sumStewardship += 0.0
		}
	}

	stewardshipScore := clamp(int(math.Round(sumStewardship/float64(totalConcepts))), 0, 100)
	if humanCount == totalConcepts {
		signals = append(signals, fmt.Sprintf("100%% human stewardship (%d/%d verified or authored by humans)", humanCount, totalConcepts))
	} else if humanCount > 0 {
		signals = append(signals, fmt.Sprintf("%d/%d concept(s) verified or authored by humans", humanCount, totalConcepts))
	} else {
		signals = append(signals, fmt.Sprintf("zero human verifications (%d synthetic unreviewed concepts)", syntheticCount))
	}

	// ------------------------------------------------------------------------
	// Pillar 3: Graph & Conformance Integrity (30%)
	// ------------------------------------------------------------------------
	vRes := Validate(b, ValidateOptions{Strict: true})
	integrityScore := 100

	integrityScore -= len(vRes.Errors) * 25
	integrityScore -= len(vRes.BrokenLinks) * 15
	integrityScore -= len(vRes.Orphans) * 10
	integrityScore -= len(vRes.GateFindings) * 10
	integrityScore -= len(vRes.Warnings) * 5
	if !vRes.IsConformant {
		integrityScore -= 25
	}
	integrityScore = clamp(integrityScore, 0, 100)

	if len(vRes.Errors) == 0 && len(vRes.BrokenLinks) == 0 && len(vRes.Orphans) == 0 && len(vRes.Warnings) == 0 {
		signals = append(signals, "strict conformance passed: 0 broken links, 0 orphans, valid DAG")
	} else {
		if len(vRes.Errors) > 0 {
			signals = append(signals, fmt.Sprintf("%d validation error(s)", len(vRes.Errors)))
		}
		if len(vRes.BrokenLinks) > 0 {
			signals = append(signals, fmt.Sprintf("%d broken link(s)", len(vRes.BrokenLinks)))
		}
		if len(vRes.Orphans) > 0 {
			signals = append(signals, fmt.Sprintf("%d orphan concept(s)", len(vRes.Orphans)))
		}
		if len(vRes.Warnings) > 0 {
			signals = append(signals, fmt.Sprintf("%d warning(s)", len(vRes.Warnings)))
		}
	}

	// ------------------------------------------------------------------------
	// Weighted Total Score & Grade
	// ------------------------------------------------------------------------
	weighted := 0.35*float64(maintenanceScore) + 0.35*float64(stewardshipScore) + 0.30*float64(integrityScore)
	totalScore := clamp(int(math.Round(weighted)), 0, 100)

	var grade string
	switch {
	case totalScore >= 90:
		grade = "A"
	case totalScore >= 80:
		grade = "B"
	case totalScore >= 70:
		grade = "C"
	case totalScore >= 60:
		grade = "D"
	default:
		grade = "F"
	}

	summary := fmt.Sprintf("Quality Scorecard: Grade %s (Score: %d/100) — M:%d%% S:%d%% I:%d%%",
		grade, totalScore, maintenanceScore, stewardshipScore, integrityScore)

	return &ScorecardResult{
		Score:       totalScore,
		Grade:       grade,
		Maintenance: maintenanceScore,
		Stewardship: stewardshipScore,
		Integrity:   integrityScore,
		Signals:     signals,
		Summary:     summary,
	}
}

func clamp(val, minVal, maxVal int) int {
	if val < minVal {
		return minVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}
