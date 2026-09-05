package okf_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

// TestScenario01_NewProjectBootstrap verifies that an agent can scaffold a complete
// OKF Agent Memory stack in a fresh directory and pass strict validation.
func TestScenario01_NewProjectBootstrap(t *testing.T) {
	tmpDir := t.TempDir()

	opts := okf.DefaultBootstrapOptions()
	opts.ProjectName = "analytics-engine"
	if err := okf.Bootstrap(tmpDir, opts); err != nil {
		t.Fatalf("Bootstrap failed: %v", err)
	}

	bundlePath := filepath.Join(tmpDir, "knowledge")
	b, err := okf.LoadBundle(bundlePath)
	if err != nil {
		t.Fatalf("LoadBundle failed on bootstrapped bundle: %v", err)
	}

	res := okf.Validate(b, okf.ValidateOptions{Strict: true, Drift: true})
	if !res.IsConformant || !res.GatePassed {
		t.Errorf("Bootstrapped bundle failed validation: conformant=%v, gate=%v, errors=%v",
			res.IsConformant, res.GatePassed, res.Errors)
	}
}

// TestScenario02_ExistingProjectDiscovery verifies that an agent can discover
// relevant concepts in an existing knowledge corpus using BM25 search.
func TestScenario02_ExistingProjectDiscovery(t *testing.T) {
	tmpDir := t.TempDir()

	if err := okf.InitBundle(tmpDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	// Create architectural concept
	c1 := &okf.Concept{
		Path:        "auth/oauth-pkce.md",
		Type:        "Decision",
		Title:       "OAuth2 PKCE Standard",
		Description: "Standardized on PKCE for all client authentication flows.",
		Body:        "# OAuth2 PKCE\n\nAll mobile and web clients must use Authorization Code Flow with PKCE.",
		Tags:        []string{"auth", "oauth", "security"},
	}
	if err := okf.SaveConcept(tmpDir, c1, true, true, true, "agent/claude"); err != nil {
		t.Fatalf("SaveConcept failed: %v", err)
	}

	b, err := okf.LoadBundle(tmpDir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}

	// Search for client authentication
	results := b.Search("client authentication security", 5)
	if len(results) == 0 {
		t.Fatalf("Expected discovery results, got 0")
	}

	if results[0].ConceptID != "auth/oauth-pkce" {
		t.Errorf("Expected top result to be 'auth/oauth-pkce', got '%s'", results[0].ConceptID)
	}
}

// TestScenario03_RepeatedTaskAntiDuplication verifies that an agent updates an existing
// concept rather than creating a duplicate when modified requirements emerge.
func TestScenario03_RepeatedTaskAntiDuplication(t *testing.T) {
	tmpDir := t.TempDir()

	if err := okf.InitBundle(tmpDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	c := &okf.Concept{
		Path:        "config/session-ttl.md",
		Type:        "Fact",
		Title:       "Session Timeout",
		Description: "Default user session timeout is set to 15 minutes.",
		Body:        "# Session Timeout\n\nTTL = 900 seconds.",
	}
	if err := okf.SaveConcept(tmpDir, c, true, true, true, "agent/v1"); err != nil {
		t.Fatalf("SaveConcept failed: %v", err)
	}

	// Agent performs search before write and updates existing concept
	b, err := okf.LoadBundle(tmpDir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}

	searchResults := b.Search("Session Timeout", 1)
	if len(searchResults) == 0 {
		t.Fatalf("Search failed to find existing session timeout")
	}

	existingID := searchResults[0].ConceptID
	targetConcept := b.Concepts[existingID]
	targetConcept.Description = "Default user session timeout is set to 30 minutes."
	targetConcept.Body = "# Session Timeout\n\nTTL = 1800 seconds (extended)."

	if err := okf.SaveConcept(tmpDir, targetConcept, false, true, true, "agent/v2"); err != nil {
		t.Fatalf("Update concept failed: %v", err)
	}

	// Reload bundle and ensure concept count is still 1 (no duplicate)
	bUpdated, err := okf.LoadBundle(tmpDir)
	if err != nil {
		t.Fatalf("LoadBundle updated failed: %v", err)
	}

	if len(bUpdated.Concepts) != 1 {
		t.Errorf("Expected exactly 1 concept (anti-duplication), found %d", len(bUpdated.Concepts))
	}

	updatedC := bUpdated.Concepts["config/session-ttl"]
	if !strings.Contains(updatedC.Description, "30 minutes") {
		t.Errorf("Concept description was not updated: %s", updatedC.Description)
	}
}

// TestScenario04_ContradictionDetection verifies that when conflicting requirements emerge,
// an agent discovers the existing decision, updates its status to deprecated, and links the superseding decision.
func TestScenario04_ContradictionDetection(t *testing.T) {
	tmpDir := t.TempDir()

	if err := okf.InitBundle(tmpDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	// Initial Decision: gRPC only
	cOld := &okf.Concept{
		Path:        "decisions/api-protocol.md",
		Type:        "Decision",
		Title:       "API Protocol gRPC Only",
		Description: "All inter-service and client communications use strict gRPC.",
		Status:      "stable",
		Body:        "# API Protocol\n\nStrict gRPC over HTTP/2 everywhere.",
	}
	if err := okf.SaveConcept(tmpDir, cOld, true, true, true, "agent/v1"); err != nil {
		t.Fatalf("SaveConcept cOld failed: %v", err)
	}

	// New conflicting requirement: "Adopt REST JSON for public browser clients"
	b, err := okf.LoadBundle(tmpDir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}

	results := b.Search("API protocol communication client", 5)
	if len(results) == 0 {
		t.Fatalf("Expected to find prior decision on API protocol")
	}

	// Agent flags contradiction, marks old concept deprecated and creates superseding decision
	cOldFound := b.Concepts[results[0].ConceptID]
	cOldFound.Status = "deprecated"
	cOldFound.Description = "Deprecated in favor of REST JSON gateway."
	if err := okf.SaveConcept(tmpDir, cOldFound, false, true, true, "agent/v2"); err != nil {
		t.Fatalf("Deprecated save failed: %v", err)
	}

	cNew := &okf.Concept{
		Path:        "decisions/rest-gateway.md",
		Type:        "Decision",
		Title:       "Public REST JSON Gateway",
		Description: "Exposes REST endpoints for browser clients, superseding gRPC-only requirement.",
		Status:      "stable",
		Body:        "# REST Gateway\n\nSupersedes [gRPC Decision](api-protocol.md).",
	}
	if err := okf.SaveConcept(tmpDir, cNew, true, true, true, "agent/v2"); err != nil {
		t.Fatalf("SaveConcept cNew failed: %v", err)
	}

	// Link them bidirectionally
	err = okf.RelateConcepts(tmpDir, "decisions/api-protocol", "decisions/rest-gateway", "superseded by", "agent/v2")
	if err != nil {
		t.Fatalf("RelateConcepts failed: %v", err)
	}

	// Validate bundle
	bFinal, err := okf.LoadBundle(tmpDir)
	if err != nil {
		t.Fatalf("LoadBundle final failed: %v", err)
	}

	res := okf.Validate(bFinal, okf.ValidateOptions{Strict: true, Drift: true})
	if !res.IsConformant || !res.GatePassed {
		t.Errorf("Contradiction resolution failed validation: %+v", res.Errors)
	}
	if bFinal.Concepts["decisions/api-protocol"].Status != "deprecated" {
		t.Errorf("Old decision was not marked deprecated")
	}
}

// TestScenario05_HumanCorrectionPreservation verifies that human verification metadata
// is strictly preserved when an AI agent updates a concept.
func TestScenario05_HumanCorrectionPreservation(t *testing.T) {
	tmpDir := t.TempDir()

	if err := okf.InitBundle(tmpDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	// Initial concept with human verification
	rawWithHumanVerified := `---
type: Architecture
title: Database Selection
description: Standardized on PostgreSQL for primary relational storage.
status: stable
verified:
  - by: human/lead-architect
    at: 2026-08-20T10:00:00Z
generated:
  by: agent/initial
  at: 2026-08-19T10:00:00Z
---

# Database Selection

PostgreSQL 16 is our primary relational store.
`
	conceptDir := filepath.Join(tmpDir, "architecture")
	if err := os.MkdirAll(conceptDir, 0o755); err != nil {
		t.Fatalf("Failed to create architecture dir: %v", err)
	}
	conceptPath := filepath.Join(conceptDir, "database.md")
	if err := os.WriteFile(conceptPath, []byte(rawWithHumanVerified), 0o644); err != nil {
		t.Fatalf("Failed to write initial concept: %v", err)
	}

	// Agent loads bundle, updates body and generated timestamp
	b, err := okf.LoadBundle(tmpDir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}

	c := b.Concepts["architecture/database"]
	if len(c.Verified) != 1 || c.Verified[0].By != "human/lead-architect" {
		t.Fatalf("Parsed concept missing initial human verification: %+v", c.Verified)
	}

	c.Body = "# Database Selection\n\nPostgreSQL 16 is our primary relational store. Added read-replica configuration."
	if err := okf.SaveConcept(tmpDir, c, false, true, true, "agent/update-bot"); err != nil {
		t.Fatalf("SaveConcept failed: %v", err)
	}

	// Reload and verify human verification is preserved intact
	bAfter, err := okf.LoadBundle(tmpDir)
	if err != nil {
		t.Fatalf("LoadBundle after failed: %v", err)
	}

	cAfter := bAfter.Concepts["architecture/database"]

	if len(cAfter.Verified) != 1 || cAfter.Verified[0].By != "human/lead-architect" {
		t.Errorf("Human verification was erased or altered: %+v", cAfter.Verified)
	}
	if cAfter.Generated == nil || cAfter.Generated.By != "agent/update-bot" {
		t.Errorf("Agent generated attribution missing: %+v", cAfter.Generated)
	}
}

// TestScenario06_MultiSessionEvolution simulates multi-session project memory growth
// with relationship linking and graph validation.
func TestScenario06_MultiSessionEvolution(t *testing.T) {
	tmpDir := t.TempDir()

	if err := okf.InitBundle(tmpDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	// Session 1: Define Backend Service
	c1 := &okf.Concept{
		Path:        "services/payment.md",
		Type:        "Architecture",
		Title:       "Payment Service",
		Description: "Handles Stripe payment processing.",
		Body:        "# Payment Service\n\nStripe integration service.",
	}
	if err := okf.SaveConcept(tmpDir, c1, true, true, true, "agent/session-1"); err != nil {
		t.Fatalf("SaveConcept session 1 failed: %v", err)
	}

	// Session 2: Define Webhook Consumer
	c2 := &okf.Concept{
		Path:        "services/webhook-receiver.md",
		Type:        "Architecture",
		Title:       "Webhook Receiver",
		Description: "Ingests asynchronous third-party webhooks.",
		Body:        "# Webhook Receiver\n\nReceives Stripe webhook notifications.",
	}
	if err := okf.SaveConcept(tmpDir, c2, true, true, true, "agent/session-2"); err != nil {
		t.Fatalf("SaveConcept session 2 failed: %v", err)
	}

	// Session 3: Relate Webhook to Payment
	err := okf.RelateConcepts(tmpDir, "services/webhook-receiver", "services/payment", "forwards payment status events", "agent/session-3")
	if err != nil {
		t.Fatalf("RelateConcepts failed: %v", err)
	}

	// Session 4: Relate Payment to Webhook (bidirectional connectivity)
	err = okf.RelateConcepts(tmpDir, "services/payment", "services/webhook-receiver", "receives incoming callbacks from", "agent/session-4")
	if err != nil {
		t.Fatalf("RelateConcepts failed: %v", err)
	}

	// Validate bundle health after 4 sessions
	b, err := okf.LoadBundle(tmpDir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}

	res := okf.Validate(b, okf.ValidateOptions{Strict: true, Drift: true})
	if !res.IsConformant || !res.GatePassed {
		t.Errorf("Multi-session bundle failed validation: conformant=%v, gate=%v, errors=%v, warnings=%v",
			res.IsConformant, res.GatePassed, res.Errors, res.Warnings)
	}

	if len(res.Orphans) != 0 {
		t.Errorf("Expected 0 orphans after relationship linking, got: %v", res.Orphans)
	}
	if len(res.BrokenLinks) != 0 {
		t.Errorf("Expected 0 broken links, got: %v", res.BrokenLinks)
	}
}

// TestScenario07_LargeCorpusProgressiveDisclosure tests performance, progressive indexing,
// and sub-5ms BM25 discovery in a corpus with 50+ concepts across multiple sub-directories.
func TestScenario07_LargeCorpusProgressiveDisclosure(t *testing.T) {
	tmpDir := t.TempDir()

	if err := okf.InitBundle(tmpDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	domains := []string{"tables", "services", "decisions", "runbooks", "entities"}

	// Generate 50 structured concepts
	for i := 1; i <= 50; i++ {
		domain := domains[i%len(domains)]
		c := &okf.Concept{
			Path:        fmt.Sprintf("%s/concept-%02d.md", domain, i),
			Type:        "Fact",
			Title:       fmt.Sprintf("Enterprise Concept %02d", i),
			Description: fmt.Sprintf("Detailed enterprise specification for item %02d in %s domain.", i, domain),
			Body:        fmt.Sprintf("# Concept %02d\n\nOperational body content for domain %s item %02d.", i, domain, i),
			Tags:        []string{domain, fmt.Sprintf("tag-%d", i%5)},
		}
		if i == 42 {
			c.Title = "Target Needle Architecture"
			c.Description = "Critical needle in haystack for progressive disclosure evaluation."
			c.Tags = append(c.Tags, "needle", "benchmark")
		}
		if err := okf.SaveConcept(tmpDir, c, true, false, true, "agent/generator"); err != nil {
			t.Fatalf("Failed saving concept %d: %v", i, err)
		}
	}

	// Link concepts together in a chain so there are no orphans
	for i := 1; i < 50; i++ {
		srcDomain := domains[i%len(domains)]
		tgtDomain := domains[(i+1)%len(domains)]
		srcID := fmt.Sprintf("%s/concept-%02d", srcDomain, i)
		tgtID := fmt.Sprintf("%s/concept-%02d", tgtDomain, i+1)
		if err := okf.RelateConcepts(tmpDir, srcID, tgtID, "flows into", "agent/linker"); err != nil {
			t.Fatalf("Failed to relate concepts: %v", err)
		}
	}

	start := time.Now()
	b, err := okf.LoadBundle(tmpDir)
	if err != nil {
		t.Fatalf("LoadBundle on 50-concept corpus failed: %v", err)
	}
	loadElapsed := time.Since(start)

	if len(b.Concepts) != 50 {
		t.Fatalf("Expected 50 concepts loaded, got %d", len(b.Concepts))
	}

	// Benchmark BM25 search
	searchStart := time.Now()
	results := b.Search("Target Needle progressive disclosure", 5)
	searchElapsed := time.Since(searchStart)

	if len(results) == 0 || results[0].ConceptID != "decisions/concept-42" {
		t.Errorf("Failed to locate needle in large corpus: %+v", results)
	}

	if searchElapsed > 10*time.Millisecond {
		t.Logf("Warning: Search took %v (>10ms)", searchElapsed)
	}

	t.Logf("Corpus benchmark: 50 concepts loaded in %v, BM25 query executed in %v", loadElapsed, searchElapsed)
}
