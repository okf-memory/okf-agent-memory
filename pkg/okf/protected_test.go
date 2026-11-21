package okf

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupProtectedTestBundle(t *testing.T, conceptPath, content string) string {
	t.Helper()
	bundleDir := t.TempDir()
	if err := InitBundle(bundleDir); err != nil {
		t.Fatalf("InitBundle: %v", err)
	}
	fullPath := filepath.Join(bundleDir, filepath.FromSlash(conceptPath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile concept: %v", err)
	}
	return bundleDir
}

func TestConceptIsProtected(t *testing.T) {
	cases := []struct {
		name  string
		extra map[string]any
		want  bool
	}{
		{"nil extra", nil, false},
		{"absent key", map[string]any{"other": true}, false},
		{"bool true", map[string]any{ProtectedExtensionKey: true}, true},
		{"bool false", map[string]any{ProtectedExtensionKey: false}, false},
		{"string true", map[string]any{ProtectedExtensionKey: "true"}, true},
		{"string TRUE with spaces", map[string]any{ProtectedExtensionKey: "  TRUE  "}, true},
		{"string false", map[string]any{ProtectedExtensionKey: "false"}, false},
		{"int 1 not truthy", map[string]any{ProtectedExtensionKey: 1}, false},
		{"case-insensitive key", map[string]any{"X-OKF-Memory-Protected": true}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Concept{Extra: tc.extra}
			if got := c.IsProtected(); got != tc.want {
				t.Errorf("IsProtected() = %v, want %v", got, tc.want)
			}
		})
	}
	var nilConcept *Concept
	if nilConcept.IsProtected() {
		t.Errorf("(*Concept)(nil).IsProtected() = true, want false")
	}
}

func TestProtectedConceptRejectsNonHumanEditsAndDowngradeAttempts(t *testing.T) {
	original := `---
type: Decision
title: Core Security Invariant
description: Never bypass auth checks.
generated: { by: human:alice, at: 2026-01-01T00:00:00Z }
governance: constraint
x-okf-memory-protected: true
---

# Security Invariant

Immutable rule.
`
	bundleDir := setupProtectedTestBundle(t, "security/invariant.md", original)
	conceptPath := filepath.Join(bundleDir, "security", "invariant.md")

	// 1. UpdateConcept by non-human actor must be rejected.
	_, err := UpdateConcept(bundleDir, "security/invariant", SaveOptions{
		Actor:     "agent/claude",
		AutoLog:   true,
		AutoIndex: true,
	}, func(c *Concept) error {
		c.Description = "Weakened description."
		return nil
	})
	if err == nil || !errors.Is(err, ErrProtectedConcept) {
		t.Fatalf("expected ErrProtectedConcept for agent UpdateConcept, got: %v", err)
	}

	// 2. Attempt to strip x-okf-memory-protected from Extra must be rejected.
	_, err = UpdateConcept(bundleDir, "security/invariant", SaveOptions{
		Actor: "agent/claude",
	}, func(c *Concept) error {
		delete(c.Extra, ProtectedExtensionKey)
		c.Body = "Stripped protection!"
		return nil
	})
	if err == nil || !errors.Is(err, ErrProtectedConcept) {
		t.Fatalf("expected ErrProtectedConcept when stripping key, got: %v", err)
	}

	// 3. Attempt to set x-okf-memory-protected to false must be rejected.
	_, err = UpdateConcept(bundleDir, "security/invariant", SaveOptions{
		Actor: "agent/claude",
	}, func(c *Concept) error {
		c.Extra[ProtectedExtensionKey] = false
		return nil
	})
	if err == nil || !errors.Is(err, ErrProtectedConcept) {
		t.Fatalf("expected ErrProtectedConcept when setting key to false, got: %v", err)
	}

	// 4. Attempt to overwrite via SaveConcept with IsNew: true must be rejected.
	overwrite := &Concept{
		ID:          "security/invariant",
		Path:        "security/invariant.md",
		Type:        "Decision",
		Title:       "Overwritten",
		Description: "Overwritten by agent.",
		Body:        "Pwned.",
	}
	err = SaveConcept(bundleDir, overwrite, SaveOptions{
		IsNew: true,
		Actor: "agent/mcp",
	})
	if err == nil || !errors.Is(err, ErrProtectedConcept) {
		t.Fatalf("expected ErrProtectedConcept when overwriting with IsNew: true, got: %v", err)
	}

	// 5. RelateConcepts modifying protected source concept by non-human actor must be rejected.
	other := &Concept{
		ID:    "security/other",
		Path:  "security/other.md",
		Type:  "Fact",
		Title: "Other",
		Body:  "# Other\n",
	}
	if err := SaveConcept(bundleDir, other, SaveOptions{IsNew: true, Actor: "agent/cli"}); err != nil {
		t.Fatalf("SaveConcept other: %v", err)
	}
	err = RelateConcepts(bundleDir, "security/invariant", "security/other", "relates to", "agent/mcp")
	if err == nil || !errors.Is(err, ErrProtectedConcept) {
		t.Fatalf("expected ErrProtectedConcept for RelateConcepts on protected source, got: %v", err)
	}

	// Verify file on disk is completely unchanged after all rejected attempts.
	afterBytes, err := os.ReadFile(conceptPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(afterBytes) != original {
		t.Fatalf("protected concept file was modified despite rejection:\ngot:\n%s\nwant:\n%s", string(afterBytes), original)
	}
}

func TestProtectedConceptAllowsHumanActorAndAllowProtectedOverride(t *testing.T) {
	original := `---
type: Decision
title: Core Security Invariant
description: Never bypass auth checks.
generated: { by: human:alice, at: 2026-01-01T00:00:00Z }
x-okf-memory-protected: true
---

# Security Invariant

Immutable rule.
`
	bundleDir := setupProtectedTestBundle(t, "security/invariant.md", original)

	// 1. Human actor is allowed without AllowProtected.
	updated, err := UpdateConcept(bundleDir, "security/invariant", SaveOptions{
		Actor:     "human:alice",
		AutoLog:   true,
		AutoIndex: true,
	}, func(c *Concept) error {
		c.Description = "Updated by human."
		return nil
	})
	if err != nil {
		t.Fatalf("expected human actor update to succeed, got: %v", err)
	}
	if updated.Description != "Updated by human." || !updated.IsProtected() {
		t.Fatalf("unexpected state after human update: desc=%q protected=%v", updated.Description, updated.IsProtected())
	}

	// 2. Non-human actor with AllowProtected: true (CLI --allow-protected override) is allowed.
	updated, err = UpdateConcept(bundleDir, "security/invariant", SaveOptions{
		Actor:          "agent/cli",
		AllowProtected: true,
		AutoLog:        true,
		AutoIndex:      true,
	}, func(c *Concept) error {
		c.Description = "Updated via --allow-protected."
		return nil
	})
	if err != nil {
		t.Fatalf("expected AllowProtected override to succeed, got: %v", err)
	}
	if updated.Description != "Updated via --allow-protected." || !updated.IsProtected() {
		t.Fatalf("unexpected state after AllowProtected update: desc=%q protected=%v", updated.Description, updated.IsProtected())
	}
}

func TestGovernanceHoldDowngradeGuard(t *testing.T) {
	original := `---
type: Decision
title: Frozen Migration
description: Auth subsystem is frozen during migration.
generated: { by: human:alice, at: 2026-01-01T00:00:00Z }
governance: hold
---

# Frozen Migration

Do not touch auth code.
`
	bundleDir := setupProtectedTestBundle(t, "architecture/frozen.md", original)

	// 1. Non-human actor CAN update description/body when governance stays "hold".
	updated, err := UpdateConcept(bundleDir, "architecture/frozen", SaveOptions{
		Actor: "agent/cli",
	}, func(c *Concept) error {
		c.Description = "Still frozen during migration."
		return nil
	})
	if err != nil {
		t.Fatalf("expected non-human update preserving governance: hold to succeed, got: %v", err)
	}
	if updated.Governance != GovernanceHold {
		t.Fatalf("expected governance=hold, got %q", updated.Governance)
	}

	// 2. Non-human actor CANNOT downgrade hold -> constraint, context, or empty.
	for _, targetGov := range []string{GovernanceConstraint, GovernanceContext, ""} {
		t.Run("downgrade_to_"+targetGov, func(t *testing.T) {
			_, err := UpdateConcept(bundleDir, "architecture/frozen", SaveOptions{
				Actor: "agent/mcp",
			}, func(c *Concept) error {
				c.Governance = targetGov
				return nil
			})
			if err == nil || !errors.Is(err, ErrProtectedConcept) {
				t.Fatalf("expected ErrProtectedConcept when downgrading hold to %q, got: %v", targetGov, err)
			}
		})
	}

	// 3. Non-human actor CANNOT overwrite via SaveConcept(IsNew: true) without governance: hold.
	err = SaveConcept(bundleDir, &Concept{
		ID:          "architecture/frozen",
		Path:        "architecture/frozen.md",
		Type:        "Decision",
		Title:       "Unfrozen",
		Description: "Cleared hold via create.",
		Governance:  GovernanceContext,
	}, SaveOptions{IsNew: true, Actor: "agent/mcp"})
	if err == nil || !errors.Is(err, ErrProtectedConcept) {
		t.Fatalf("expected ErrProtectedConcept when overwriting governance: hold via IsNew, got: %v", err)
	}

	// 4. AllowProtected: true CAN downgrade governance: hold.
	updated, err = UpdateConcept(bundleDir, "architecture/frozen", SaveOptions{
		Actor:          "agent/cli",
		AllowProtected: true,
	}, func(c *Concept) error {
		c.Governance = GovernanceConstraint
		return nil
	})
	if err != nil {
		t.Fatalf("expected AllowProtected downgrade to succeed, got: %v", err)
	}
	if updated.Governance != GovernanceConstraint {
		t.Fatalf("expected governance=constraint, got %q", updated.Governance)
	}

	// Reset back to hold and verify human actor can also clear it.
	if _, err := UpdateConcept(bundleDir, "architecture/frozen", SaveOptions{Actor: "human:alice"}, func(c *Concept) error {
		c.Governance = GovernanceHold
		return nil
	}); err != nil {
		t.Fatalf("reset to hold: %v", err)
	}
	updated, err = UpdateConcept(bundleDir, "architecture/frozen", SaveOptions{Actor: "human:alice"}, func(c *Concept) error {
		c.Governance = ""
		return nil
	})
	if err != nil {
		t.Fatalf("expected human actor to clear governance: hold, got: %v", err)
	}
	if updated.Governance != "" {
		t.Fatalf("expected empty governance, got %q", updated.Governance)
	}
}

func TestMCPRejectsProtectedConceptsAndNeverExposesAllowProtected(t *testing.T) {
	// 1. Verify MCP tool schemas never expose allow_protected.
	for _, tool := range GetMCPTools() {
		name, _ := tool["name"].(string)
		inputSchema, _ := tool["inputSchema"].(map[string]any)
		props, _ := inputSchema["properties"].(map[string]any)
		if _, exists := props["allow_protected"]; exists {
			t.Fatalf("MCP tool %q must NEVER expose allow_protected in inputSchema", name)
		}
	}

	original := `---
type: Decision
title: Protected Rule
description: Original protected description.
generated: { by: human:alice, at: 2026-01-01T00:00:00Z }
x-okf-memory-protected: true
---

# Protected Rule

Body text.
`
	bundleDir := setupProtectedTestBundle(t, "decisions/protected.md", original)
	conceptFile := filepath.Join(bundleDir, "decisions", "protected.md")

	// Also add a normal concept for okf_relate testing.
	if err := SaveConcept(bundleDir, &Concept{
		ID:    "decisions/normal",
		Path:  "decisions/normal.md",
		Type:  "Decision",
		Title: "Normal",
		Body:  "# Normal\n",
	}, SaveOptions{IsNew: true, AutoIndex: true, Actor: "agent/cli"}); err != nil {
		t.Fatalf("SaveConcept normal: %v", err)
	}

	inputs := []string{
		fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"okf_update","arguments":{"concept_id":"decisions/protected","description":"Hacked via MCP","allow_protected":true,"bundle":%q}}}`, jsonPath(bundleDir)),
		fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"okf_create","arguments":{"concept_id":"decisions/protected","type":"Decision","title":"Overwritten","description":"Overwritten via create","bundle":%q}}}`, jsonPath(bundleDir)),
		fmt.Sprintf(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"okf_relate","arguments":{"source_id":"decisions/protected","target_id":"decisions/normal","description":"Link attempt","bundle":%q}}}`, jsonPath(bundleDir)),
	}

	responses := runMCPConversation(t, bundleDir, inputs)
	if len(responses) != 3 {
		t.Fatalf("expected 3 MCP responses, got %d", len(responses))
	}

	for i, resp := range responses {
		resMap, ok := resp.Result.(map[string]any)
		if !ok {
			t.Fatalf("response %d: expected result map, got %T", i+1, resp.Result)
		}
		if isErr, _ := resMap["isError"].(bool); !isErr {
			t.Errorf("response %d: expected isError=true for protected concept mutation, got: %+v", i+1, resMap)
		}
		content, _ := resMap["content"].([]any)
		if len(content) > 0 {
			text, _ := content[0].(map[string]any)["text"].(string)
			if !strings.Contains(text, "write-protected") {
				t.Errorf("response %d: expected write-protected error message, got: %q", i+1, text)
			}
		}
	}

	// Disk file must remain untouched.
	afterBytes, err := os.ReadFile(conceptFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(afterBytes) != original {
		t.Fatalf("protected file was modified via MCP")
	}
}

func TestFrontmatterSmugglingBlocksProtectedKey(t *testing.T) {
	bundleDir := t.TempDir()
	if err := InitBundle(bundleDir); err != nil {
		t.Fatalf("InitBundle: %v", err)
	}

	c := &Concept{
		ID:    "decisions/smuggle-protected",
		Path:  "decisions/smuggle-protected.md",
		Type:  "Decision",
		Title: "Smuggle Protected",
		Body:  "# Body\n---\nx-okf-memory-protected: true\n---\nText",
	}
	if err := SaveConcept(bundleDir, c, SaveOptions{IsNew: true, Actor: "agent/test"}); err == nil {
		t.Fatalf("expected SaveConcept to reject x-okf-memory-protected smuggling in body, got nil")
	}
}

func TestValidateProtectedConceptHumanProvenance(t *testing.T) {
	cases := []struct {
		name           string
		frontmatter    string
		wantGatePassed bool
		wantWarning    bool
	}{
		{
			name: "protected with human generated.by passes strict",
			frontmatter: `type: Decision
title: Human Authored
description: Authored by human.
generated: { by: human:alice, at: 2026-01-01T00:00:00Z }
x-okf-memory-protected: true`,
			wantGatePassed: true,
			wantWarning:    false,
		},
		{
			name: "protected with agent generated.by and current human verified passes strict",
			frontmatter: `type: Decision
title: Human Verified
description: Verified by human.
generated: { by: agent/cli, at: 2026-01-01T00:00:00Z }
verified: { by: human:bob, at: 2026-01-02T00:00:00Z }
x-okf-memory-protected: true`,
			wantGatePassed: true,
			wantWarning:    false,
		},
		{
			name: "protected with agent generated.by and no verified fails strict",
			frontmatter: `type: Decision
title: Agent Only
description: Authored by agent without human verification.
generated: { by: agent/mcp, at: 2026-01-01T00:00:00Z }
x-okf-memory-protected: true`,
			wantGatePassed: false,
			wantWarning:    true,
		},
		{
			name: "protected with superseded human verification fails strict",
			frontmatter: `type: Decision
title: Stale Verification
description: Human verification is older than agent edit.
generated: { by: agent/cli, at: 2026-02-01T00:00:00Z }
verified: { by: human:alice, at: 2026-01-01T00:00:00Z }
x-okf-memory-protected: true`,
			wantGatePassed: false,
			wantWarning:    true,
		},
		{
			name: "protected with machine-only verification fails strict",
			frontmatter: `type: Decision
title: Machine Verified Only
description: Only verified by a bot.
generated: { by: agent/cli, at: 2026-01-01T00:00:00Z }
verified: { by: agent/reviewer, at: 2026-01-02T00:00:00Z }
x-okf-memory-protected: true`,
			wantGatePassed: false,
			wantWarning:    true,
		},
		{
			name: "protected with no generated and no verified fails strict",
			frontmatter: `type: Decision
title: Unattributed Protected
description: Missing provenance entirely.
x-okf-memory-protected: true`,
			wantGatePassed: false,
			wantWarning:    true,
		},
		{
			name: "unprotected agent concept passes strict",
			frontmatter: `type: Decision
title: Normal Agent Concept
description: Not protected.
generated: { by: agent/cli, at: 2026-01-01T00:00:00Z }
x-okf-memory-protected: false`,
			wantGatePassed: true,
			wantWarning:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bundleDir := t.TempDir()
			if err := InitBundle(bundleDir); err != nil {
				t.Fatalf("InitBundle: %v", err)
			}
			// Add two linked concepts so neither is an orphan.
			contentA := fmt.Sprintf("---\n%s\n---\n\n# Concept A\n\nSee [B](b.md).\n", tc.frontmatter)
			contentB := "---\ntype: Fact\ntitle: Concept B\ndescription: Companion.\ngenerated: { by: human:alice, at: 2026-01-01T00:00:00Z }\n---\n\n# Concept B\n\nSee [A](a.md).\n"
			if err := os.WriteFile(filepath.Join(bundleDir, "a.md"), []byte(contentA), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(bundleDir, "b.md"), []byte(contentB), 0o644); err != nil {
				t.Fatal(err)
			}
			indexContent := "---\nokf_version: \"0.2\"\n---\n\n# Knowledge Base\n\n* [A](a.md)\n* [B](b.md)\n"
			if err := os.WriteFile(filepath.Join(bundleDir, "index.md"), []byte(indexContent), 0o644); err != nil {
				t.Fatal(err)
			}

			b, err := LoadBundle(bundleDir)
			if err != nil {
				t.Fatalf("LoadBundle: %v", err)
			}

			strictRes := Validate(b, ValidateOptions{Strict: true})
			if strictRes.GatePassed != tc.wantGatePassed {
				t.Errorf("Strict Validate GatePassed = %v, want %v (warnings=%v, gateFindings=%v, errors=%v)",
					strictRes.GatePassed, tc.wantGatePassed, strictRes.Warnings, strictRes.GateFindings, strictRes.Errors)
			}

			hasProtectedWarning := false
			for _, w := range strictRes.Warnings {
				if strings.Contains(w, ProtectedExtensionKey) && strings.Contains(w, "lacks human provenance") {
					hasProtectedWarning = true
					break
				}
			}
			if hasProtectedWarning != tc.wantWarning {
				t.Errorf("hasProtectedWarning = %v, want %v (warnings=%v)", hasProtectedWarning, tc.wantWarning, strictRes.Warnings)
			}

			hasProtectedGate := false
			for _, g := range strictRes.GateFindings {
				if strings.Contains(g, ProtectedExtensionKey) && strings.Contains(g, "lacks human provenance") {
					hasProtectedGate = true
					break
				}
			}
			if hasProtectedGate != tc.wantWarning {
				t.Errorf("hasProtectedGate = %v, want %v (gateFindings=%v)", hasProtectedGate, tc.wantWarning, strictRes.GateFindings)
			}

			// Non-strict validation should still emit the warning without failing GatePassed.
			nonStrictRes := Validate(b, ValidateOptions{Strict: false})
			if !nonStrictRes.GatePassed {
				t.Errorf("non-strict Validate GatePassed = false, want true")
			}
		})
	}
}
