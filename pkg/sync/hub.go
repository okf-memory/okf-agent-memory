package sync

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/okf-memory/okf-agent-memory/pkg/vault"
)

// VaultConfig stores local configuration for syncing an OKF bundle.
type VaultConfig struct {
	VaultID   string `json:"vault_id"`
	HubURL    string `json:"hub_url,omitempty"`
	AuthToken string `json:"auth_token,omitempty"`
}

const vaultConfigFileName = ".okf-vault.json"

// ResolveHubURL returns the hub URL from flag, config, or default.
func ResolveHubURL(flagURL string, cfg *VaultConfig) string {
	if flagURL != "" {
		return flagURL
	}
	if cfg != nil && cfg.HubURL != "" {
		return cfg.HubURL
	}
	return "http://127.0.0.1:8080"
}

// ResolveToken returns the auth token from flag, env, config, or empty.
func ResolveToken(flagToken string, cfg *VaultConfig) string {
	if flagToken != "" {
		return flagToken
	}
	if env := os.Getenv("OKF_HUB_TOKEN"); env != "" {
		return env
	}
	if cfg != nil && cfg.AuthToken != "" {
		return cfg.AuthToken
	}
	return ""
}

// LoadVaultConfig reads vault configuration from the bundle directory.
func LoadVaultConfig(dir string) (*VaultConfig, error) {
	cfgPath := filepath.Join(dir, vaultConfigFileName)
	// #nosec G304 -- cfgPath is within user bundle root
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", cfgPath, err)
	}
	var cfg VaultConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", cfgPath, err)
	}
	return &cfg, nil
}

// SaveVaultConfig writes vault configuration to the bundle directory.
func SaveVaultConfig(dir string, cfg *VaultConfig) error {
	cfgPath := filepath.Join(dir, vaultConfigFileName)
	// #nosec G117 -- local vault configuration optionally stores user hub bearer token
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	// #nosec G703,G306 -- cfgPath is within user bundle root
	return os.WriteFile(cfgPath, data, 0o644)
}

// InitVault generates a new vault ID and secret key, saves config, and prints the emergency kit.
func InitVault(w io.Writer, dir, hubURL, authToken string) error {
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return fmt.Errorf("failed to generate random vault id: %w", err)
	}
	vaultID := "v_" + hex.EncodeToString(idBytes)

	secretKey, err := vault.GenerateSecretKey()
	if err != nil {
		return fmt.Errorf("failed to generate secret key: %w", err)
	}

	if hubURL == "" {
		hubURL = "http://127.0.0.1:8080"
	}

	cfg := &VaultConfig{
		VaultID:   vaultID,
		HubURL:    hubURL,
		AuthToken: authToken,
	}
	if err := SaveVaultConfig(dir, cfg); err != nil {
		return fmt.Errorf("failed to save %s: %w", vaultConfigFileName, err)
	}

	_, _ = fmt.Fprintf(w, `
================================================================================
                       OKF MEMORY HUB — EMERGENCY KIT
================================================================================
Vault ID:   %s
Secret Key: %s

CRITICAL WARNING:
Your knowledge bundle is encrypted client-side using zero-knowledge AES-256-GCM.
If you lose your master password and this Secret Key, your data stored on the
Hub CANNOT be recovered by anyone. Store this Emergency Kit in a safe place.
================================================================================
`, vaultID, secretKey)

	return nil
}

// HubOp configures a remote hub push, pull, or sync operation.
type HubOp struct {
	Dir          string
	Client       *Client
	VaultID      string
	Password     string
	SecretKey    string
	Message      string // used by HubPush and HubSync
	AgentVersion string // used by HubPush and HubSync
}

// HubPush pushes local changes to the remote hub.
func HubPush(w io.Writer, op HubOp) error {
	vaultKey, err := vault.DeriveVaultKey(op.Password, op.SecretKey)
	if err != nil {
		return fmt.Errorf("failed to derive vault key: %w", err)
	}

	engine := NewEngine(op.Client, op.VaultID, vaultKey, op.Dir)
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "local-device"
	}
	author := vault.CommitAuthor{
		ClientID: hostname,
		Agent:    "okf-cli/" + op.AgentVersion,
	}

	res, err := engine.Push(context.Background(), author, op.Message)
	if err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	_, _ = fmt.Fprintf(w, "Push completed: commit %s (uploaded %d, unchanged %d)\n", res.CommitHash, res.UploadedBlobs, res.UnchangedBlobs)
	return nil
}

// HubPull pulls latest remote changes into the local bundle.
func HubPull(w io.Writer, op HubOp) error {
	vaultKey, err := vault.DeriveVaultKey(op.Password, op.SecretKey)
	if err != nil {
		return fmt.Errorf("failed to derive vault key: %w", err)
	}

	engine := NewEngine(op.Client, op.VaultID, vaultKey, op.Dir)
	res, err := engine.Pull(context.Background())
	if err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}

	_, _ = fmt.Fprintf(w, "Pull completed: commit %s (updated %d, deleted %d)\n", res.CommitHash, len(res.UpdatedFiles), len(res.DeletedFiles))
	return nil
}

// HubSync performs a pull-then-push with automated conflict reconciliation.
func HubSync(w io.Writer, op HubOp) error {
	vaultKey, err := vault.DeriveVaultKey(op.Password, op.SecretKey)
	if err != nil {
		return fmt.Errorf("failed to derive vault key: %w", err)
	}

	engine := NewEngine(op.Client, op.VaultID, vaultKey, op.Dir)
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "local-device"
	}
	author := vault.CommitAuthor{
		ClientID: hostname,
		Agent:    "okf-cli/" + op.AgentVersion,
	}

	res, err := engine.Sync(context.Background(), author, op.Message)
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	if len(res.Conflicts) > 0 {
		for _, c := range res.Conflicts {
			forked := ConflictLocalPath(c.Path)
			_, _ = fmt.Fprintf(w, "⚠️  Collision detected at %s. Local version saved as %s.\n", c.Path, forked)
		}
	}

	_, _ = fmt.Fprintf(w, "Sync completed: commit %s\n", res.CommitHash)
	return nil
}
