package okf

import (
	"regexp"
	"strings"
)

// Actor format regex: <producer>/<version>, human:<id>, or process:<id>
var actorRegex = regexp.MustCompile(`^(?:[a-zA-Z][\w.-]*:\S+|[^\s/]+/[^\s/]+)$`)

// Known standard actor prefixes
var standardActorPrefixes = map[string]bool{
	"human":   true,
	"process": true,
}

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
	StaleAfter  string               `json:"stale_after,omitempty"` // YYYY-MM-DD
	Sources     []Source             `json:"sources,omitempty"`
	Attestation *AttestedComputation `json:"attestation,omitempty"`
	Extra       map[string]any       `json:"extra,omitempty"` // Preserved unknown fields
	Body        string               `json:"body"`            // Markdown body after frontmatter
	RawContent  string               `json:"raw_content,omitempty"`
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

// GetNonStandardPrefix returns the prefix if the actor uses a non-standard prefix (outside human, process, producer/ver).
func GetNonStandardPrefix(actor string) string {
	if !strings.Contains(actor, ":") {
		return ""
	}
	parts := strings.SplitN(actor, ":", 2)
	prefix := strings.ToLower(parts[0])
	if !standardActorPrefixes[prefix] {
		return prefix
	}
	return ""
}
