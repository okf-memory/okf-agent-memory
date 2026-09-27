package okf

import (
	"testing"
	"time"
)

func TestParseRelativeDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"14d", 14 * 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"1w", 7 * 24 * time.Hour, false},
		{"1m", 30 * 24 * time.Hour, false},
		{"3m", 90 * 24 * time.Hour, false},
		{"1y", 365 * 24 * time.Hour, false},
		{"48h", 48 * time.Hour, false},
		{"", 0, true},
		{"invalid", 0, true},
		{"-5d", 0, true},
	}

	for _, tt := range tests {
		got, err := ParseRelativeDuration(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseRelativeDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.expected {
			t.Errorf("ParseRelativeDuration(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestConceptMatchesFilter(t *testing.T) {
	c := &Concept{
		ID:          "auth/oauth2",
		Type:        "Decision",
		Title:       "OAuth2 Implementation",
		Description: "OAuth2 authentication details",
		Status:      "stable",
		Governance:  "constraint",
		Tags:        []string{"auth", "security", "jwt"},
		StaleAfter:  "2026-10-15",
		Generated: &Generated{
			By: "agent/claude",
			At: "2026-09-01T12:00:00Z",
		},
		Verified: []Verified{
			{By: "human:reviewer", At: "2026-09-05T10:00:00Z"},
		},
		Extra: map[string]any{
			"team": "core-infra",
		},
	}

	tests := []struct {
		filter string
		match  bool
	}{
		// Basic exact matches
		{"type=Decision", true},
		{"type=Fact", false},
		{"status=stable", true},
		{"status=deprecated", false},
		{"governance=constraint", true},
		{"id=auth/oauth2", true},

		// Inequality matches
		{"type!=Fact", true},
		{"type!=Decision", false},
		{"status!=deprecated", true},

		// Tag array matching
		{"tags=security", true},
		{"tags=database", false},
		{"tags!=database", true},

		// Nested fields: verified
		{"verified.by=human:reviewer", true},
		{"verified.by=bot", false},
		{"verified.by!=null", true},
		{"verified.by!=nil", true},
		{"verified!=null", true},

		// Nested fields: generated
		{"generated.by=agent/claude", true},
		{"generated.by!=null", true},

		// Extra fields
		{"team=core-infra", true},
		{"team!=security", true},

		// Empty / null checks for unset fields
		{"nonexistent=null", true},
		{"nonexistent!=null", false},

		// Multiple comma-separated conditions
		{"type=Decision,governance=constraint", true},
		{"type=Decision,status=deprecated", false},
	}

	for _, tt := range tests {
		got, err := c.MatchesFilter(tt.filter)
		if err != nil {
			t.Errorf("MatchesFilter(%q) returned unexpected error: %v", tt.filter, err)
			continue
		}
		if got != tt.match {
			t.Errorf("MatchesFilter(%q) = %v, want %v", tt.filter, got, tt.match)
		}
	}
}

func TestConceptIsStaleWithin(t *testing.T) {
	// Base date: 2026-09-25
	baseDate := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		staleAfter  string
		within      time.Duration
		expectMatch bool
	}{
		{
			name:        "Stale 5 days ago (already stale)",
			staleAfter:  "2026-09-20",
			within:      14 * 24 * time.Hour,
			expectMatch: true,
		},
		{
			name:        "Stale in 10 days (within 14d horizon)",
			staleAfter:  "2026-10-05",
			within:      14 * 24 * time.Hour,
			expectMatch: true,
		},
		{
			name:        "Stale in 20 days (outside 14d horizon)",
			staleAfter:  "2026-10-15",
			within:      14 * 24 * time.Hour,
			expectMatch: false,
		},
		{
			name:        "No stale_after declared",
			staleAfter:  "",
			within:      14 * 24 * time.Hour,
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		c := &Concept{
			ID:         "test/doc",
			StaleAfter: tt.staleAfter,
		}
		got := c.IsStaleWithin(baseDate, tt.within)
		if got != tt.expectMatch {
			t.Errorf("%s: IsStaleWithin(..., %v) = %v, want %v", tt.name, tt.within, got, tt.expectMatch)
		}
	}
}
