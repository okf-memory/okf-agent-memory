package okf

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"time"
)

// Trust tier constants defined by OKF v0.3 §5.3.
const (
	TrustTierUnverified       = "unverified"
	TrustTierMachineConfirmed = "machine-confirmed"
	TrustTierHumanReviewed    = "human-reviewed"
)

// CurrentVerifications returns the valid verification events on c that are current
// per OKF v0.3 §5.3: both 'by' (valid actor) and 'at' (valid timestamp) are present,
// and either generated.at is absent or verified.at >= generated.at.
func CurrentVerifications(c *Concept) []Verified {
	if c == nil || len(c.Verified) == 0 {
		return nil
	}
	hasGenAt := c.Generated != nil && strings.TrimSpace(c.Generated.At) != ""
	var genT time.Time
	if hasGenAt {
		var ok bool
		genT, ok = parseTimestamp(c.Generated.At)
		if !ok {
			// Malformed generated.at: cannot confirm any verification is current.
			return nil
		}
	}
	var current []Verified
	for _, v := range c.Verified {
		by := strings.TrimSpace(v.By)
		at := strings.TrimSpace(v.At)
		if by == "" || at == "" || !IsValidActor(by) {
			continue
		}
		vT, ok := parseTimestamp(at)
		if !ok {
			continue
		}
		if hasGenAt && vT.Before(genT) {
			continue
		}
		current = append(current, v)
	}
	return current
}

// TrustTier calculates the OKF v0.3 §5.3 trust tier of c from its current
// verification events (verified.at >= generated.at, or all if generated.at is absent):
//   - TrustTierHumanReviewed ("human-reviewed") if any current event is by a human: actor
//   - TrustTierMachineConfirmed ("machine-confirmed") if current events exist from non-human actors only
//   - TrustTierUnverified ("unverified") if no current verification events exist
func TrustTier(c *Concept) string {
	current := CurrentVerifications(c)
	if len(current) == 0 {
		return TrustTierUnverified
	}
	for _, v := range current {
		if IsHumanIdentity(v.By) {
			return TrustTierHumanReviewed
		}
	}
	return TrustTierMachineConfirmed
}

// IsHumanIdentity reports whether an actor or verifier identifier denotes a human:
// "human" alone or followed by ':' or '/', compared case-insensitively.
// It is the single definition used by the provenance guard and by the verified.by filter.
func IsHumanIdentity(id string) bool {
	lower := strings.ToLower(strings.TrimSpace(id))
	if lower == "human" {
		return true
	}
	return strings.HasPrefix(lower, "human:") || strings.HasPrefix(lower, "human/")
}

// ensureNoForgedHumanVerification rejects human verifications that a non-human actor adds to c.
// Verifications already recorded for the concept at fullPath may be preserved. A new concept
// inherits nothing from a file that already exists at its path.
//
// The actor is self-declared, so this guards callers that cannot choose it (the MCP server)
// and accidental or injected forgery; it does not authenticate a CLI user who passes --actor.
func ensureNoForgedHumanVerification(fullPath string, c *Concept, actor string, isNew bool) error {
	if IsHumanIdentity(actor) {
		return nil
	}
	var recorded []Verified
	if !isNew {
		recorded = recordedVerifications(fullPath)
	}
	for _, v := range c.Verified {
		if IsHumanIdentity(v.By) && !hasRecordedVerification(recorded, v) {
			return fmt.Errorf("actor %q cannot add human verification %q", actor, v.By)
		}
	}
	return nil
}

func hasRecordedVerification(recorded []Verified, target Verified) bool {
	return slices.ContainsFunc(recorded, func(r Verified) bool {
		return r.By == target.By && r.At == target.At && maps.Equal(r.Extra, target.Extra)
	})
}

// recordedVerifications returns the verifications stored in the concept file at fullPath.
func recordedVerifications(fullPath string) []Verified {
	// #nosec G304 -- fullPath is resolved and confined to the bundle by resolveInBundle
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil
	}
	existing, err := ParseConcept("", string(data))
	if err != nil {
		return nil
	}
	return existing.Verified
}
