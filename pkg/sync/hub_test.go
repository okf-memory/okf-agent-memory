package sync

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okf-memory/okf-agent-memory/pkg/vault"
)

type localRoundTripper struct {
	handler http.Handler
}

func (l *localRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	l.handler.ServeHTTP(rec, req)
	return rec.Result(), nil
}

func newTestHubServerForHub() *Server {
	return NewServer("")
}

func newTestClientFromHandlerForHub(handler http.Handler, token string) *Client {
	c := NewClient("http://hub.local", token)
	c.HTTPClient.Transport = &localRoundTripper{handler: handler}
	return c
}

func TestHub_InitVault(t *testing.T) {
	var buf bytes.Buffer
	dir := t.TempDir()

	err := InitVault(&buf, dir, "http://127.0.0.1:8080", "")
	if err != nil {
		t.Fatalf("InitVault error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "EMERGENCY KIT") {
		t.Fatalf("output missing EMERGENCY KIT header:\n%s", out)
	}
	if !strings.Contains(out, "Vault ID:") {
		t.Fatalf("output missing Vault ID:\n%s", out)
	}
	if !strings.Contains(out, "Secret Key:") {
		t.Fatalf("output missing Secret Key:\n%s", out)
	}

	cfgPath := filepath.Join(dir, ".okf-vault.json")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("expected config file %s to exist", cfgPath)
	}
}

func TestHub_PushPullSyncWithServer(t *testing.T) {
	dirA := t.TempDir()
	_ = os.WriteFile(filepath.Join(dirA, "index.md"), []byte("# Knowledge Index\nokf_version: 0.2\n"), 0o644)

	var initBuf bytes.Buffer
	if err := InitVault(&initBuf, dirA, "http://127.0.0.1:8080", ""); err != nil {
		t.Fatalf("init error: %v", err)
	}

	cfgA, err := LoadVaultConfig(dirA)
	if err != nil {
		t.Fatalf("load config error: %v", err)
	}

	secretKey, err := vault.GenerateSecretKey()
	if err != nil {
		t.Fatalf("GenerateSecretKey error: %v", err)
	}
	password := "master-pass-123"

	server := newTestHubServerForHub()
	clientA := newTestClientFromHandlerForHub(server.Handler(), "token-a")

	var pushBuf bytes.Buffer
	err = HubPush(&pushBuf, HubOp{
		Dir:          dirA,
		Client:       clientA,
		VaultID:      cfgA.VaultID,
		Password:     password,
		SecretKey:    secretKey,
		Message:      "Initial push",
		AgentVersion: "test",
	})
	if err != nil {
		t.Fatalf("HubPush error: %v", err)
	}
	if !strings.Contains(pushBuf.String(), "Push completed") {
		t.Fatalf("expected push completion message, got:\n%s", pushBuf.String())
	}

	dirB := t.TempDir()
	clientB := newTestClientFromHandlerForHub(server.Handler(), "token-b")
	var pullBuf bytes.Buffer
	err = HubPull(&pullBuf, HubOp{
		Dir:       dirB,
		Client:    clientB,
		VaultID:   cfgA.VaultID,
		Password:  password,
		SecretKey: secretKey,
	})
	if err != nil {
		t.Fatalf("HubPull error: %v", err)
	}

	pulledIndex, err := os.ReadFile(filepath.Join(dirB, "index.md"))
	if err != nil {
		t.Fatalf("failed to read pulled index.md: %v", err)
	}
	if string(pulledIndex) != "# Knowledge Index\nokf_version: 0.2\n" {
		t.Fatalf("unexpected content in pulled index.md: %s", string(pulledIndex))
	}
}

func TestHub_ResolveToken(t *testing.T) {
	cfg := &VaultConfig{
		VaultID:   "v_test",
		HubURL:    "http://127.0.0.1:8080",
		AuthToken: "cfg-token",
	}

	t.Setenv("OKF_HUB_TOKEN", "env-token")

	if tok := ResolveToken("flag-token", cfg); tok != "flag-token" {
		t.Fatalf("expected flag-token, got %s", tok)
	}

	if tok := ResolveToken("", cfg); tok != "env-token" {
		t.Fatalf("expected env-token, got %s", tok)
	}

	t.Setenv("OKF_HUB_TOKEN", "")
	if tok := ResolveToken("", cfg); tok != "cfg-token" {
		t.Fatalf("expected cfg-token, got %s", tok)
	}

	if tok := ResolveToken("", nil); tok != "" {
		t.Fatalf("expected empty string, got %s", tok)
	}
}

func TestHub_BearerAuthProtection(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "index.md"), []byte("# Index\n"), 0o644)
	var initBuf bytes.Buffer
	if err := InitVault(&initBuf, dir, "http://127.0.0.1:8080", "secret-token"); err != nil {
		t.Fatalf("init error: %v", err)
	}
	cfg, _ := LoadVaultConfig(dir)
	secretKey, _ := vault.GenerateSecretKey()

	authHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer secret-token" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		newTestHubServerForHub().Handler().ServeHTTP(w, r)
	})

	clientValid := newTestClientFromHandlerForHub(authHandler, "secret-token")
	var pushBuf bytes.Buffer
	err := HubPush(&pushBuf, HubOp{
		Dir:          dir,
		Client:       clientValid,
		VaultID:      cfg.VaultID,
		Password:     "pass",
		SecretKey:    secretKey,
		Message:      "msg",
		AgentVersion: "test",
	})
	if err != nil {
		t.Fatalf("expected push to succeed with valid token, got: %v", err)
	}

	clientInvalid := newTestClientFromHandlerForHub(authHandler, "wrong-token")
	err = HubPush(&pushBuf, HubOp{
		Dir:          dir,
		Client:       clientInvalid,
		VaultID:      cfg.VaultID,
		Password:     "pass",
		SecretKey:    secretKey,
		Message:      "msg",
		AgentVersion: "test",
	})
	if err == nil || !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("expected unauthorized error with wrong token, got: %v", err)
	}
}

func TestHub_ResolveHubURL(t *testing.T) {
	cfg := &VaultConfig{
		VaultID: "v_test",
		HubURL:  "https://hub.example.com",
	}

	if u := ResolveHubURL("http://override.local", cfg); u != "http://override.local" {
		t.Fatalf("expected override URL, got %s", u)
	}

	if u := ResolveHubURL("", cfg); u != "https://hub.example.com" {
		t.Fatalf("expected config URL, got %s", u)
	}

	if u := ResolveHubURL("", nil); u != "http://127.0.0.1:8080" {
		t.Fatalf("expected default URL, got %s", u)
	}
}

func TestHub_Serve_E2E_Socket(t *testing.T) {
	srv := NewServer("")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	password := "master-pass-e2e"
	secretKey, err := vault.GenerateSecretKey()
	if err != nil {
		t.Fatalf("GenerateSecretKey error: %v", err)
	}

	dirA := t.TempDir()
	_ = os.WriteFile(filepath.Join(dirA, "index.md"), []byte("# Root Index\nokf_version: 0.2\n"), 0o644)
	subDirA := filepath.Join(dirA, "concepts")
	_ = os.MkdirAll(subDirA, 0o755)
	_ = os.WriteFile(filepath.Join(subDirA, "architecture.md"), []byte("# Architecture\nSocket E2E test\n"), 0o644)

	var initBuf bytes.Buffer
	if err := InitVault(&initBuf, dirA, ts.URL, "secret-token-e2e"); err != nil {
		t.Fatalf("init-vault error: %v", err)
	}

	cfgA, err := LoadVaultConfig(dirA)
	if err != nil {
		t.Fatalf("load config error: %v", err)
	}
	if cfgA.HubURL != ts.URL {
		t.Fatalf("expected config hub_url %s, got %s", ts.URL, cfgA.HubURL)
	}

	clientA := NewClient(ts.URL, "secret-token-e2e")
	var pushBuf bytes.Buffer
	err = HubPush(&pushBuf, HubOp{
		Dir:          dirA,
		Client:       clientA,
		VaultID:      cfgA.VaultID,
		Password:     password,
		SecretKey:    secretKey,
		Message:      "Initial socket push",
		AgentVersion: "test",
	})
	if err != nil {
		t.Fatalf("real socket push failed: %v", err)
	}
	if !strings.Contains(pushBuf.String(), "Push completed") {
		t.Fatalf("expected push completed, got:\n%s", pushBuf.String())
	}

	headResp, err := clientA.GetHead(context.Background(), cfgA.VaultID)
	if err != nil {
		t.Fatalf("GetHead over socket failed: %v", err)
	}
	if headResp.HeadCommit == "" {
		t.Fatalf("expected non-empty head commit on remote server")
	}

	dirB := t.TempDir()
	clientB := NewClient(ts.URL, "secret-token-e2e")
	var pullBuf bytes.Buffer
	err = HubPull(&pullBuf, HubOp{
		Dir:       dirB,
		Client:    clientB,
		VaultID:   cfgA.VaultID,
		Password:  password,
		SecretKey: secretKey,
	})
	if err != nil {
		t.Fatalf("real socket pull failed: %v", err)
	}

	pulledIndex, err := os.ReadFile(filepath.Join(dirB, "index.md"))
	if err != nil {
		t.Fatalf("failed to read pulled index.md: %v", err)
	}
	if string(pulledIndex) != "# Root Index\nokf_version: 0.2\n" {
		t.Fatalf("mismatch in pulled index.md: %s", string(pulledIndex))
	}

	pulledArch, err := os.ReadFile(filepath.Join(dirB, "concepts", "architecture.md"))
	if err != nil {
		t.Fatalf("failed to read pulled architecture.md: %v", err)
	}
	if string(pulledArch) != "# Architecture\nSocket E2E test\n" {
		t.Fatalf("mismatch in pulled architecture.md: %s", string(pulledArch))
	}

	noteBPath := filepath.Join(dirB, "concepts", "note_b.md")
	_ = os.WriteFile(noteBPath, []byte("# Note B\nCreated on Device B\n"), 0o644)
	var syncBuf bytes.Buffer
	err = HubSync(&syncBuf, HubOp{
		Dir:          dirB,
		Client:       clientB,
		VaultID:      cfgA.VaultID,
		Password:     password,
		SecretKey:    secretKey,
		Message:      "Sync note B from device B",
		AgentVersion: "test",
	})
	if err != nil {
		t.Fatalf("real socket sync failed: %v", err)
	}
	if !strings.Contains(syncBuf.String(), "Sync completed") {
		t.Fatalf("expected sync completed, got:\n%s", syncBuf.String())
	}
	if strings.Contains(syncBuf.String(), "Collision detected") {
		t.Fatalf("expected no collision for disjoint add, got:\n%s", syncBuf.String())
	}

	var pullBuf2 bytes.Buffer
	err = HubPull(&pullBuf2, HubOp{
		Dir:       dirA,
		Client:    clientA,
		VaultID:   cfgA.VaultID,
		Password:  password,
		SecretKey: secretKey,
	})
	if err != nil {
		t.Fatalf("device A pull failed: %v", err)
	}

	pulledNoteB, err := os.ReadFile(filepath.Join(dirA, "concepts", "note_b.md"))
	if err != nil {
		t.Fatalf("device A failed to receive note_b.md: %v", err)
	}
	if string(pulledNoteB) != "# Note B\nCreated on Device B\n" {
		t.Fatalf("content mismatch for note_b.md on device A: %s", string(pulledNoteB))
	}

	_ = os.WriteFile(noteBPath, []byte("# Note B Conflicting Local Edit\n"), 0o644)
	var syncBufConflict bytes.Buffer
	err = HubSync(&syncBufConflict, HubOp{
		Dir:          dirB,
		Client:       clientB,
		VaultID:      cfgA.VaultID,
		Password:     password,
		SecretKey:    secretKey,
		Message:      "Device B sync conflicting edit",
		AgentVersion: "test",
	})
	if err != nil {
		t.Fatalf("device B sync with conflict failed: %v", err)
	}
	if !strings.Contains(syncBufConflict.String(), "Collision detected at concepts/note_b.md") {
		t.Fatalf("expected collision detected warning, got:\n%s", syncBufConflict.String())
	}

	conflictForkPath := filepath.Join(dirB, "concepts", "note_b.conflict-local.md")
	conflictContent, err := os.ReadFile(conflictForkPath)
	if err != nil {
		t.Fatalf("conflict fork file %s was not created: %v", conflictForkPath, err)
	}
	if string(conflictContent) != "# Note B Conflicting Local Edit\n" {
		t.Fatalf("unexpected content in conflict fork: %s", string(conflictContent))
	}
}
