package okf_test

import (
	"testing"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

func TestIsHumanIdentity(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"human", true},
		{"human:lead", true},
		{"human/lead", true},
		{"Human:Lead", true},
		{"HUMAN/lead", true},
		{"  human:lead  ", true},
		{"human:", true},

		{"", false},
		{"agent/claude", false},
		{"agent/human", false},
		{"humanoid-bot", false},
		{"human-reviewer", false},
		{"humans:lead", false},
		{"not-human:lead", false},
		{"team:platform", false},
		{"team:human", false},
		{"team:human-reviewers", false},
		{"process:human-check", false},
	}
	for _, tt := range tests {
		if got := okf.IsHumanIdentity(tt.id); got != tt.want {
			t.Errorf("IsHumanIdentity(%q) = %v, want %v", tt.id, got, tt.want)
		}
	}
}
