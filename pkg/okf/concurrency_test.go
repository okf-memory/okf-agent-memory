package okf

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConcurrent_SaveConcept_PreservesAllIndexAndLogEntries(t *testing.T) {
	bundleDir := t.TempDir()
	if err := InitBundle(bundleDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	const workers = 20
	var wg sync.WaitGroup
	errs := make(chan error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := fmt.Sprintf("architecture/service-%02d", idx)
			c := &Concept{
				ID:          id,
				Path:        id + ".md",
				Type:        "Architecture",
				Title:       fmt.Sprintf("Service %02d", idx),
				Description: fmt.Sprintf("Microservice component %02d.", idx),
				Status:      "stable",
				Body:        fmt.Sprintf("# Service %02d\n\nDetailed specification for service %02d.\n", idx, idx),
			}
			if err := SaveConcept(bundleDir, c, SaveOptions{
				IsNew:     true,
				AutoIndex: true,
				AutoLog:   true,
				Actor:     fmt.Sprintf("agent/worker-%02d", idx),
			}); err != nil {
				errs <- fmt.Errorf("worker %02d: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent SaveConcept error: %v", err)
	}

	// Ensure lock file is cleaned up
	if _, err := os.Stat(filepath.Join(bundleDir, bundleWriteLockFile)); !os.IsNotExist(err) {
		t.Errorf("expected %s to be removed after mutations, got err=%v", bundleWriteLockFile, err)
	}

	// Root index must list architecture/index.md so strict validation has zero orphans
	rootIdxPath := filepath.Join(bundleDir, "index.md")
	rootIdxBytes, err := os.ReadFile(rootIdxPath)
	if err != nil {
		t.Fatalf("read root index.md: %v", err)
	}
	rootIdxUpdated := string(rootIdxBytes) + "* [Architecture](architecture/index.md)\n"
	if err := os.WriteFile(rootIdxPath, []byte(rootIdxUpdated), 0o644); err != nil {
		t.Fatalf("update root index.md: %v", err)
	}

	b, err := LoadBundle(bundleDir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}

	if len(b.Concepts) != workers {
		t.Fatalf("expected %d concepts in bundle, got %d", workers, len(b.Concepts))
	}

	archIndex := b.Indexes["architecture/index.md"]
	for i := 0; i < workers; i++ {
		filename := fmt.Sprintf("service-%02d.md", i)
		if !strings.Contains(archIndex, "("+filename+")") {
			t.Errorf("architecture/index.md missing listing for %s", filename)
		}
		if !strings.Contains(b.LogContent, fmt.Sprintf("`architecture/%s`", filename)) {
			t.Errorf("log.md missing entry for architecture/%s", filename)
		}
	}

	res := Validate(b, ValidateOptions{Strict: true, Drift: true})
	if len(res.Errors) > 0 || len(res.Warnings) > 0 {
		t.Fatalf("expected 0 errors and 0 warnings after concurrent SaveConcept, got errors=%v warnings=%v", res.Errors, res.Warnings)
	}
}

func TestConcurrent_RelateConcepts_PreservesAllOutboundLinksAndLogEntries(t *testing.T) {
	bundleDir := t.TempDir()
	if err := InitBundle(bundleDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	src := &Concept{
		ID:          "architecture/gateway",
		Path:        "architecture/gateway.md",
		Type:        "Architecture",
		Title:       "API Gateway",
		Description: "Central ingress gateway.",
		Status:      "stable",
		Body:        "# API Gateway\n\nRoutes requests to downstream services.\n",
	}
	if err := SaveConcept(bundleDir, src, SaveOptions{
		IsNew:     true,
		AutoIndex: true,
		AutoLog:   true,
		Actor:     "agent/setup",
	}); err != nil {
		t.Fatalf("SaveConcept(gateway) failed: %v", err)
	}

	const targets = 15
	for i := 0; i < targets; i++ {
		id := fmt.Sprintf("architecture/dep-%02d", i)
		tgt := &Concept{
			ID:          id,
			Path:        id + ".md",
			Type:        "Architecture",
			Title:       fmt.Sprintf("Dependency %02d", i),
			Description: fmt.Sprintf("Downstream dependency %02d.", i),
			Status:      "stable",
			Body:        fmt.Sprintf("# Dependency %02d\n", i),
		}
		if err := SaveConcept(bundleDir, tgt, SaveOptions{
			IsNew:     true,
			AutoIndex: true,
			AutoLog:   true,
			Actor:     "agent/setup",
		}); err != nil {
			t.Fatalf("SaveConcept(%s) failed: %v", id, err)
		}
	}

	var wg sync.WaitGroup
	errs := make(chan error, targets)
	for i := 0; i < targets; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			targetID := fmt.Sprintf("architecture/dep-%02d", idx)
			desc := fmt.Sprintf("routes traffic to dependency %02d", idx)
			if err := RelateConcepts(bundleDir, "architecture/gateway", targetID, desc, fmt.Sprintf("agent/rel-%02d", idx)); err != nil {
				errs <- fmt.Errorf("relate %02d: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent RelateConcepts error: %v", err)
	}

	b, err := LoadBundle(bundleDir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}

	gw := b.Concepts["architecture/gateway"]
	if gw == nil {
		t.Fatal("architecture/gateway concept missing")
	}

	for i := 0; i < targets; i++ {
		targetFile := fmt.Sprintf("dep-%02d.md", i)
		if !strings.Contains(gw.Body, "("+targetFile+")") {
			t.Errorf("gateway.md lost link to %s during concurrent RelateConcepts", targetFile)
		}
		logLine := fmt.Sprintf("Linked `architecture/gateway.md` to `architecture/%s`", targetFile)
		if !strings.Contains(b.LogContent, logLine) {
			t.Errorf("log.md lost entry %q", logLine)
		}
	}
}

func TestConcurrent_UpdateConcept_And_MCPUpdate_PreservesAllFieldMutations(t *testing.T) {
	bundleDir := t.TempDir()
	if err := InitBundle(bundleDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	initial := &Concept{
		ID:          "decisions/caching",
		Path:        "decisions/caching.md",
		Type:        "Decision",
		Title:       "Initial Title",
		Description: "Initial description.",
		Status:      "draft",
		Tags:        []string{"initial"},
		Body:        "# Caching\n\nInitial body.\n",
	}
	if err := SaveConcept(bundleDir, initial, SaveOptions{
		IsNew:     true,
		AutoIndex: true,
		AutoLog:   true,
		Actor:     "agent/init",
	}); err != nil {
		t.Fatalf("SaveConcept failed: %v", err)
	}

	// Run 3 concurrent updates on different fields (1 via MCP handleToolCall, 2 via UpdateConcept)
	var wg sync.WaitGroup
	errs := make(chan error, 3)

	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := UpdateConcept(bundleDir, "decisions/caching", SaveOptions{
			AutoIndex: true,
			AutoLog:   true,
			Actor:     "agent/updater-title",
		}, func(c *Concept) error {
			c.Title = "Distributed Redis Caching"
			return nil
		})
		if err != nil {
			errs <- err
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := UpdateConcept(bundleDir, "decisions/caching", SaveOptions{
			AutoIndex: true,
			AutoLog:   true,
			Actor:     "agent/updater-tags",
		}, func(c *Concept) error {
			c.Status = "stable"
			c.Tags = []string{"redis", "cache", "cluster"}
			return nil
		})
		if err != nil {
			errs <- err
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		var out bytes.Buffer
		srv := &mcpServer{
			bundleDir: bundleDir,
			rootDir:   filepath.Dir(bundleDir),
			writer:    &out,
		}
		rawID := json.RawMessage(`1`)
		params, _ := json.Marshal(mcpToolCallParams{
			Name: "okf_update",
			Arguments: map[string]any{
				"concept_id":  "decisions/caching",
				"description": "Updated description from concurrent MCP tool call.",
			},
		})
		srv.handleToolCall(jsonRPCRequest{
			JSONRPC: "2.0",
			ID:      &rawID,
			Method:  "tools/call",
			Params:  params,
		})
		if !strings.Contains(out.String(), "Successfully updated concept") {
			errs <- fmt.Errorf("MCP okf_update failed: %s", out.String())
		}
	}()

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent update failed: %v", err)
	}

	b, err := LoadBundle(bundleDir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}
	c := b.Concepts["decisions/caching"]
	if c.Title != "Distributed Redis Caching" {
		t.Errorf("expected Title='Distributed Redis Caching', got %q", c.Title)
	}
	if c.Status != "stable" {
		t.Errorf("expected Status='stable', got %q", c.Status)
	}
	if len(c.Tags) != 3 || c.Tags[0] != "redis" {
		t.Errorf("expected updated tags [redis cache cluster], got %v", c.Tags)
	}
	if c.Description != "Updated description from concurrent MCP tool call." {
		t.Errorf("expected updated description from MCP, got %q", c.Description)
	}
}

func TestSaveConcept_OptimisticConcurrencyCheck_RejectsStaleSnapshot(t *testing.T) {
	bundleDir := t.TempDir()
	if err := InitBundle(bundleDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	orig := &Concept{
		ID:          "architecture/db",
		Path:        "architecture/db.md",
		Type:        "Architecture",
		Title:       "Database v1",
		Description: "Original database spec.",
		Status:      "stable",
		Body:        "# Database v1\n",
	}
	if err := SaveConcept(bundleDir, orig, SaveOptions{IsNew: true, AutoIndex: true, AutoLog: true, Actor: "agent/v1"}); err != nil {
		t.Fatalf("SaveConcept(orig) failed: %v", err)
	}

	// Process A loads the bundle and holds a snapshot of architecture/db
	bStale, err := LoadBundle(bundleDir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}
	staleConcept := bStale.Concepts["architecture/db"]

	// Process B updates architecture/db on disk in the meantime
	if _, err := UpdateConcept(bundleDir, "architecture/db", SaveOptions{
		AutoIndex: true,
		AutoLog:   true,
		Actor:     "agent/v2",
	}, func(c *Concept) error {
		c.Title = "Database v2 (Committed by B)"
		c.Body = "# Database v2\n\nCommitted by process B.\n"
		return nil
	}); err != nil {
		t.Fatalf("UpdateConcept by B failed: %v", err)
	}

	// Process A now attempts to save its stale snapshot: must fail with ErrConcurrentModification
	staleConcept.Title = "Database v1 (Stale Overwrite by A)"
	err = SaveConcept(bundleDir, staleConcept, SaveOptions{
		IsNew:     false,
		AutoIndex: true,
		AutoLog:   true,
		Actor:     "agent/v1",
	})
	if !errors.Is(err, ErrConcurrentModification) {
		t.Fatalf("expected ErrConcurrentModification when saving stale snapshot, got: %v", err)
	}

	// Verify Process B's committed version on disk was NOT overwritten
	bVerify, err := LoadBundle(bundleDir)
	if err != nil {
		t.Fatalf("LoadBundle verify failed: %v", err)
	}
	if got := bVerify.Concepts["architecture/db"].Title; got != "Database v2 (Committed by B)" {
		t.Fatalf("stale snapshot overwrote committed version: got Title=%q", got)
	}

	// Also verify that sequential saves on a fresh loaded concept advance sourceHash cleanly
	fresh := bVerify.Concepts["architecture/db"]
	fresh.Description = "Sequential edit 1"
	if err := SaveConcept(bundleDir, fresh, SaveOptions{IsNew: false, Actor: "agent/v2"}); err != nil {
		t.Fatalf("first sequential SaveConcept failed: %v", err)
	}
	fresh.Description = "Sequential edit 2"
	if err := SaveConcept(bundleDir, fresh, SaveOptions{IsNew: false, Actor: "agent/v2"}); err != nil {
		t.Fatalf("second sequential SaveConcept on same struct failed: %v", err)
	}
}

func TestWithBundleLock_StaleLockEvictionAndTimeout(t *testing.T) {
	bundleDir := t.TempDir()
	if err := InitBundle(bundleDir); err != nil {
		t.Fatalf("InitBundle failed: %v", err)
	}

	lockPath := filepath.Join(bundleDir, bundleWriteLockFile)

	// 1. Stale lock from a crashed process (timestamp 2 minutes ago) must be evicted automatically
	staleMeta := fmt.Sprintf("pid=999999\nacquired_at=%s\n", time.Now().UTC().Add(-2*time.Minute).Format(time.RFC3339Nano))
	if err := os.WriteFile(lockPath, []byte(staleMeta), 0o600); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}

	executed := false
	if err := withBundleLockConfig(bundleDir, 200*time.Millisecond, 1*time.Second, func() error {
		executed = true
		return nil
	}); err != nil {
		t.Fatalf("expected stale lock to be evicted, got error: %v", err)
	}
	if !executed {
		t.Fatal("expected callback to execute after stale lock eviction")
	}

	// 2. Fresh lock held by an active writer must cause timeout when not released within timeout
	freshMeta := fmt.Sprintf("pid=%d\nacquired_at=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano))
	if err := os.WriteFile(lockPath, []byte(freshMeta), 0o600); err != nil {
		t.Fatalf("write fresh lock: %v", err)
	}
	defer func() { _ = os.Remove(lockPath) }()

	err := withBundleLockConfig(bundleDir, 25*time.Millisecond, 10*time.Second, func() error {
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "timed out waiting for bundle write lock") {
		t.Fatalf("expected lock timeout error, got: %v", err)
	}
}
