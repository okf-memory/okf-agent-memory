package okf

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"time"
)

// ErrProtectedConcept is returned when a non-human actor attempts to modify a
// write-protected concept (x-okf-memory-protected: true) or downgrade an
// existing governance: hold without human identity or explicit AllowProtected override.
var ErrProtectedConcept = errors.New("concept is write-protected")

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

// HasHumanProvenance reports whether c carries human authorship (generated.by)
// or a current human verification event (verified.by with verified.at >= generated.at).
func HasHumanProvenance(c *Concept) bool {
	if c == nil {
		return false
	}
	if c.Generated != nil && IsHumanIdentity(c.Generated.By) {
		return true
	}
	return TrustTier(c) == TrustTierHumanReviewed
}

// ensureWriteAllowed enforces write protection on existing concept files on disk:
//  1. Concepts with x-okf-memory-protected: true reject edits from non-human actors
//     unless allowProtected is set, including attempts to strip or unset the key.
//  2. Concepts with governance: hold reject edits from non-human actors that
//     downgrade or clear governance unless allowProtected is set.
func ensureWriteAllowed(fullPath string, c *Concept, actor string, allowProtected bool) error {
	if allowProtected || IsHumanIdentity(actor) {
		return nil
	}
	// #nosec G304 -- fullPath is resolved and confined to the bundle by resolveInBundle
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil
	}
	existing, err := ParseConcept("", string(data))
	if err != nil {
		return nil
	}
	target := c.Path
	if target == "" {
		target = c.ID
	}
	if existing.IsProtected() {
		return fmt.Errorf("%w: %q declares %s: true and cannot be modified by non-human actor %q (use --allow-protected via CLI or edit as a human actor)", ErrProtectedConcept, target, ProtectedExtensionKey, actor)
	}
	if strings.EqualFold(strings.TrimSpace(existing.Governance), GovernanceHold) &&
		!strings.EqualFold(strings.TrimSpace(c.Governance), GovernanceHold) {
		return fmt.Errorf("%w: %q has governance: hold and cannot be downgraded to %q by non-human actor %q (use --allow-protected via CLI or edit as a human actor)", ErrProtectedConcept, target, c.Governance, actor)
	}
	return nil
}
