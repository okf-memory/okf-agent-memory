package okf_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

func TestParseAndSerializeConcept(t *testing.T) {
	raw := `---
type: Decision
title: Test Decision
description: A test decision concept.
tags: [test, go]
generated: { by: agent/test-v1, at: 2026-08-27T12:00:00Z }
sources:
  - id: src1
    resource: https://example.com/spec
    title: Example Spec
status: stable
custom_field: custom_value
---

# Decision

This is the body text with a footnote.[^src1]
`

	c, err := okf.ParseConcept("decisions/test.md", raw)
	if err != nil {
		t.Fatalf("ParseConcept failed: %v", err)
	}

	if c.Type != "Decision" {
		t.Errorf("Expected Type Decision, got %s", c.Type)
	}
	if c.Title != "Test Decision" {
		t.Errorf("Expected Title 'Test Decision', got '%s'", c.Title)
	}
	if len(c.Tags) != 2 || c.Tags[0] != "test" || c.Tags[1] != "go" {
		t.Errorf("Unexpected tags: %v", c.Tags)
	}
	if c.Generated == nil || c.Generated.By != "agent/test-v1" {
		t.Errorf("Unexpected generated: %+v", c.Generated)
	}
	if len(c.Sources) != 1 || c.Sources[0].ID != "src1" {
		t.Errorf("Unexpected sources: %+v", c.Sources)
	}
	if c.Extra["custom_field"] != "custom_value" {
		t.Errorf("Expected custom_field 'custom_value', got '%v'", c.Extra["custom_field"])
	}

	serialized := okf.SerializeConcept(c)
	if !strings.Contains(serialized, "custom_field: custom_value") {
		t.Errorf("Serialized output did not preserve custom_field: %s", serialized)
	}
	if !strings.Contains(serialized, "type: Decision") {
		t.Errorf("Serialized output missing type: %s", serialized)
	}
}

func TestValidateKnowledgeCorpus(t *testing.T) {
	bundlePath := "../../knowledge"
	if _, err := os.Stat(bundlePath); os.IsNotExist(err) {
		bundlePath = "knowledge"
	}

	b, err := okf.LoadBundle(bundlePath)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}

	if len(b.Concepts) == 0 {
		t.Fatalf("Expected at least 1 concept in knowledge bundle, got 0")
	}

	res := okf.Validate(b, okf.ValidateOptions{Strict: true, Drift: true})
	if !res.IsConformant {
		t.Errorf("Bundle is not conformant: errors = %v", res.Errors)
	}
	if !res.GatePassed {
		t.Errorf("Bundle failed producer gate: findings = %v, broken = %v, orphans = %v", res.GateFindings, res.BrokenLinks, res.Orphans)
	}
	if len(res.Errors) != 0 {
		t.Errorf("Expected 0 errors, got %d: %v", len(res.Errors), res.Errors)
	}
}

func TestSearchEngine(t *testing.T) {
	bundlePath := "../../knowledge"
	if _, err := os.Stat(bundlePath); os.IsNotExist(err) {
		bundlePath = "knowledge"
	}

	b, err := okf.LoadBundle(bundlePath)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}

	results := b.Search("architecture layer", 5)
	if len(results) == 0 {
		t.Fatalf("Expected search results for 'architecture layer', got 0")
	}

	top := results[0]
	if !strings.Contains(top.ConceptID, "layers") && !strings.Contains(top.ConceptID, "architecture") {
		t.Errorf("Expected top result to be architecture-related, got %s (score: %f)", top.ConceptID, top.Score)
	}
}

func TestMutateAndAutoBookkeeping(t *testing.T) {
	tmpDir := t.TempDir()

	if err := okf.InitBundle(tmpDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	c := &okf.Concept{
		Path:        "test/example.md",
		Type:        "Fact",
		Title:       "Example Fact",
		Description: "An example fact for testing.",
		Body:        "# Fact\n\nTesting fact content.",
	}

	if err := okf.SaveConcept(tmpDir, c, true, true, true, "agent/unit-test"); err != nil {
		t.Fatalf("SaveConcept failed: %v", err)
	}

	// Verify concept file was written
	conceptFile := filepath.Join(tmpDir, "test", "example.md")
	if _, err := os.Stat(conceptFile); os.IsNotExist(err) {
		t.Fatalf("Concept file not created: %s", conceptFile)
	}

	// Verify log.md updated
	logContent, err := os.ReadFile(filepath.Join(tmpDir, "log.md"))
	if err != nil {
		t.Fatalf("Failed to read log.md: %v", err)
	}
	if !strings.Contains(string(logContent), "Documented concept `test/example.md`") {
		t.Errorf("log.md does not contain creation entry: %s", string(logContent))
	}

	// Verify sub-index updated
	indexContent, err := os.ReadFile(filepath.Join(tmpDir, "test", "index.md"))
	if err != nil {
		t.Fatalf("Failed to read test/index.md: %v", err)
	}
	if !strings.Contains(string(indexContent), "[Example Fact](example.md)") {
		t.Errorf("test/index.md does not contain listing: %s", string(indexContent))
	}
}

func TestRelateConcepts(t *testing.T) {
	tmpDir := t.TempDir()

	if err := okf.InitBundle(tmpDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	c1 := &okf.Concept{
		Path:        "auth/oauth.md",
		Type:        "Decision",
		Title:       "OAuth2 Flow",
		Description: "OAuth2 implementation.",
		Body:        "# OAuth2\n\nOAuth2 details.",
	}
	c2 := &okf.Concept{
		Path:        "services/gateway.md",
		Type:        "Architecture",
		Title:       "API Gateway",
		Description: "Gateway routing.",
		Body:        "# API Gateway\n\nGateway details.",
	}

	if err := okf.SaveConcept(tmpDir, c1, true, true, true, "agent/test"); err != nil {
		t.Fatalf("Save c1 failed: %v", err)
	}
	if err := okf.SaveConcept(tmpDir, c2, true, true, true, "agent/test"); err != nil {
		t.Fatalf("Save c2 failed: %v", err)
	}

	if err := okf.RelateConcepts(tmpDir, "services/gateway", "auth/oauth", "verifies incoming tokens", "agent/test"); err != nil {
		t.Fatalf("RelateConcepts failed: %v", err)
	}

	// Verify gateway.md now has link to ../auth/oauth.md
	data, err := os.ReadFile(filepath.Join(tmpDir, "services", "gateway.md"))
	if err != nil {
		t.Fatalf("Failed to read gateway.md: %v", err)
	}
	if !strings.Contains(string(data), "[OAuth2 Flow](../auth/oauth.md)") {
		t.Errorf("gateway.md missing relative link: %s", string(data))
	}
}

func TestBootstrap(t *testing.T) {
	tmpDir := t.TempDir()

	opts := okf.DefaultBootstrapOptions()
	opts.ProjectName = "custom-service"
	if err := okf.Bootstrap(tmpDir, opts); err != nil {
		t.Fatalf("Bootstrap failed: %v", err)
	}

	// Verify knowledge bundle exists
	indexData, err := os.ReadFile(filepath.Join(tmpDir, "knowledge", "index.md"))
	if err != nil {
		t.Errorf("knowledge/index.md was not created: %v", err)
	} else if !strings.Contains(string(indexData), "# custom-service Knowledge Base") {
		t.Errorf("knowledge/index.md missing custom project name: %s", string(indexData))
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "knowledge", "log.md")); os.IsNotExist(err) {
		t.Errorf("knowledge/log.md was not created")
	}

	// Verify skill files exist and are non-empty
	expectedSkillFiles := []string{
		"SKILL.md",
		"discovery.md",
		"remember.md",
		"update.md",
		"relationships.md",
		"examples.md",
	}
	for _, sf := range expectedSkillFiles {
		sfPath := filepath.Join(tmpDir, ".agents", "skills", "okf-memory", sf)
		st, err := os.Stat(sfPath)
		if os.IsNotExist(err) {
			t.Errorf(".agents/skills/okf-memory/%s was not created", sf)
		} else if err == nil && st.Size() == 0 {
			t.Errorf(".agents/skills/okf-memory/%s is empty (0 bytes)", sf)
		}
	}

	// Verify AGENTS.md exists and has customized project name
	agentsData, err := os.ReadFile(filepath.Join(tmpDir, "AGENTS.md"))
	if err != nil {
		t.Errorf("AGENTS.md was not created: %v", err)
	} else {
		content := string(agentsData)
		if !strings.Contains(content, "Instructions for AI Agents in `custom-service`") {
			t.Errorf("AGENTS.md missing custom project name in title: %s", content)
		}
		if !strings.Contains(content, "Welcome to the **custom-service** repository") {
			t.Errorf("AGENTS.md missing custom project name in welcome: %s", content)
		}
		if strings.Contains(content, "{{PROJECT_NAME}}") {
			t.Errorf("AGENTS.md still contains raw {{PROJECT_NAME}} placeholder")
		}
	}

	// Verify Makefile exists
	if _, err := os.Stat(filepath.Join(tmpDir, "Makefile")); os.IsNotExist(err) {
		t.Errorf("Makefile was not created")
	}
}

func TestBootstrapExistingAGENTS_SmartAppend(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Create pre-existing AGENTS.md with user-defined coding rules
	userOriginalContent := "# Custom Team Rules\n\n1. Always run npm test before committing.\n2. Use React 19.\n"
	agentsMDPath := filepath.Join(tmpDir, "AGENTS.md")
	if err := os.WriteFile(agentsMDPath, []byte(userOriginalContent), 0o644); err != nil {
		t.Fatalf("Failed to write initial custom AGENTS.md: %v", err)
	}

	// 2. Run bootstrap
	opts := okf.DefaultBootstrapOptions()
	opts.ProjectName = "my-existing-app"
	if err := okf.Bootstrap(tmpDir, opts); err != nil {
		t.Fatalf("Bootstrap failed on existing repo: %v", err)
	}

	// 3. Verify user rules are preserved at top AND OKF block is appended at bottom
	data, err := os.ReadFile(agentsMDPath)
	if err != nil {
		t.Fatalf("Failed reading AGENTS.md: %v", err)
	}
	content := string(data)

	if !strings.HasPrefix(content, "# Custom Team Rules\n\n1. Always run npm test before committing.") {
		t.Errorf("Existing user rules were overwritten or destroyed: %s", content)
	}
	if !strings.Contains(content, "<!-- BEGIN OKF AGENT MEMORY -->") {
		t.Errorf("OKF memory block marker was not appended: %s", content)
	}
	if !strings.Contains(content, "https://github.com/okf-memory/okf-agent-memory") {
		t.Errorf("Distribution repository link is missing in appended block: %s", content)
	}

	// 4. Test Idempotency: Running bootstrap again must NOT double-append
	if err := okf.Bootstrap(tmpDir, opts); err != nil {
		t.Fatalf("Second bootstrap failed: %v", err)
	}

	dataSecond, _ := os.ReadFile(agentsMDPath)
	contentSecond := string(dataSecond)
	firstCount := strings.Count(contentSecond, "<!-- BEGIN OKF AGENT MEMORY -->")
	if firstCount != 1 {
		t.Errorf("Idempotency check failed: expected exactly 1 OKF block, found %d", firstCount)
	}
}
