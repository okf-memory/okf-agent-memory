package cli

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/okf-memory/okf-agent-memory/pkg/sync"
)

func cmdHub(args []string) {
	if len(args) < 1 {
		printHubUsage()
		os.Exit(1)
	}

	subcmd := args[0]
	subargs := args[1:]

	switch subcmd {
	case "init-vault":
		fs := flag.NewFlagSet("hub init-vault", flag.ExitOnError)
		hubURL := fs.String("hub", "", "Hub server URL (default from .okf-vault.json or http://127.0.0.1:8080)")
		authToken := fs.String("auth-token", "", "Optional Hub authentication Bearer token")
		_ = fs.Parse(subargs)

		dir := "."
		if fs.NArg() > 0 {
			dir = fs.Arg(0)
		}
		if err := sync.InitVault(os.Stdout, dir, *hubURL, *authToken); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "push":
		fs := flag.NewFlagSet("hub push", flag.ExitOnError)
		hubURL := fs.String("hub", "", "Hub server URL (default from .okf-vault.json or http://127.0.0.1:8080)")
		authToken := fs.String("auth-token", "", "Hub authentication Bearer token (or OKF_HUB_TOKEN env)")
		password := fs.String("password", "", "Master password")
		secretKey := fs.String("secret-key", "", "Secret key")
		msg := fs.String("message", "CLI push", "Commit message")
		_ = fs.Parse(subargs)

		dir := "."
		if fs.NArg() > 0 {
			dir = fs.Arg(0)
		}

		cfg, err := sync.LoadVaultConfig(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v (run 'okf hub init-vault' first)\n", err)
			os.Exit(1)
		}

		client := sync.NewClient(sync.ResolveHubURL(*hubURL, cfg), sync.ResolveToken(*authToken, cfg))
		op := sync.HubOp{
			Dir:          dir,
			Client:       client,
			VaultID:      cfg.VaultID,
			Password:     *password,
			SecretKey:    *secretKey,
			Message:      *msg,
			AgentVersion: Version,
		}
		if err := sync.HubPush(os.Stdout, op); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "pull":
		fs := flag.NewFlagSet("hub pull", flag.ExitOnError)
		hubURL := fs.String("hub", "", "Hub server URL (default from .okf-vault.json or http://127.0.0.1:8080)")
		authToken := fs.String("auth-token", "", "Hub authentication Bearer token (or OKF_HUB_TOKEN env)")
		password := fs.String("password", "", "Master password")
		secretKey := fs.String("secret-key", "", "Secret key")
		_ = fs.Parse(subargs)

		dir := "."
		if fs.NArg() > 0 {
			dir = fs.Arg(0)
		}

		cfg, err := sync.LoadVaultConfig(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v (run 'okf hub init-vault' first)\n", err)
			os.Exit(1)
		}

		client := sync.NewClient(sync.ResolveHubURL(*hubURL, cfg), sync.ResolveToken(*authToken, cfg))
		op := sync.HubOp{
			Dir:       dir,
			Client:    client,
			VaultID:   cfg.VaultID,
			Password:  *password,
			SecretKey: *secretKey,
		}
		if err := sync.HubPull(os.Stdout, op); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "sync":
		fs := flag.NewFlagSet("hub sync", flag.ExitOnError)
		hubURL := fs.String("hub", "", "Hub server URL (default from .okf-vault.json or http://127.0.0.1:8080)")
		authToken := fs.String("auth-token", "", "Hub authentication Bearer token (or OKF_HUB_TOKEN env)")
		password := fs.String("password", "", "Master password")
		secretKey := fs.String("secret-key", "", "Secret key")
		msg := fs.String("message", "CLI sync", "Commit message")
		_ = fs.Parse(subargs)

		dir := "."
		if fs.NArg() > 0 {
			dir = fs.Arg(0)
		}

		cfg, err := sync.LoadVaultConfig(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v (run 'okf hub init-vault' first)\n", err)
			os.Exit(1)
		}

		client := sync.NewClient(sync.ResolveHubURL(*hubURL, cfg), sync.ResolveToken(*authToken, cfg))
		op := sync.HubOp{
			Dir:          dir,
			Client:       client,
			VaultID:      cfg.VaultID,
			Password:     *password,
			SecretKey:    *secretKey,
			Message:      *msg,
			AgentVersion: Version,
		}
		if err := sync.HubSync(os.Stdout, op); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "serve":
		fs := flag.NewFlagSet("hub serve", flag.ExitOnError)
		port := fs.Int("port", 8080, "Port to listen on")
		storage := fs.String("storage", "", "Path to storage directory (empty for in-memory)")
		_ = fs.Parse(subargs)

		srv := sync.NewServer(*storage)
		addr := fmt.Sprintf(":%d", *port)
		fmt.Printf("Starting OKF Memory Hub server on http://localhost%s ...\n", addr)
		httpServer := &http.Server{
			Addr:              addr,
			Handler:           srv.Handler(),
			ReadHeaderTimeout: 10 * time.Second,
		}
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown hub command '%s'\n\n", subcmd)
		printHubUsage()
		os.Exit(1)
	}
}

func printHubUsage() {
	fmt.Print(`OKF Memory Hub Commands:

Usage:
  okf hub <command> [arguments] [flags]

Commands:
  init-vault [bundle]      Initialize a new zero-knowledge vault and print Emergency Kit
  push [bundle]            Push local changes to the remote hub
  pull [bundle]            Pull latest remote changes into the local bundle
  sync [bundle]            Pull and push with automated conflict reconciliation
  serve [--port 8080]      Run embedded blind CAS server for self-hosting and testing

Flags (push, pull, sync, init-vault):
  -hub <url>               Hub server URL (default from .okf-vault.json or http://127.0.0.1:8080)
  -auth-token <token>      Hub authentication Bearer token (or OKF_HUB_TOKEN env)
  -password <pass>         Master password
  -secret-key <key>        Secret key
  -message <msg>           Commit message (push, sync)
`)
}
