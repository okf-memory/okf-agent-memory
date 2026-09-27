package main

import (
	"testing"

	"github.com/okf-memory/okf-agent-memory/internal/cli"
)

func TestMainDelegatesToCLI(t *testing.T) {
	code := cli.Execute([]string{"version"}, cli.BuildInfo{
		Version: "test-ver",
		Commit:  "abcdef0",
		Date:    "2026-09-25",
	})
	if code != 0 {
		t.Fatalf("expected cli.Execute with version to return 0, got %d", code)
	}
}
