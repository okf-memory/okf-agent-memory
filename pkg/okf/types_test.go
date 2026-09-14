package okf_test

import (
	"testing"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

func TestIsValidActor(t *testing.T) {
	tests := []struct {
		name  string
		actor string
		want  bool
	}{
		{"empty string", "", false},
		{"valid prefix:id", "agent:opencode/deepseek-v4-flash", true},
		{"valid team", "team:ga4-docs", true},
		{"valid bot", "bot:linter-v2", true},
		{"valid human", "human:alice", true},
		{"valid standard format", "openai/gpt-4o", true},
		{"valid actor without prefix", "agent/test-v1", true},
		{"invalid, no prefix and no slash", "human", false},
		{"invalid, space in name", "agent: space in name", false},
		{"invalid, multiple slashes", "too/many/slashes/here", false},
		{"invalid, starts with number prefix", "1invalid:prefix", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := okf.IsValidActor(tt.actor); got != tt.want {
				t.Errorf("IsValidActor(%q) = %v, want %v", tt.actor, got, tt.want)
			}
		})
	}
}

func TestGetNonStandardPrefix(t *testing.T) {
	if got := okf.GetNonStandardPrefix("anything"); got != "" {
		t.Errorf("GetNonStandardPrefix() = %v, want empty string", got)
	}
}
