package okf_test

import (
	"testing"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

func TestTrustTierTransitions(t *testing.T) {
	tests := []struct {
		name    string
		concept *okf.Concept
		want    string
	}{
		{
			name:    "nil concept is unverified",
			concept: nil,
			want:    okf.TrustTierUnverified,
		},
		{
			name: "no generated and no verified is unverified",
			concept: &okf.Concept{
				ID:   "a",
				Type: "Note",
			},
			want: okf.TrustTierUnverified,
		},
		{
			name: "agent generated without verified is unverified",
			concept: &okf.Concept{
				ID:        "a",
				Type:      "Note",
				Generated: &okf.Generated{By: "reference_agent/gemini-2.5-pro", At: "2026-06-20T14:00:00Z"},
			},
			want: okf.TrustTierUnverified,
		},
		{
			name: "human generated without verified is unverified per SPEC v0.3 §5.3",
			concept: &okf.Concept{
				ID:        "a",
				Type:      "Note",
				Generated: &okf.Generated{By: "human:ahormati", At: "2026-06-20T14:00:00Z"},
			},
			want: okf.TrustTierUnverified,
		},
		{
			name: "historical human verification predating agent edit drops to unverified",
			concept: &okf.Concept{
				ID:        "a",
				Type:      "Note",
				Generated: &okf.Generated{By: "reference_agent/gemini-2.5-pro", At: "2026-06-25T14:00:00Z"},
				Verified: []okf.Verified{
					{By: "human:ahormati", At: "2026-06-20T09:00:00Z"},
				},
			},
			want: okf.TrustTierUnverified,
		},
		{
			name: "historical human verification plus current human verification is human-reviewed",
			concept: &okf.Concept{
				ID:        "a",
				Type:      "Note",
				Generated: &okf.Generated{By: "reference_agent/gemini-2.5-pro", At: "2026-06-25T14:00:00Z"},
				Verified: []okf.Verified{
					{By: "human:alice", At: "2026-06-20T09:00:00Z"},
					{By: "human:bob", At: "2026-06-26T09:00:00Z"},
				},
			},
			want: okf.TrustTierHumanReviewed,
		},
		{
			name: "exact timestamp equality verified.at == generated.at is current",
			concept: &okf.Concept{
				ID:        "a",
				Type:      "Note",
				Generated: &okf.Generated{By: "reference_agent/gemini-2.5-pro", At: "2026-06-25T14:00:00Z"},
				Verified: []okf.Verified{
					{By: "human:alice", At: "2026-06-25T14:00:00Z"},
				},
			},
			want: okf.TrustTierHumanReviewed,
		},
		{
			name: "timezone-equivalent timestamps compare by instant",
			concept: &okf.Concept{
				ID:        "a",
				Type:      "Note",
				Generated: &okf.Generated{By: "reference_agent/gemini-2.5-pro", At: "2026-06-25T14:00:00Z"},
				Verified: []okf.Verified{
					{By: "human:alice", At: "2026-06-25T16:00:00+02:00"},
				},
			},
			want: okf.TrustTierHumanReviewed,
		},
		{
			name: "date-only timestamp treated as UTC midnight",
			concept: &okf.Concept{
				ID:        "a",
				Type:      "Note",
				Generated: &okf.Generated{By: "reference_agent/gemini-2.5-pro", At: "2026-06-25T00:00:00Z"},
				Verified: []okf.Verified{
					{By: "human:alice", At: "2026-06-25"},
				},
			},
			want: okf.TrustTierHumanReviewed,
		},
		{
			name: "date-only verified predates later same-day generated.at",
			concept: &okf.Concept{
				ID:        "a",
				Type:      "Note",
				Generated: &okf.Generated{By: "reference_agent/gemini-2.5-pro", At: "2026-06-25T01:00:00Z"},
				Verified: []okf.Verified{
					{By: "human:alice", At: "2026-06-25"},
				},
			},
			want: okf.TrustTierUnverified,
		},
		{
			name: "generated absent treats all valid verifications as current",
			concept: &okf.Concept{
				ID:   "a",
				Type: "Note",
				Verified: []okf.Verified{
					{By: "human:alice", At: "2026-06-20T09:00:00Z"},
				},
			},
			want: okf.TrustTierHumanReviewed,
		},
		{
			name: "generated.at absent treats all valid verifications as current",
			concept: &okf.Concept{
				ID:        "a",
				Type:      "Note",
				Generated: &okf.Generated{By: "reference_agent/gemini-2.5-pro"},
				Verified: []okf.Verified{
					{By: "process:nightly", At: "2026-06-20T09:00:00Z"},
				},
			},
			want: okf.TrustTierMachineConfirmed,
		},
		{
			name: "historical human plus current process yields machine-confirmed",
			concept: &okf.Concept{
				ID:        "a",
				Type:      "Note",
				Generated: &okf.Generated{By: "reference_agent/gemini-2.5-pro", At: "2026-06-25T14:00:00Z"},
				Verified: []okf.Verified{
					{By: "human:alice", At: "2026-06-20T09:00:00Z"},
					{By: "process:finance-nightly", At: "2026-06-26T02:00:00Z"},
				},
			},
			want: okf.TrustTierMachineConfirmed,
		},
		{
			name: "current team verification yields machine-confirmed never human-reviewed",
			concept: &okf.Concept{
				ID:        "a",
				Type:      "Note",
				Generated: &okf.Generated{By: "reference_agent/gemini-2.5-pro", At: "2026-06-20T14:00:00Z"},
				Verified: []okf.Verified{
					{By: "team:platform", At: "2026-06-25T09:00:00Z"},
				},
			},
			want: okf.TrustTierMachineConfirmed,
		},
		{
			name: "current producer/version verification yields machine-confirmed",
			concept: &okf.Concept{
				ID:        "a",
				Type:      "Note",
				Generated: &okf.Generated{By: "reference_agent/gemini-2.5-pro", At: "2026-06-20T14:00:00Z"},
				Verified: []okf.Verified{
					{By: "verifier_agent/v1.2", At: "2026-06-25T09:00:00Z"},
				},
			},
			want: okf.TrustTierMachineConfirmed,
		},
		{
			name: "current custom scheme verification yields machine-confirmed",
			concept: &okf.Concept{
				ID:        "a",
				Type:      "Note",
				Generated: &okf.Generated{By: "reference_agent/gemini-2.5-pro", At: "2026-06-20T14:00:00Z"},
				Verified: []okf.Verified{
					{By: "bot:linter-v2", At: "2026-06-25T09:00:00Z"},
				},
			},
			want: okf.TrustTierMachineConfirmed,
		},
		{
			name: "current team plus current human yields human-reviewed",
			concept: &okf.Concept{
				ID:        "a",
				Type:      "Note",
				Generated: &okf.Generated{By: "reference_agent/gemini-2.5-pro", At: "2026-06-20T14:00:00Z"},
				Verified: []okf.Verified{
					{By: "team:platform", At: "2026-06-25T09:00:00Z"},
					{By: "human:ahormati", At: "2026-06-25T10:00:00Z"},
				},
			},
			want: okf.TrustTierHumanReviewed,
		},
		{
			name: "verification missing at is ignored",
			concept: &okf.Concept{
				ID:   "a",
				Type: "Note",
				Verified: []okf.Verified{
					{By: "human:alice", At: ""},
				},
			},
			want: okf.TrustTierUnverified,
		},
		{
			name: "verification missing by is ignored",
			concept: &okf.Concept{
				ID:   "a",
				Type: "Note",
				Verified: []okf.Verified{
					{By: "", At: "2026-06-25T09:00:00Z"},
				},
			},
			want: okf.TrustTierUnverified,
		},
		{
			name: "verification with invalid actor is ignored",
			concept: &okf.Concept{
				ID:   "a",
				Type: "Note",
				Verified: []okf.Verified{
					{By: "human", At: "2026-06-25T09:00:00Z"},
					{By: "human:", At: "2026-06-25T09:00:00Z"},
				},
			},
			want: okf.TrustTierUnverified,
		},
		{
			name: "malformed generated.at conservatively returns unverified",
			concept: &okf.Concept{
				ID:        "a",
				Type:      "Note",
				Generated: &okf.Generated{By: "reference_agent/gemini-2.5-pro", At: "not-a-timestamp"},
				Verified: []okf.Verified{
					{By: "human:alice", At: "2026-06-25T09:00:00Z"},
				},
			},
			want: okf.TrustTierUnverified,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := okf.TrustTier(tc.concept); got != tc.want {
				t.Errorf("TrustTier() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestV03ActorIDsAndHumanIdentity(t *testing.T) {
	validNonHuman := []string{
		"team:platform",
		"team:finance-fpa",
		"team:human",
		"process:finance-nightly",
		"reference_agent/gemini-2.5-pro",
		"custom-scheme:opaque-123",
	}
	for _, actor := range validNonHuman {
		if !okf.IsValidActor(actor) {
			t.Errorf("IsValidActor(%q) = false, want true", actor)
		}
		if okf.IsHumanIdentity(actor) {
			t.Errorf("IsHumanIdentity(%q) = true, want false", actor)
		}
	}

	validHuman := []string{
		"human:ahormati",
		"human:alice@example.com",
		"human/bob",
	}
	for _, actor := range validHuman {
		if !okf.IsValidActor(actor) {
			t.Errorf("IsValidActor(%q) = false, want true", actor)
		}
		if !okf.IsHumanIdentity(actor) {
			t.Errorf("IsHumanIdentity(%q) = false, want true", actor)
		}
	}

	invalidActors := []string{
		"",
		"team",
		"team:",
		"human",
		"human:",
		"process:",
		"1bad:id",
		"team:has space",
		"a/b/c",
	}
	for _, actor := range invalidActors {
		if okf.IsValidActor(actor) {
			t.Errorf("IsValidActor(%q) = true, want false", actor)
		}
	}
}

func TestV03ValidatorAndScorecardHistoricalVerifications(t *testing.T) {
	t.Run("historical human sign-off after agent edit: not human-reviewed, stewardship != 100, 0 gate findings", func(t *testing.T) {
		c := &okf.Concept{
			ID:        "metrics/rev",
			Path:      "metrics/rev.md",
			Type:      "Metric",
			Title:     "Revenue",
			Status:    "stable",
			Generated: &okf.Generated{By: "reference_agent/gemini-2.5-pro", At: "2026-06-25T14:00:00Z"},
			Verified: []okf.Verified{
				{By: "human:ahormati", At: "2026-06-20T09:00:00Z"},
			},
		}
		b := &okf.Bundle{
			DeclaredVer: "0.3",
			Concepts:    map[string]*okf.Concept{c.ID: c},
		}

		if got := okf.TrustTier(c); got == okf.TrustTierHumanReviewed {
			t.Fatalf("expected TrustTier != %q for historical verification, got %q", okf.TrustTierHumanReviewed, got)
		}

		res := okf.Validate(b, okf.ValidateOptions{Strict: true, Scorecard: true})
		if !res.IsConformant || !res.GatePassed {
			t.Errorf("expected conformant & gate passed, got conformant=%v gate=%v errors=%v findings=%v",
				res.IsConformant, res.GatePassed, res.Errors, res.GateFindings)
		}
		if len(res.Errors) != 0 || len(res.GateFindings) != 0 {
			t.Errorf("expected 0 errors and 0 gate findings, got errors=%v findings=%v", res.Errors, res.GateFindings)
		}
		if res.Scorecard == nil || res.Scorecard.Stewardship != 20 {
			t.Errorf("expected Stewardship=20 (not 100) for stale human review after agent edit, got %+v", res.Scorecard)
		}
	})

	t.Run("historical plus current human sign-off: human-reviewed, stewardship == 100, 0 errors, 0 gate findings", func(t *testing.T) {
		c := &okf.Concept{
			ID:        "metrics/rev",
			Path:      "metrics/rev.md",
			Type:      "Metric",
			Title:     "Revenue",
			Status:    "stable",
			Generated: &okf.Generated{By: "reference_agent/gemini-2.5-pro", At: "2026-06-25T14:00:00Z"},
			Verified: []okf.Verified{
				{By: "human:alice", At: "2026-06-20T09:00:00Z"},
				{By: "human:bob", At: "2026-06-26T09:00:00Z"},
			},
		}
		b := &okf.Bundle{
			DeclaredVer: "0.3",
			Concepts:    map[string]*okf.Concept{c.ID: c},
		}

		if got := okf.TrustTier(c); got != okf.TrustTierHumanReviewed {
			t.Fatalf("expected TrustTier %q, got %q", okf.TrustTierHumanReviewed, got)
		}

		res := okf.Validate(b, okf.ValidateOptions{Strict: true, Scorecard: true})
		if !res.IsConformant || !res.GatePassed {
			t.Errorf("expected conformant & gate passed, got errors=%v findings=%v", res.Errors, res.GateFindings)
		}
		if len(res.Errors) != 0 || len(res.GateFindings) != 0 {
			t.Errorf("expected 0 errors and 0 gate findings, got errors=%v findings=%v", res.Errors, res.GateFindings)
		}
		if res.Scorecard == nil || res.Scorecard.Stewardship != 100 {
			t.Errorf("expected Stewardship=100, got %+v", res.Scorecard)
		}
	})

	t.Run("team:platform verification is valid actor but never yields human-reviewed or 100 stewardship", func(t *testing.T) {
		c := &okf.Concept{
			ID:        "metrics/rev",
			Path:      "metrics/rev.md",
			Type:      "Metric",
			Title:     "Revenue",
			Status:    "stable",
			Generated: &okf.Generated{By: "reference_agent/gemini-2.5-pro", At: "2026-06-20T14:00:00Z"},
			Verified: []okf.Verified{
				{By: "team:platform", At: "2026-06-25T09:00:00Z"},
			},
		}
		b := &okf.Bundle{
			DeclaredVer: "0.3",
			Concepts:    map[string]*okf.Concept{c.ID: c},
		}

		if got := okf.TrustTier(c); got != okf.TrustTierMachineConfirmed {
			t.Fatalf("expected TrustTier %q for team:platform, got %q", okf.TrustTierMachineConfirmed, got)
		}

		res := okf.Validate(b, okf.ValidateOptions{Strict: true, Scorecard: true})
		if len(res.Errors) != 0 || len(res.Warnings) != 0 || len(res.GateFindings) != 0 {
			t.Errorf("expected clean validation for team:platform, got errors=%v warnings=%v findings=%v",
				res.Errors, res.Warnings, res.GateFindings)
		}
		if res.Scorecard == nil || res.Scorecard.Stewardship != 20 {
			t.Errorf("expected Stewardship=20 for team:platform verification, got %+v", res.Scorecard)
		}
	})

	t.Run("human author without verified gets 100 stewardship while TrustTier stays unverified", func(t *testing.T) {
		c := &okf.Concept{
			ID:        "playbooks/oncall",
			Path:      "playbooks/oncall.md",
			Type:      "Playbook",
			Title:     "Oncall",
			Status:    "stable",
			Generated: &okf.Generated{By: "human:ahormati", At: "2026-06-20T14:00:00Z"},
		}
		b := &okf.Bundle{
			DeclaredVer: "0.3",
			Concepts:    map[string]*okf.Concept{c.ID: c},
		}

		if got := okf.TrustTier(c); got != okf.TrustTierUnverified {
			t.Fatalf("expected TrustTier %q, got %q", okf.TrustTierUnverified, got)
		}

		sc := okf.ComputeScorecard(b)
		if sc.Stewardship != 100 {
			t.Errorf("expected Stewardship=100 for human author, got %d", sc.Stewardship)
		}
	})
}
