package sync_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/okf-memory/okf-agent-memory/pkg/sync"
	"github.com/okf-memory/okf-agent-memory/pkg/vault"
)

func TestEngine_PushAndPullRoundtrip(t *testing.T) {
	server := sync.NewServer("")
	ctx := context.Background()
	vaultID := "test-vault-roundtrip"
	vaultKey := make([]byte, 32)
	for i := range vaultKey {
		vaultKey[i] = byte(i + 1)
	}

	// 1. Setup local Bundle A
	dirA := t.TempDir()
	file1Path := filepath.Join(dirA, "index.md")
	file2Path := filepath.Join(dirA, "architecture", "sync.md")
	_ = os.MkdirAll(filepath.Dir(file2Path), 0o755)

	_ = os.WriteFile(file1Path, []byte("# Root Index\nokf_version: 0.2\n"), 0o644)
	_ = os.WriteFile(file2Path, []byte("# Sync Architecture\nZero-knowledge details\n"), 0o644)

	clientA := newTestClient(server.Handler(), "token-a")
	engineA := sync.NewEngine(clientA, vaultID, vaultKey, dirA)

	authorA := vault.CommitAuthor{ClientID: "client-a", Agent: "agent-a"}
	pushResA, err := engineA.Push(ctx, authorA, "Initial commit from A")
	if err != nil {
		t.Fatalf("EngineA Push error: %v", err)
	}

	if pushResA.CommitHash == "" {
		t.Fatalf("expected non-empty commit hash")
	}

	// Verify server head is now at pushResA.CommitHash
	head, err := clientA.GetHead(ctx, vaultID)
	if err != nil {
		t.Fatalf("GetHead error: %v", err)
	}
	if head.HeadCommit != pushResA.CommitHash {
		t.Fatalf("expected remote head %s, got %s", pushResA.CommitHash, head.HeadCommit)
	}

	// 2. Setup Client B (empty folder)
	dirB := t.TempDir()
	clientB := newTestClient(server.Handler(), "token-b")
	engineB := sync.NewEngine(clientB, vaultID, vaultKey, dirB)

	pullResB, err := engineB.Pull(ctx)
	if err != nil {
		t.Fatalf("EngineB Pull error: %v", err)
	}

	if pullResB.CommitHash != pushResA.CommitHash {
		t.Fatalf("EngineB expected commit %s, got %s", pushResA.CommitHash, pullResB.CommitHash)
	}

	// Verify files exist in Dir B with identical plaintext content
	bFile1, err := os.ReadFile(filepath.Join(dirB, "index.md"))
	if err != nil {
		t.Fatalf("failed to read pulled index.md in dirB: %v", err)
	}
	if string(bFile1) != "# Root Index\nokf_version: 0.2\n" {
		t.Fatalf("unexpected content in bFile1: %s", string(bFile1))
	}

	bFile2, err := os.ReadFile(filepath.Join(dirB, "architecture", "sync.md"))
	if err != nil {
		t.Fatalf("failed to read pulled sync.md in dirB: %v", err)
	}
	if string(bFile2) != "# Sync Architecture\nZero-knowledge details\n" {
		t.Fatalf("unexpected content in bFile2: %s", string(bFile2))
	}
}

func TestEngine_Sync_DisjointMerge(t *testing.T) {
	server := sync.NewServer("")
	ctx := context.Background()
	vaultID := "test-vault-disjoint"
	vaultKey := make([]byte, 32)
	vaultKey[0] = 42

	// Setup Base
	dirA := t.TempDir()
	_ = os.WriteFile(filepath.Join(dirA, "base.md"), []byte("Base content"), 0o644)

	clientA := newTestClient(server.Handler(), "token-a")
	engineA := sync.NewEngine(clientA, vaultID, vaultKey, dirA)
	authorA := vault.CommitAuthor{ClientID: "client-a", Agent: "agent"}

	_, err := engineA.Push(ctx, authorA, "Initial commit")
	if err != nil {
		t.Fatalf("initial push error: %v", err)
	}

	// Client B pulls base
	dirB := t.TempDir()
	clientB := newTestClient(server.Handler(), "token-b")
	engineB := sync.NewEngine(clientB, vaultID, vaultKey, dirB)
	authorB := vault.CommitAuthor{ClientID: "client-b", Agent: "agent"}

	_, err = engineB.Pull(ctx)
	if err != nil {
		t.Fatalf("engineB pull error: %v", err)
	}

	// Client B adds note_b.md and pushes to Hub
	_ = os.WriteFile(filepath.Join(dirB, "note_b.md"), []byte("Note B content"), 0o644)
	_, err = engineB.Push(ctx, authorB, "Add note B")
	if err != nil {
		t.Fatalf("engineB push error: %v", err)
	}

	// Meanwhile, Client A adds note_a.md locally (without having pulled note_b.md)
	_ = os.WriteFile(filepath.Join(dirA, "note_a.md"), []byte("Note A content"), 0o644)

	// Client A runs Sync!
	syncResA, err := engineA.Sync(ctx, authorA, "Add note A and sync")
	if err != nil {
		t.Fatalf("engineA Sync error: %v", err)
	}

	if len(syncResA.Conflicts) != 0 {
		t.Fatalf("expected 0 conflicts in disjoint sync, got: %v", syncResA.Conflicts)
	}

	// Verify Client A now has both note_a.md and note_b.md
	if _, err := os.Stat(filepath.Join(dirA, "note_a.md")); err != nil {
		t.Fatalf("note_a.md missing in dirA")
	}
	if _, err := os.Stat(filepath.Join(dirA, "note_b.md")); err != nil {
		t.Fatalf("note_b.md should have been pulled to dirA during sync")
	}
}

func TestEngine_Sync_CollisionForking(t *testing.T) {
	server := sync.NewServer("")
	ctx := context.Background()
	vaultID := "test-vault-collision"
	vaultKey := make([]byte, 32)
	vaultKey[1] = 99

	// Setup Base
	dirA := t.TempDir()
	_ = os.WriteFile(filepath.Join(dirA, "shared.md"), []byte("Initial shared note"), 0o644)

	clientA := newTestClient(server.Handler(), "token-a")
	engineA := sync.NewEngine(clientA, vaultID, vaultKey, dirA)
	authorA := vault.CommitAuthor{ClientID: "client-a", Agent: "agent"}

	_, err := engineA.Push(ctx, authorA, "Initial commit")
	if err != nil {
		t.Fatalf("initial push error: %v", err)
	}

	// Client B pulls base
	dirB := t.TempDir()
	clientB := newTestClient(server.Handler(), "token-b")
	engineB := sync.NewEngine(clientB, vaultID, vaultKey, dirB)
	authorB := vault.CommitAuthor{ClientID: "client-b", Agent: "agent"}

	_, err = engineB.Pull(ctx)
	if err != nil {
		t.Fatalf("engineB pull error: %v", err)
	}

	// Client B modifies shared.md and pushes
	_ = os.WriteFile(filepath.Join(dirB, "shared.md"), []byte("Remote edits by B"), 0o644)
	_, err = engineB.Push(ctx, authorB, "Client B modified shared.md")
	if err != nil {
		t.Fatalf("engineB push error: %v", err)
	}

	// Client A modifies same shared.md differently
	_ = os.WriteFile(filepath.Join(dirA, "shared.md"), []byte("Local edits by A"), 0o644)

	// Client A runs Sync
	syncResA, err := engineA.Sync(ctx, authorA, "Client A modified shared.md and syncs")
	if err != nil {
		t.Fatalf("engineA Sync error: %v", err)
	}

	if len(syncResA.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(syncResA.Conflicts))
	}

	// Verify collision failsafe on disk:
	// shared.md contains Remote edits ("Remote edits by B")
	// shared.conflict-local.md contains Local edits ("Local edits by A")
	sharedContent, err := os.ReadFile(filepath.Join(dirA, "shared.md"))
	if err != nil {
		t.Fatalf("failed to read shared.md in dirA: %v", err)
	}
	if string(sharedContent) != "Remote edits by B" {
		t.Fatalf("shared.md expected 'Remote edits by B', got %q", string(sharedContent))
	}

	forkedPath := sync.ConflictForkPath("shared.md", syncResA.Conflicts[0].LocalEntry)
	forkedContent, err := os.ReadFile(filepath.Join(dirA, forkedPath))
	if err != nil {
		t.Fatalf("failed to read %s in dirA: %v", forkedPath, err)
	}
	if string(forkedContent) != "Local edits by A" {
		t.Fatalf("%s expected 'Local edits by A', got %q", forkedPath, string(forkedContent))
	}
}

func TestEngine_Sync_AutoMergeLogFile(t *testing.T) {
	server := sync.NewServer("")
	ctx := context.Background()
	vaultID := "test-vault-log-merge"
	vaultKey := make([]byte, 32)
	vaultKey[1] = 77

	initialLog := "# Knowledge Log\n\n## 2026-09-15\n* **Init**: Initialized knowledge bundle\n"

	// Base commit with log.md
	dirA := t.TempDir()
	_ = os.WriteFile(filepath.Join(dirA, "log.md"), []byte(initialLog), 0o644)

	clientA := newTestClient(server.Handler(), "token-a")
	engineA := sync.NewEngine(clientA, vaultID, vaultKey, dirA)
	authorA := vault.CommitAuthor{ClientID: "client-a", Agent: "agent"}

	_, err := engineA.Push(ctx, authorA, "Initial commit with log.md")
	if err != nil {
		t.Fatalf("initial push error: %v", err)
	}

	// Client B pulls base
	dirB := t.TempDir()
	clientB := newTestClient(server.Handler(), "token-b")
	engineB := sync.NewEngine(clientB, vaultID, vaultKey, dirB)
	authorB := vault.CommitAuthor{ClientID: "client-b", Agent: "agent"}

	_, err = engineB.Pull(ctx)
	if err != nil {
		t.Fatalf("engineB pull error: %v", err)
	}

	// Client B appends an entry to log.md and pushes
	logB := "# Knowledge Log\n\n## 2026-09-17\n* **Create**: Added concepts/remote.md\n\n## 2026-09-15\n* **Init**: Initialized knowledge bundle\n"
	_ = os.WriteFile(filepath.Join(dirB, "log.md"), []byte(logB), 0o644)
	_, err = engineB.Push(ctx, authorB, "Client B added concept and logged it")
	if err != nil {
		t.Fatalf("engineB push error: %v", err)
	}

	// Client A appends a DIFFERENT entry to log.md
	logA := "# Knowledge Log\n\n## 2026-09-17\n* **Create**: Added concepts/local.md\n\n## 2026-09-15\n* **Init**: Initialized knowledge bundle\n"
	_ = os.WriteFile(filepath.Join(dirA, "log.md"), []byte(logA), 0o644)

	// Client A runs Sync
	syncResA, err := engineA.Sync(ctx, authorA, "Client A syncs after logging local concept")
	if err != nil {
		t.Fatalf("engineA Sync error: %v", err)
	}

	// Should auto-merge log.md cleanly without conflict
	if len(syncResA.Conflicts) != 0 {
		t.Fatalf("expected 0 conflicts because log.md should auto-merge, got: %v", syncResA.Conflicts)
	}

	// log.conflict-local.md should NOT exist
	if _, err := os.Stat(filepath.Join(dirA, "log.conflict-local.md")); !os.IsNotExist(err) {
		t.Fatalf("log.conflict-local.md should not exist")
	}

	// Merged log.md on disk must contain both entries
	mergedContent, err := os.ReadFile(filepath.Join(dirA, "log.md"))
	if err != nil {
		t.Fatalf("failed to read log.md in dirA: %v", err)
	}
	mergedStr := string(mergedContent)
	if !strings.Contains(mergedStr, "* **Create**: Added concepts/local.md") {
		t.Errorf("merged log.md missing local entry:\n%s", mergedStr)
	}
	if !strings.Contains(mergedStr, "* **Create**: Added concepts/remote.md") {
		t.Errorf("merged log.md missing remote entry:\n%s", mergedStr)
	}
}

func TestEngine_Sync_RetriesOnConcurrentRace(t *testing.T) {
	server := sync.NewServer("")
	ctx := context.Background()
	vaultID := "test-vault-retry-race"
	vaultKey := make([]byte, 32)
	vaultKey[1] = 42

	// Setup Base
	dirA := t.TempDir()
	_ = os.WriteFile(filepath.Join(dirA, "fileA.md"), []byte("Local file A"), 0o644)

	clientBase := newTestClient(server.Handler(), "token-base")
	engineBase := sync.NewEngine(clientBase, vaultID, vaultKey, dirA)
	authorBase := vault.CommitAuthor{ClientID: "client-base", Agent: "agent"}

	_, err := engineBase.Push(ctx, authorBase, "Base commit")
	if err != nil {
		t.Fatalf("base push error: %v", err)
	}

	// Client B pushes commit B
	dirB := t.TempDir()
	clientB := newTestClient(server.Handler(), "token-b")
	engineB := sync.NewEngine(clientB, vaultID, vaultKey, dirB)
	authorB := vault.CommitAuthor{ClientID: "client-b", Agent: "agent"}
	_, err = engineB.Pull(ctx)
	if err != nil {
		t.Fatalf("pull B error: %v", err)
	}
	_ = os.WriteFile(filepath.Join(dirB, "fileB.md"), []byte("Remote file B"), 0o644)
	_, err = engineB.Push(ctx, authorB, "Commit from B")
	if err != nil {
		t.Fatalf("push B error: %v", err)
	}

	// Client C will be used to inject an intervening commit while Client A is syncing
	dirC := t.TempDir()
	clientC := newTestClient(server.Handler(), "token-c")
	engineC := sync.NewEngine(clientC, vaultID, vaultKey, dirC)
	authorC := vault.CommitAuthor{ClientID: "client-c", Agent: "agent"}
	_, err = engineC.Pull(ctx)
	if err != nil {
		t.Fatalf("pull C error: %v", err)
	}
	_ = os.WriteFile(filepath.Join(dirC, "fileC.md"), []byte("Remote file C"), 0o644)

	// Now Client A modifies fileA.md locally
	_ = os.WriteFile(filepath.Join(dirA, "fileA.md"), []byte("Local file A modified"), 0o644)

	// We wrap server.Handler() for Client A:
	// 1st commit is Client A's initial Push (which 409s because B pushed).
	// 2nd commit is Client A's merge commit attempt. We intervene right before it executes,
	// moving head to C. Without a retry loop, Client A immediately fails!
	var commitCount atomic.Int32
	interceptingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/commit") && r.Method == http.MethodPost {
			count := commitCount.Add(1)
			if count == 2 {
				// Intervene on Client A's merge commit: Client C pushes now, moving head before A's commit
				_, cErr := engineC.Push(ctx, authorC, "Intervening commit from C")
				if cErr != nil {
					t.Errorf("intervening push failed: %v", cErr)
				}
			}
		}
		server.Handler().ServeHTTP(w, r)
	})

	clientA := newTestClient(interceptingHandler, "token-a")
	engineA := sync.NewEngine(clientA, vaultID, vaultKey, dirA)
	authorA := vault.CommitAuthor{ClientID: "client-a", Agent: "agent"}

	syncRes, err := engineA.Sync(ctx, authorA, "Client A sync with race")
	if err != nil {
		t.Fatalf("expected sync to succeed via retry loop, but failed with: %v", err)
	}

	if syncRes.CommitHash == "" {
		t.Fatalf("expected non-empty commit hash on successful sync")
	}

	// Verify all files exist in dirA after successful sync: fileA.md, fileB.md, fileC.md
	for _, expectedFile := range []string{"fileA.md", "fileB.md", "fileC.md"} {
		if _, err := os.Stat(filepath.Join(dirA, expectedFile)); err != nil {
			t.Errorf("expected %s to be present in dirA after multi-reconcile sync", expectedFile)
		}
	}
}
