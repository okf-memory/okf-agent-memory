package okf

import (
	"testing"
	"time"
)

func TestBundleSearchWithOptions(t *testing.T) {
	b := &Bundle{
		Concepts: map[string]*Concept{
			"decisions/adr-001": {
				ID:          "decisions/adr-001",
				Title:       "PostgreSQL Migration",
				Type:        "Decision",
				Description: "Migrating from SQLite to PostgreSQL",
				Status:      "stable",
				Governance:  "constraint",
				Tags:        []string{"database", "sql"},
				StaleAfter:  "2026-10-01",
				Verified: []Verified{
					{By: "human:lead-architect", At: "2026-09-01"},
				},
				Body: "Database connection pooling and migration roadmap.",
			},
			"architecture/caching": {
				ID:          "architecture/caching",
				Title:       "Redis Caching Strategy",
				Type:        "Architecture",
				Description: "In-memory caching layer with Redis",
				Status:      "draft",
				Governance:  "context",
				Tags:        []string{"cache", "performance"},
				StaleAfter:  "2026-11-15",
				Generated: &Generated{
					By: "agent/claude",
					At: "2026-09-10",
				},
				Body: "Redis LRU cache configuration.",
			},
			"facts/api-spec": {
				ID:          "facts/api-spec",
				Title:       "Public API Specification",
				Type:        "Fact",
				Description: "OpenAPI specification endpoint",
				Status:      "stable",
				Governance:  "context",
				Tags:        []string{"api", "rest"},
				StaleAfter:  "2026-09-20", // already stale
				Body:        "API contracts and endpoints.",
			},
		},
		Graph:        make(map[string][]string),
		InboundGraph: make(map[string][]string),
	}

	t.Run("Filter by type Decision", func(t *testing.T) {
		res, err := b.SearchAdvanced(SearchOptions{
			Filter: "type=Decision",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 1 || res[0].ConceptID != "decisions/adr-001" {
			t.Errorf("expected 1 result (decisions/adr-001), got %+v", res)
		}
	})

	t.Run("Filter by verified human", func(t *testing.T) {
		res, err := b.SearchAdvanced(SearchOptions{
			Filter: "verified.by=human",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 1 || res[0].ConceptID != "decisions/adr-001" {
			t.Errorf("expected adr-001, got %+v", res)
		}
	})

	t.Run("Filter by StaleWithin 14d", func(t *testing.T) {
		// Mock base time for test consistency: 2026-09-25
		baseTime := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
		res, err := b.SearchAdvanced(SearchOptions{
			StaleWithin: 14 * 24 * time.Hour,
			BaseTime:    baseTime,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Expect "facts/api-spec" (stale 2026-09-20) and "decisions/adr-001" (stale 2026-10-01 is within 14d)
		if len(res) != 2 {
			t.Errorf("expected 2 concepts within 14d horizon, got %d: %+v", len(res), res)
		}
	})

	t.Run("Combined text query with Filter", func(t *testing.T) {
		res, err := b.SearchAdvanced(SearchOptions{
			Query:  "database",
			Filter: "status=stable",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 1 || res[0].ConceptID != "decisions/adr-001" {
			t.Errorf("expected adr-001, got %+v", res)
		}
	})
}
