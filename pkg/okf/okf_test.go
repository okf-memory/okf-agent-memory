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

func TestSearchSupportsUnicodeTerms(t *testing.T) {
	b := &okf.Bundle{
		Concepts: map[string]*okf.Concept{
			"research/tensor": {
				ID:          "research/tensor",
				Title:       "Тензорная декомпозиция",
				Description: "Методы восстановления динамических систем.",
			},
		},
		Graph:        map[string][]string{},
		InboundGraph: map[string][]string{},
	}

	results := b.Search("тензорная декомпозиция", 5)
	if len(results) != 1 || results[0].ConceptID != "research/tensor" {
		t.Fatalf("Search() = %#v, want research/tensor", results)
	}
}

func TestSearchUsesConceptIDToBreakScoreTies(t *testing.T) {
	b := &okf.Bundle{
		Concepts: map[string]*okf.Concept{
			"zeta":  {ID: "zeta", Title: "Shared title"},
			"alpha": {ID: "alpha", Title: "Shared title"},
		},
		Graph:        map[string][]string{},
		InboundGraph: map[string][]string{},
	}

	results := b.Search("shared", 1)
	if len(results) != 1 || results[0].ConceptID != "alpha" {
		t.Fatalf("Search() top result = %#v, want alpha", results)
	}
}

func TestSearchTreatsSingleRuneTermsConsistently(t *testing.T) {
	b := &okf.Bundle{
		Concepts: map[string]*okf.Concept{
			"latin":    {ID: "latin", Title: "A"},
			"cyrillic": {ID: "cyrillic", Title: "Я"},
		},
		Graph:        map[string][]string{},
		InboundGraph: map[string][]string{},
	}

	if got := b.Search("a", 1); len(got) != 1 || got[0].ConceptID != "latin" {
		t.Fatalf("Search(\"a\") = %#v, want latin", got)
	}
	if got := b.Search("я", 1); len(got) != 1 || got[0].ConceptID != "cyrillic" {
		t.Fatalf("Search(\"я\") = %#v, want cyrillic", got)
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

func TestValidateTrustOrdering(t *testing.T) {
	b := &okf.Bundle{
		DeclaredVer: "0.2",
		Concepts: map[string]*okf.Concept{
			"test/concept": {
				ID:   "test/concept",
				Path: "test/concept.md",
				Type: "Decision",
				Generated: &okf.Generated{
					By: "agent/cli",
					At: "2026-09-05T12:00:00Z",
				},
				Verified: []okf.Verified{
					{
						By: "human:reviewer@domain.dev",
						At: "2026-08-10T12:00:00Z", // Predates generated.at
					},
				},
			},
		},
		BrokenLinks: []okf.BrokenLink{},
		Orphans:     []string{},
	}

	// Without --strict: warnings issued, but gate passed
	res := okf.Validate(b, okf.ValidateOptions{Strict: false})
	if !res.IsConformant {
		t.Errorf("Expected bundle to be conformant")
	}
	if !res.GatePassed {
		t.Errorf("Expected gate to pass without --strict")
	}
	if len(res.GateFindings) != 1 {
		t.Errorf("Expected 1 gate finding for superseded verification, got %d", len(res.GateFindings))
	}

	// With --strict: producer gate must fail
	resStrict := okf.Validate(b, okf.ValidateOptions{Strict: true})
	if !resStrict.IsConformant {
		t.Errorf("Expected bundle to still be syntactically conformant")
	}
	if resStrict.GatePassed {
		t.Errorf("Expected gate to fail under --strict due to superseded verification")
	}

	// Now fix verification date: verified after generated
	b.Concepts["test/concept"].Verified[0].At = "2026-09-05T14:00:00Z"
	resFixed := okf.Validate(b, okf.ValidateOptions{Strict: true})
	if !resFixed.GatePassed {
		t.Errorf("Expected gate to pass under --strict when verification is up-to-date, got findings: %v", resFixed.GateFindings)
	}
}

func TestValidateStaleGating(t *testing.T) {
	b := &okf.Bundle{
		DeclaredVer: "0.2",
		Concepts: map[string]*okf.Concept{
			"test/concept": {
				ID:         "test/concept",
				Path:       "test/concept.md",
				Type:       "Decision",
				StaleAfter: "2020-01-01", // Past date
			},
		},
		BrokenLinks: []okf.BrokenLink{},
		Orphans:     []string{},
	}

	// Under --strict only (without --stale): StaleCount is reported, but gate passes
	resStrict := okf.Validate(b, okf.ValidateOptions{Strict: true, Stale: false})
	if resStrict.StaleCount != 1 {
		t.Errorf("Expected StaleCount to be 1, got %d", resStrict.StaleCount)
	}
	if !resStrict.GatePassed {
		t.Errorf("Expected gate to pass under --strict alone when concept is stale")
	}

	// With --stale: gate must fail
	resStale := okf.Validate(b, okf.ValidateOptions{Strict: false, Stale: true})
	if resStale.StaleCount != 1 {
		t.Errorf("Expected StaleCount to be 1, got %d", resStale.StaleCount)
	}
	if resStale.GatePassed {
		t.Errorf("Expected gate to fail when Stale: true and StaleCount > 0")
	}
}

func TestValidateLegacyV01Checks(t *testing.T) {
	// 1. Concept with v0.1 legacy timestamp in frontmatter and # Citations heading in body
	legacyRaw := `---
type: Decision
timestamp: 2024-01-01
---
# Legacy Decision

Here is some content.

# Citations
- Source 1
`
	cLegacy, err := okf.ParseConcept("legacy/decision.md", legacyRaw)
	if err != nil {
		t.Fatalf("ParseConcept failed: %v", err)
	}

	b := &okf.Bundle{
		DeclaredVer: "0.2",
		Concepts: map[string]*okf.Concept{
			"legacy/decision": cLegacy,
		},
		BrokenLinks: []okf.BrokenLink{},
		Orphans:     []string{},
	}

	res := okf.Validate(b, okf.ValidateOptions{Strict: true})
	if res.GatePassed {
		t.Errorf("Expected bundle with v0.1 legacy constructs to fail under --strict")
	}

	foundTimestamp := false
	foundCitations := false
	for _, f := range res.GateFindings {
		if strings.Contains(f, "legacy 'timestamp'") {
			foundTimestamp = true
		}
		if strings.Contains(f, "legacy '# Citations'") {
			foundCitations = true
		}
	}
	if !foundTimestamp {
		t.Errorf("Expected gate finding for legacy 'timestamp', got: %v", res.GateFindings)
	}
	if !foundCitations {
		t.Errorf("Expected gate finding for legacy '# Citations', got: %v", res.GateFindings)
	}

	// 2. False-positive immunity: timestamp in code block and # Citations inside markdown fence
	safeRaw := `---
type: Decision
title: Safe Decision
generated: { by: agent/test, at: 2026-09-01T12:00:00Z }
---
# Safe Decision

Here is an example code block with timestamp:
` + "```yaml" + `
timestamp: 2024-01-01
# Citations
` + "```" + `

And mention timestamp: in regular prose.
`
	cSafe, err := okf.ParseConcept("safe/decision.md", safeRaw)
	if err != nil {
		t.Fatalf("ParseConcept failed: %v", err)
	}

	bSafe := &okf.Bundle{
		DeclaredVer: "0.2",
		Concepts: map[string]*okf.Concept{
			"safe/decision": cSafe,
		},
		BrokenLinks: []okf.BrokenLink{},
		Orphans:     []string{},
	}

	resSafe := okf.Validate(bSafe, okf.ValidateOptions{Strict: true})
	if !resSafe.GatePassed {
		t.Errorf("Expected concept with timestamp/citations in code blocks to pass gate, got: %v", resSafe.GateFindings)
	}
}

func TestResolveLinkCrossPlatform(t *testing.T) {
	b := &okf.Bundle{}

	tests := []struct {
		name     string
		source   string
		href     string
		expected string
	}{
		{
			name:     "same directory relative link",
			source:   "decisions/a.md",
			href:     "b.md",
			expected: "decisions/b",
		},
		{
			name:     "same directory relative link with dot slash",
			source:   "decisions/a.md",
			href:     "./b.md",
			expected: "decisions/b",
		},
		{
			name:     "cross directory relative link",
			source:   "decisions/a.md",
			href:     "../architecture/x.md",
			expected: "architecture/x",
		},
		{
			name:     "root relative link with leading slash",
			source:   "decisions/a.md",
			href:     "/architecture/x.md",
			expected: "architecture/x",
		},
		{
			name:     "root level source concept to subfolder",
			source:   "overview.md",
			href:     "decisions/a.md",
			expected: "decisions/a",
		},
		{
			name:     "root level source concept to sibling",
			source:   "overview.md",
			href:     "faq.md",
			expected: "faq",
		},
		{
			name:     "link with anchor fragment",
			source:   "decisions/a.md",
			href:     "b.md#section-heading",
			expected: "decisions/b",
		},
		{
			name:     "link with query and anchor",
			source:   "decisions/a.md",
			href:     "b.md?view=diff#anchor",
			expected: "decisions/b",
		},
		{
			name:     "windows backslash in source path",
			source:   "decisions\\a.md",
			href:     "b.md",
			expected: "decisions/b",
		},
		{
			name:     "windows backslash in href",
			source:   "decisions/a.md",
			href:     "..\\architecture\\x.md",
			expected: "architecture/x",
		},
		{
			name:     "traversal attempt in href stays isolated string",
			source:   "decisions/a.md",
			href:     "../../../../etc/passwd.md",
			expected: "../../../etc/passwd",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := b.ResolveLink(tc.source, tc.href)
			if got != tc.expected {
				t.Errorf("ResolveLink(%q, %q) = %q, expected %q", tc.source, tc.href, got, tc.expected)
			}
		})
	}
}

func TestValidateBundleRelativeLinksCrossPlatform(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Create decisions/a.md linking to sibling b.md
	rawA := `---
type: Decision
title: Decision A
description: First decision.
generated: { by: agent/test, at: 2026-09-08T08:00:00Z }
---
# Decision A

See [Decision B](b.md): dependency.
`
	// 2. Create decisions/b.md
	rawB := `---
type: Decision
title: Decision B
description: Second decision.
generated: { by: agent/test, at: 2026-09-08T08:00:00Z }
---
# Decision B

Details about B.
`
	decisionsDir := filepath.Join(tmpDir, "decisions")
	if err := os.MkdirAll(decisionsDir, 0o755); err != nil {
		t.Fatalf("Failed to create decisions dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(decisionsDir, "a.md"), []byte(rawA), 0o644); err != nil {
		t.Fatalf("Failed to write a.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(decisionsDir, "b.md"), []byte(rawB), 0o644); err != nil {
		t.Fatalf("Failed to write b.md: %v", err)
	}
	rootIndex := `---
okf_version: "0.2"
---
# Knowledge Base
`
	if err := os.WriteFile(filepath.Join(tmpDir, "index.md"), []byte(rootIndex), 0o644); err != nil {
		t.Fatalf("Failed to write index.md: %v", err)
	}

	bundle, err := okf.LoadBundle(tmpDir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}

	res := okf.Validate(bundle, okf.ValidateOptions{Strict: true})
	if len(res.BrokenLinks) != 0 {
		t.Errorf("Expected 0 broken links, got %d: %+v", len(res.BrokenLinks), res.BrokenLinks)
	}
	if len(res.Orphans) != 0 {
		t.Errorf("Expected 0 orphans, got %d: %+v", len(res.Orphans), res.Orphans)
	}
	if !res.GatePassed {
		t.Errorf("Expected gate to pass, got findings: %v", res.GateFindings)
	}
}

func TestActorOpenFamilyAndStrictValidation(t *testing.T) {
	// 1. Valid actors according to OKF v0.2 §7 open family
	validActors := []string{
		"agent:opencode/deepseek-v4-flash",
		"team:ga4-docs",
		"bot:linter-v2",
		"service:sync-daemon",
		"human:alice",
		"process:ci-runner",
		"openai/gpt-4o",
		"anthropic/claude-3-opus",
		"agent/test-v1",
	}

	for _, a := range validActors {
		if !okf.IsValidActor(a) {
			t.Errorf("Expected %q to be a valid actor", a)
		}
	}

	// 2. Invalid actors
	invalidActors := []string{
		"",
		"human",
		"agent",
		"agent: space in name",
		"too/many/slashes/here",
		"1invalid:prefix-must-start-with-letter",
	}

	for _, a := range invalidActors {
		if okf.IsValidActor(a) {
			t.Errorf("Expected %q to be invalid actor", a)
		}
	}

	// 3. Strict validation of a bundle using agent: prefix produces 0 warnings and passes gate
	raw := `---
type: Decision
title: Open Actor Decision
description: Demonstrates open actor prefix family.
generated: { by: agent:opencode/deepseek-v4-flash, at: 2026-09-04T12:40:00Z }
verified:
  - by: team:ga4-docs
    at: 2026-09-05T12:00:00Z
sources:
  - resource: https://example.com/spec
    author: process:ci-bot
---
# Open Actor Decision

Prose content.
`
	c, err := okf.ParseConcept("decisions/open-actor.md", raw)
	if err != nil {
		t.Fatalf("ParseConcept failed: %v", err)
	}

	bundle := &okf.Bundle{
		DeclaredVer: "0.2",
		Concepts: map[string]*okf.Concept{
			"decisions/open-actor": c,
		},
		BrokenLinks: []okf.BrokenLink{},
		Orphans:     []string{},
	}

	res := okf.Validate(bundle, okf.ValidateOptions{Strict: true})
	if len(res.Warnings) != 0 {
		t.Errorf("Expected 0 warnings for spec-valid open actor prefixes, got %d: %v", len(res.Warnings), res.Warnings)
	}
	if !res.GatePassed {
		t.Errorf("Expected gate to pass, got findings: %v", res.GateFindings)
	}
}

func TestLoadAndValidateDotDirectoryBundle(t *testing.T) {
	tmpDir := t.TempDir()
	dotBundleDir := filepath.Join(tmpDir, ".okf")

	if err := os.MkdirAll(filepath.Join(dotBundleDir, "decisions"), 0o755); err != nil {
		t.Fatalf("Failed to create decisions dir: %v", err)
	}
	// Create hidden subdir that should be skipped
	if err := os.MkdirAll(filepath.Join(dotBundleDir, ".git"), 0o755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dotBundleDir, ".git", "HEAD.md"), []byte("ref: refs/heads/main"), 0o644); err != nil {
		t.Fatalf("Failed to write .git file: %v", err)
	}
	// Create hidden file that should be skipped
	if err := os.WriteFile(filepath.Join(dotBundleDir, ".ignored.md"), []byte("ignored"), 0o644); err != nil {
		t.Fatalf("Failed to write .ignored.md: %v", err)
	}

	rootIndex := `---
okf_version: "0.2"
---
# Dot Directory Knowledge Base
`
	if err := os.WriteFile(filepath.Join(dotBundleDir, "index.md"), []byte(rootIndex), 0o644); err != nil {
		t.Fatalf("Failed to write index.md: %v", err)
	}

	rawA := `---
type: Decision
title: Decision A
description: Decision A in dot bundle.
generated: { by: agent/test, at: 2026-09-08T08:00:00Z }
---
# Decision A

Content of A.
`
	if err := os.WriteFile(filepath.Join(dotBundleDir, "decisions", "a.md"), []byte(rawA), 0o644); err != nil {
		t.Fatalf("Failed to write decisions/a.md: %v", err)
	}

	// 1. Test LoadBundle on dotBundleDir
	bundle, err := okf.LoadBundle(dotBundleDir)
	if err != nil {
		t.Fatalf("LoadBundle failed on dot-directory bundle: %v", err)
	}

	if bundle.DeclaredVer != "0.2" {
		t.Errorf("Expected DeclaredVer '0.2', got %q", bundle.DeclaredVer)
	}

	if len(bundle.Concepts) != 1 {
		t.Fatalf("Expected exactly 1 concept in dot bundle, got %d: %+v", len(bundle.Concepts), bundle.Concepts)
	}
	if _, ok := bundle.Concepts["decisions/a"]; !ok {
		t.Errorf("Expected concept 'decisions/a' to be loaded")
	}

	// 2. Validate dot directory bundle
	res := okf.Validate(bundle, okf.ValidateOptions{Strict: true})
	if !res.IsConformant {
		t.Errorf("Expected dot bundle to be conformant, got errors: %v", res.Errors)
	}
	if len(res.Errors) != 0 || len(res.Warnings) != 0 {
		t.Errorf("Expected 0 errors and 0 warnings, got errors=%v, warnings=%v", res.Errors, res.Warnings)
	}

	// 3. Test relative path variations: .okf, .okf/, ./.okf
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get wd: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to chdir: %v", err)
	}

	for _, relRoot := range []string{".okf", ".okf/", "./.okf"} {
		bRel, err := okf.LoadBundle(relRoot)
		if err != nil {
			t.Errorf("LoadBundle(%q) failed: %v", relRoot, err)
			continue
		}
		if len(bRel.Concepts) != 1 {
			t.Errorf("LoadBundle(%q) loaded %d concepts, expected 1", relRoot, len(bRel.Concepts))
		}
	}
}

func TestBuildGraph_NestedAgentsConceptAndRootReservedFiles(t *testing.T) {
	tmpDir := t.TempDir()

	rootIndex := "---\nokf_version: \"0.2\"\n---\n# Knowledge Base\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "index.md"), []byte(rootIndex), 0o644); err != nil {
		t.Fatalf("WriteFile index.md: %v", err)
	}

	decisionsDir := filepath.Join(tmpDir, "decisions")
	if err := os.MkdirAll(decisionsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll decisions: %v", err)
	}

	// decisions/agents.md is a perfectly legitimate nested concept
	agentsConcept := `---
type: Concept
title: Agent Workers
description: How agents work.
generated: { by: agent/test, at: 2026-09-08T08:00:00Z }
---
# Agent Workers
Details about agents.
`
	if err := os.WriteFile(filepath.Join(decisionsDir, "agents.md"), []byte(agentsConcept), 0o644); err != nil {
		t.Fatalf("WriteFile agents.md: %v", err)
	}

	// decisions/caller.md links to decisions/agents.md (valid), but also attempts to link to root AGENTS.md, log.md, and index.md
	callerConcept := `---
type: Concept
title: Caller Concept
description: Calls agents and navigation.
generated: { by: agent/test, at: 2026-09-08T08:00:00Z }
---
# Caller
See [Agent Workers](agents.md) for concepts.
Also invalid navigation: [Root Agents](../AGENTS.md), [Root Log](../log.md), and [Folder Index](index.md).
`
	if err := os.WriteFile(filepath.Join(decisionsDir, "caller.md"), []byte(callerConcept), 0o644); err != nil {
		t.Fatalf("WriteFile caller.md: %v", err)
	}

	bundle, err := okf.LoadBundle(tmpDir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}

	// The link from caller to agents must exist in the graph!
	outbound := bundle.Graph["decisions/caller"]
	foundNestedLink := false
	for _, target := range outbound {
		if target == "decisions/agents" {
			foundNestedLink = true
			break
		}
	}
	if !foundNestedLink {
		t.Errorf("Expected decisions/caller -> decisions/agents link in graph, but got: %v", outbound)
	}

	// Exactly 3 broken links for navigation: ../AGENTS.md, ../log.md, index.md
	if len(bundle.BrokenLinks) != 3 {
		t.Fatalf("Expected 3 broken links for navigation files, got %d: %+v", len(bundle.BrokenLinks), bundle.BrokenLinks)
	}

	for _, bl := range bundle.BrokenLinks {
		if !strings.Contains(bl.Reason, "navigation") {
			t.Errorf("Expected navigation reason for broken link %q, got %q", bl.TargetHref, bl.Reason)
		}
	}
}
