package okf

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseRelativeDuration parses a human-friendly duration string.
// Supports days ("d"), weeks ("w"), months ("m" or "mo", 30 days), years ("y", 365 days),
// as well as standard Go durations (e.g. "48h").
func ParseRelativeDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}

	if strings.HasPrefix(s, "-") {
		return 0, fmt.Errorf("negative durations are not allowed: %s", s)
	}

	// Days: e.g. "14d"
	if after, ok := strings.CutSuffix(s, "d"); ok {
		days, err := strconv.Atoi(after)
		if err != nil || days <= 0 {
			return 0, fmt.Errorf("invalid day duration: %s", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	// Weeks: e.g. "2w"
	if after, ok := strings.CutSuffix(s, "w"); ok {
		weeks, err := strconv.Atoi(after)
		if err != nil || weeks <= 0 {
			return 0, fmt.Errorf("invalid week duration: %s", s)
		}
		return time.Duration(weeks) * 7 * 24 * time.Hour, nil
	}

	// Months: e.g. "3mo" or "1m" (when not followed by 's' for minutes)
	if after, ok := strings.CutSuffix(s, "mo"); ok {
		months, err := strconv.Atoi(after)
		if err != nil || months <= 0 {
			return 0, fmt.Errorf("invalid month duration: %s", s)
		}
		return time.Duration(months) * 30 * 24 * time.Hour, nil
	}

	// Check if ending in 'm' could be month if it's "1m", "3m" and not e.g. "30m" where standard duration could apply.
	// But in CLI horizon terms, "1m", "3m", "6m" almost always mean months.
	// We disambiguate: if "m" is used, interpret as 30 days unless standard time.ParseDuration applies and has minutes.
	// In our specification, "1m", "3m" are treated as 30-day months.
	if after, ok := strings.CutSuffix(s, "m"); ok {
		// If it's a pure integer before 'm', treat as months (e.g. 1m, 3m).
		if val, err := strconv.Atoi(after); err == nil && val > 0 {
			return time.Duration(val) * 30 * 24 * time.Hour, nil
		}
	}

	// Years: e.g. "1y"
	if after, ok := strings.CutSuffix(s, "y"); ok {
		years, err := strconv.Atoi(after)
		if err != nil || years <= 0 {
			return 0, fmt.Errorf("invalid year duration: %s", s)
		}
		return time.Duration(years) * 365 * 24 * time.Hour, nil
	}

	// Standard Go duration fallback (e.g. "48h", "72h")
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("invalid duration format %q (expected e.g. 14d, 2w, 1m, 48h)", s)
	}
	return d, nil
}

// IsStaleWithin checks whether this concept is already stale or will become stale
// within the given duration relative to baseTime.
func (c *Concept) IsStaleWithin(baseTime time.Time, duration time.Duration) bool {
	if c.StaleAfter == "" {
		return false
	}
	staleDate, err := time.Parse("2006-01-02", c.StaleAfter)
	if err != nil {
		return false
	}
	// Target horizon is baseTime + duration
	horizon := baseTime.Add(duration)
	// Truncate to day resolution for clean comparison
	horizonDate := time.Date(horizon.Year(), horizon.Month(), horizon.Day(), 23, 59, 59, 0, time.UTC)
	return !staleDate.After(horizonDate)
}

type filterPredicate struct {
	key      string
	operator string // "=" or "!="
	value    string
}

func parseFilterClauses(filterExpr string) ([]filterPredicate, error) {
	filterExpr = strings.TrimSpace(filterExpr)
	if filterExpr == "" {
		return nil, nil
	}

	// Split by comma or semicolon
	clauses := strings.FieldsFunc(filterExpr, func(r rune) bool {
		return r == ',' || r == ';'
	})

	var preds []filterPredicate
	for _, clause := range clauses {
		clause = strings.TrimSpace(clause)
		if clause == "" {
			continue
		}

		var key, op, val string
		if idx := strings.Index(clause, "!="); idx != -1 {
			key = strings.TrimSpace(clause[:idx])
			op = "!="
			val = strings.TrimSpace(clause[idx+2:])
		} else if idx := strings.Index(clause, "="); idx != -1 {
			key = strings.TrimSpace(clause[:idx])
			op = "="
			val = strings.TrimSpace(clause[idx+1:])
		} else {
			return nil, fmt.Errorf("invalid filter clause %q: must contain '=' or '!='", clause)
		}

		// Strip optional outer quotes from val
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}

		preds = append(preds, filterPredicate{
			key:      strings.ToLower(key),
			operator: op,
			value:    val,
		})
	}

	return preds, nil
}

// MatchesFilter evaluates whether the concept satisfies the provided filter expression.
// Supports comma-separated clauses with "=" and "!=", e.g.:
// "type=Decision,status=stable"
// "verified.by=human,governance!=hold"
// "tags=security"
// "verified.by!=null"
func (c *Concept) MatchesFilter(filterExpr string) (bool, error) {
	preds, err := parseFilterClauses(filterExpr)
	if err != nil {
		return false, err
	}
	if len(preds) == 0 {
		return true, nil
	}

	for _, p := range preds {
		matched := c.evalPredicate(p)
		if !matched {
			return false, nil
		}
	}
	return true, nil
}

func (c *Concept) evalPredicate(p filterPredicate) bool {
	isNullCheck := p.value == "null" || p.value == "nil" || p.value == ""

	switch p.key {
	case "type":
		return compareString(c.Type, p.operator, p.value, isNullCheck)
	case "title":
		return compareString(c.Title, p.operator, p.value, isNullCheck)
	case "description", "desc":
		return compareString(c.Description, p.operator, p.value, isNullCheck)
	case "status":
		return compareString(c.Status, p.operator, p.value, isNullCheck)
	case "governance":
		return compareString(c.EffectiveGovernance(), p.operator, p.value, isNullCheck)
	case "id":
		return compareString(c.ID, p.operator, p.value, isNullCheck)
	case "path":
		return compareString(c.Path, p.operator, p.value, isNullCheck)
	case "stale_after":
		return compareString(c.StaleAfter, p.operator, p.value, isNullCheck)

	case "tags", "tag":
		if isNullCheck {
			hasTags := len(c.Tags) > 0
			if p.operator == "!=" {
				return hasTags
			}
			return !hasTags
		}
		contains := false
		for _, t := range c.Tags {
			if strings.EqualFold(t, p.value) {
				contains = true
				break
			}
		}
		if p.operator == "=" {
			return contains
		}
		return !contains

	case "code_refs", "code_ref":
		if isNullCheck {
			hasRefs := len(c.CodeRefs) > 0
			if p.operator == "!=" {
				return hasRefs
			}
			return !hasRefs
		}
		contains := false
		for _, ref := range c.CodeRefs {
			if strings.EqualFold(ref, p.value) {
				contains = true
				break
			}
		}
		if p.operator == "=" {
			return contains
		}
		return !contains

	case "verified":
		hasVerified := len(c.Verified) > 0
		if isNullCheck {
			if p.operator == "!=" {
				return hasVerified
			}
			return !hasVerified
		}
		// If value given, check if any verified entry matches by Actor
		for _, v := range c.Verified {
			if strings.EqualFold(v.By, p.value) {
				return p.operator == "="
			}
		}
		return p.operator == "!="

	case "verified.by":
		if isNullCheck {
			hasAny := false
			for _, v := range c.Verified {
				if v.By != "" {
					hasAny = true
					break
				}
			}
			if p.operator == "!=" {
				return hasAny
			}
			return !hasAny
		}
		for _, v := range c.Verified {
			if strings.EqualFold(v.By, p.value) || (p.value == "human" && strings.HasPrefix(strings.ToLower(v.By), "human")) {
				return p.operator == "="
			}
		}
		return p.operator == "!="

	case "verified.at":
		if isNullCheck {
			hasAny := false
			for _, v := range c.Verified {
				if v.At != "" {
					hasAny = true
					break
				}
			}
			if p.operator == "!=" {
				return hasAny
			}
			return !hasAny
		}
		for _, v := range c.Verified {
			if strings.EqualFold(v.At, p.value) {
				return p.operator == "="
			}
		}
		return p.operator == "!="

	case "generated":
		hasGen := c.Generated != nil
		if isNullCheck {
			if p.operator == "!=" {
				return hasGen
			}
			return !hasGen
		}
		if c.Generated == nil {
			return p.operator == "!="
		}
		return compareString(c.Generated.By, p.operator, p.value, false)

	case "generated.by":
		val := ""
		if c.Generated != nil {
			val = c.Generated.By
		}
		return compareString(val, p.operator, p.value, isNullCheck)

	case "generated.at":
		val := ""
		if c.Generated != nil {
			val = c.Generated.At
		}
		return compareString(val, p.operator, p.value, isNullCheck)

	default:
		// Check extra frontmatter map
		if c.Extra != nil {
			if extraVal, exists := c.Extra[p.key]; exists {
				strVal := fmt.Sprintf("%v", extraVal)
				return compareString(strVal, p.operator, p.value, isNullCheck)
			}
		}
		// If key not present
		if isNullCheck {
			return p.operator == "="
		}
		return p.operator == "!="
	}
}

func compareString(actual, op, expected string, isNullCheck bool) bool {
	if isNullCheck {
		isEmpty := strings.TrimSpace(actual) == ""
		if op == "=" {
			return isEmpty
		}
		return !isEmpty
	}

	equal := strings.EqualFold(actual, expected)
	if op == "=" {
		return equal
	}
	return !equal
}
