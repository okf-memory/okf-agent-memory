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

// ProtectedExtensionKey is the OKF v0.3 extension frontmatter key that marks a
// concept as write-protected against non-human mutations.
const ProtectedExtensionKey = "x-okf-memory-protected"

// MaxConceptDirectoryDepth defines the maximum allowed directory nesting level for concept IDs.
const MaxConceptDirectoryDepth = 8

// Actor format regex: <producer>/<version> or <prefix>:<id> (OKF v0.2 §7 open family)
var actorRegex = regexp.MustCompile(`^(?:[a-zA-Z][\w.-]*:\S+|[^\s/]+/[^\s/]+)$`)

// Concept represents one non-reserved .md file in an OKF bundle.
type Concept struct {
	ID            string               `json:"id"`   // bundle-relative path without .md (e.g. "tables/orders")
	Path          string               `json:"path"` // bundle-relative path with .md (e.g. "tables/orders.md")
	Type          string               `json:"type"` // Required non-empty concept type
	Title         string               `json:"title,omitempty"`
	Description   string               `json:"description,omitempty"`
	Resource      string               `json:"resource,omitempty"`
	Tags          []string             `json:"tags,omitempty"`
	Generated     *Generated           `json:"generated,omitempty"`
	Verified      []Verified           `json:"verified,omitempty"`
	Status        string               `json:"status,omitempty"`        // draft | stable | deprecated
	SupersededBy  string               `json:"superseded_by,omitempty"` // OKF v0.3 §5.4 replacement concept path
	Governance    string               `json:"governance,omitempty"`    // constraint | hold | context
	CodeRefs      []string             `json:"code_refs,omitempty"`     // Referenced source paths or globs
	StaleAfter    string               `json:"stale_after,omitempty"`   // YYYY-MM-DD
	Sources       []Source             `json:"sources,omitempty"`
	Attestation   *AttestedComputation `json:"attestation,omitempty"`
	Extra         map[string]any       `json:"extra,omitempty"`          // Preserved unknown fields
	ParseWarnings []string             `json:"parse_warnings,omitempty"` // Fault-tolerant parse diagnostics for validator
	Body          string               `json:"body"`                     // Markdown body after frontmatter
	RawContent    string               `json:"raw_content,omitempty"`
	extraBlocks   map[string]bool
	sourceHash    string
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

// IsProtected reports whether the concept declares x-okf-memory-protected: true
// in its frontmatter extension fields.
func (c *Concept) IsProtected() bool {
	if c == nil || len(c.Extra) == 0 {
		return false
	}
	if val, ok := c.Extra[ProtectedExtensionKey]; ok {
		return isTruthyFlag(val)
	}
	for k, val := range c.Extra {
		if strings.EqualFold(strings.TrimSpace(k), ProtectedExtensionKey) {
			return isTruthyFlag(val)
		}
	}
	return false
}

func isTruthyFlag(v any) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return strings.EqualFold(strings.TrimSpace(val), "true")
	default:
		return false
	}
}

// Generated records who authored the concept and when.
type Generated struct {
	By    string            `json:"by"`
	At    string            `json:"at"`
	Extra map[string]string `json:"extra,omitempty"`
}

// Verified records human or machine verification events.
type Verified struct {
	By    string            `json:"by"`
	At    string            `json:"at"`
	Extra map[string]string `json:"extra,omitempty"`
}

// Source records provenance origin for claims.
type Source struct {
	ID           string            `json:"id,omitempty"`
	Resource     string            `json:"resource"`
	Title        string            `json:"title,omitempty"`
	Author       string            `json:"author,omitempty"`
	LastModified string            `json:"last_modified,omitempty"`
	UsageCount   int               `json:"usage_count,omitempty"`
	Extra        map[string]string `json:"extra,omitempty"`
}

// Executor describes how to run an attested computation (OKF v0.3 §10.2).
type Executor struct {
	Resource string            `json:"resource,omitempty"`
	Receipt  []string          `json:"receipt,omitempty"`
	Extra    map[string]string `json:"extra,omitempty"`
}

// Attester describes the deterministic verification check for a computation receipt (OKF v0.3 §10.2).
type Attester struct {
	Resource string            `json:"resource,omitempty"`
	Extra    map[string]string `json:"extra,omitempty"`
}

// AttestedComputation metadata for verifiable computations.
type AttestedComputation struct {
	Runtime     string      `json:"runtime,omitempty"`
	Computation string      `json:"computation,omitempty"`
	Executor    *Executor   `json:"executor,omitempty"`
	Attester    *Attester   `json:"attester,omitempty"`
	Parameters  []Parameter `json:"parameters,omitempty"`
}

// Parameter defines a parameter for an attested computation.
type Parameter struct {
	Name     string            `json:"name"`
	Type     string            `json:"type,omitempty"`
	Required bool              `json:"required,omitempty"`
	Extra    map[string]string `json:"extra,omitempty"`
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
