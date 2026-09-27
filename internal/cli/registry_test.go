package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCommandRegistryLookup(t *testing.T) {
	// Standard commands must all be registered
	expectedCommands := []string{
		"validate",
		"search",
		"show",
		"create",
		"update",
		"relate",
		"init",
		"bootstrap",
		"agents",
		"mcp",
		"hub",
	}

	for _, name := range expectedCommands {
		cmd, found := FindCommand(name)
		if !found {
			t.Errorf("expected command %q to be registered, but was not found", name)
			continue
		}
		if cmd.Name != name {
			t.Errorf("expected command Name %q, got %q", name, cmd.Name)
		}
		if cmd.Run == nil {
			t.Errorf("expected command %q to have non-nil Run func", name)
		}
		if cmd.PrintUsage == nil {
			t.Errorf("expected command %q to have non-nil PrintUsage func", name)
		}
	}

	// Unknown command
	if _, found := FindCommand("non_existent_cmd"); found {
		t.Errorf("expected non_existent_cmd to not be found")
	}
}

func TestPrintUsageListsAllCommands(t *testing.T) {
	var buf bytes.Buffer
	renderUsageTo(&buf)
	output := buf.String()

	requiredSubstrings := []string{
		"validate",
		"search",
		"show",
		"create",
		"update",
		"relate",
		"init",
		"bootstrap",
		"agents",
		"mcp",
		"hub",
		"--filter",
		"--stale-within",
	}

	for _, sub := range requiredSubstrings {
		if !strings.Contains(output, sub) {
			t.Errorf("printUsage output missing substring %q", sub)
		}
	}
}
