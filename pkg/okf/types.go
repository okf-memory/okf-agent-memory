package okf

import (
	"regexp"
	"strings"
)

// Governance level constants defining agent authority over code modifications.
const (
	GovernanceConstraint = "constraint"
	GovernanceHold       = "hold"
	GovernanceContext    = "context"
)

// Actor format regex: <producer>/<version> or <prefix>:<id> (OKF v0.2 §7 open family)
var actorRegex = regexp.MustCompile(`^(?:[a-zA-Z][\w.-]*:\S+|[^\s/]+/[^\s/]+)$`)

// Concept represents one non-reserved .md file in an OKF bundle.
type Concept struct {
	ID          string               `json:"id"`   // bundle-relative path without .md (e.g. "tables/orders")
	Path        string               `json:"path"` // bundle-relative path with .md (e.g. "tables/orders.md")
	Type        string               `json:"type"` // Required non-empty concept type
	Title       string               `json:"title,omitempty"`
	Description string               `json:"description,omitempty"`
	Resource    string               `json:"resource,omitempty"`
	Tags        []string             `json:"tags,omitempty"`
	Generated   *Generated           `json:"generated,omitempty"`
	Verified    []Verified           `json:"verified,omitempty"`
	Status      string               `json:"status,omitempty"`      // draft | stable | deprecated
	Governance  string               `json:"governance,omitempty"`  // constraint | hold | context
	CodeRefs    []string             `json:"code_refs,omitempty"`   // Referenced source paths or globs
	StaleAfter  string               `json:"stale_after,omitempty"` // YYYY-MM-DD
	Sources     []Source             `json:"sources,omitempty"`
	Attestation *AttestedComputation `json:"attestation,omitempty"`
	Extra       map[string]any       `json:"extra,omitempty"` // Preserved unknown fields
	Body        string               `json:"body"`            // Markdown body after frontmatter
	RawContent  string               `json:"raw_content,omitempty"`
	extraBlocks map[string]bool
}

// EffectiveGovernance returns the effective governance level of the concept:
// "hold" (execution freeze / manual signoff required),
// "constraint" (mandatory rules/guardrails for code modifications), or
// "context" (informative domain knowledge).
// If explicitly declared, c.Governance is normalized to lowercase and returned.
// Otherwise, concepts with ID starting with "convention/" default to "constraint".
// All other concepts default to "context".
func (c *Concept) EffectiveGovernance() string {
	if c.Governance != "" {
		return strings.ToLower(c.Governance)
	}
	if strings.HasPrefix(c.ID, "convention/") {
		return GovernanceConstraint
	}
	return GovernanceContext
}

// Generated records who authored the concept and when.
type Generated struct {
	By string `json:"by"`
	At string `json:"at"`
}

// Verified records human or machine verification events.
type Verified struct {
	By string `json:"by"`
	At string `json:"at"`
}

// Source records provenance origin for claims.
type Source struct {
	ID           string `json:"id,omitempty"`
	Resource     string `json:"resource"`
	Title        string `json:"title,omitempty"`
	Author       string `json:"author,omitempty"`
	LastModified string `json:"last_modified,omitempty"`
	UsageCount   int    `json:"usage_count,omitempty"`
}

// AttestedComputation metadata for verifiable computations.
type AttestedComputation struct {
	Runtime     string      `json:"runtime,omitempty"`
	Computation string      `json:"computation,omitempty"`
	Executor    string      `json:"executor,omitempty"`
	Attester    string      `json:"attester,omitempty"`
	Parameters  []Parameter `json:"parameters,omitempty"`
}

// Parameter defines a parameter for an attested computation.
type Parameter struct {
	Name     string `json:"name"`
	Type     string `json:"type,omitempty"`
	Required bool   `json:"required,omitempty"`
}

// IsValidActor checks if an actor string matches the OKF specification.
func IsValidActor(actor string) bool {
	if actor == "" {
		return false
	}
	return actorRegex.MatchString(actor)
}

// GetNonStandardPrefix returns empty string.
// Deprecated: In OKF v0.2 §7, the <prefix>:<id> family is open (not a whitelist).
func GetNonStandardPrefix(actor string) string {
	return ""
}
