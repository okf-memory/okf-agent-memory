package okf

import (
	"testing"
	"time"
)

func TestValidateStaleWithin(t *testing.T) {
	// Base today date in validator is time.Now().UTC()
	// Let's create concepts with stale_after in 5 days, 20 days, and in the past
	now := time.Now().UTC()
	past := now.Add(-48 * time.Hour).Format("2006-01-02")
	soon := now.Add(5 * 24 * time.Hour).Format("2006-01-02")
	far := now.Add(40 * 24 * time.Hour).Format("2006-01-02")

	b := &Bundle{
		DeclaredVer: "0.2",
		Concepts: map[string]*Concept{
			"decisions/soon": {
				ID:          "decisions/soon",
				Path:        "decisions/soon.md",
				Type:        "Decision",
				Title:       "Soon Stale",
				Description: "Will expire in 5 days",
				StaleAfter:  soon,
				Body:        "Body content",
			},
			"decisions/far": {
				ID:          "decisions/far",
				Path:        "decisions/far.md",
				Type:        "Decision",
				Title:       "Far Stale",
				Description: "Will expire in 40 days",
				StaleAfter:  far,
				Body:        "Body content",
			},
			"decisions/past": {
				ID:          "decisions/past",
				Path:        "decisions/past.md",
				Type:        "Decision",
				Title:       "Already Stale",
				Description: "Expired in the past",
				StaleAfter:  past,
				Body:        "Body content",
			},
		},
		Indexes: make(map[string]string),
	}

	t.Run("Standard validate with stale=false does not fail gate", func(t *testing.T) {
		res := Validate(b, ValidateOptions{})
		if !res.GatePassed {
			t.Errorf("expected gate to pass when stale check is disabled")
		}
	})

	t.Run("Validate with StaleWithin=14d fails gate on past and soon", func(t *testing.T) {
		res := Validate(b, ValidateOptions{
			StaleWithin: 14 * 24 * time.Hour,
		})
		if res.GatePassed {
			t.Errorf("expected gate to fail when concepts expire within 14d")
		}
		if res.StaleCount != 2 {
			t.Errorf("expected StaleCount=2 (past + soon), got %d", res.StaleCount)
		}
	})
}
