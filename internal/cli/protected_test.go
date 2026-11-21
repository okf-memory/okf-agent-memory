package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

func TestCLIUpdateProtectedConcept(t *testing.T) {
	if mode := os.Getenv("OKF_TEST_PROTECTED_UPDATE"); mode != "" {
		bundle := os.Getenv("OKF_TEST_PROTECTED_BUNDLE")
		switch mode {
		case "blocked":
			cmdUpdate([]string{"locked", bundle, "--desc", "Unauthorized agent edit"})
		}
		return
	}

	bundle := mutationTestBundle(t)
	original := `---
type: Decision
title: Locked Rule
description: Original locked description.
generated: { by: human:alice, at: 2026-01-01T00:00:00Z }
x-okf-memory-protected: true
---

# Locked Rule

Immutable content.
`
	lockedPath := filepath.Join(bundle, "locked.md")
	if err := os.WriteFile(lockedPath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	// 1. Without --allow-protected (default actor agent/cli), update must exit non-zero and leave file unchanged.
	cmd := exec.Command(os.Args[0], "-test.run=^TestCLIUpdateProtectedConcept$")
	cmd.Env = append(os.Environ(), "OKF_TEST_PROTECTED_UPDATE=blocked", "OKF_TEST_PROTECTED_BUNDLE="+bundle)
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "write-protected") {
		t.Fatalf("expected write-protected error without --allow-protected, got err=%v output=%s", err, string(output))
	}
	afterBlocked, err := os.ReadFile(lockedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterBlocked) != original {
		t.Fatalf("protected file was modified despite rejection")
	}

	// 2. With --allow-protected, update succeeds and preserves x-okf-memory-protected: true.
	cmdUpdate([]string{"locked", bundle, "--desc", "Updated with override", "--allow-protected"})
	b, err := okf.LoadBundle(bundle)
	if err != nil {
		t.Fatal(err)
	}
	c := b.Concepts["locked"]
	if c == nil {
		t.Fatalf("concept 'locked' missing after --allow-protected update")
	}
	if c.Description != "Updated with override" {
		t.Errorf("Description = %q, want %q", c.Description, "Updated with override")
	}
	if !c.IsProtected() {
		t.Errorf("expected x-okf-memory-protected: true to be preserved after --allow-protected update")
	}

	// 3. With --actor human:alice (without --allow-protected), update succeeds.
	cmdUpdate([]string{"locked", bundle, "--desc", "Updated by human", "--actor", "human:alice"})
	b, err = okf.LoadBundle(bundle)
	if err != nil {
		t.Fatal(err)
	}
	c = b.Concepts["locked"]
	if c.Description != "Updated by human" || !c.IsProtected() {
		t.Errorf("unexpected concept after human update: desc=%q protected=%v", c.Description, c.IsProtected())
	}
}
