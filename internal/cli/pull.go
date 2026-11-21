package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/okf-memory/okf-agent-memory/pkg/lock"
	"github.com/okf-memory/okf-agent-memory/pkg/okf"
	"github.com/okf-memory/okf-agent-memory/pkg/registry"
)

var newRegistryClient = func(baseURL string) *registry.Client {
	return registry.NewClient(baseURL)
}

func printPullUsage() {
	fmt.Println(`Usage:
  okf pull [<bundle-id|git-url>] [flags]

Pulls and installs an external knowledge bundle into .okf/vendor/ and updates okf.lock.
If no bundle is specified, restores all bundles declared in okf.lock.

Flags:
  --registry <url>   Override canonical registry endpoint (default: https://registry.okf-memory.dev)
  --min-score <N>    Gate minimum bundle quality score (0-100; aborts installation if below)
  --force            Overwrite existing vendor installation if present

Examples:
  okf pull
  okf pull nextjs-15
  okf pull nextjs-15 --min-score 80
  okf pull @peter/django-5-rules
  okf pull peter/django-5-rules@1.0.0
  okf pull github.com/acme/agent-rules@v1.0.0`)
}

func cmdPull(args []string) {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)
	registryURL := fs.String("registry", registry.DefaultRegistryURL, "Registry endpoint")
	minScore := fs.Int("min-score", 0, "Gate minimum bundle quality score (0-100)")
	force := fs.Bool("force", false, "Overwrite existing vendor installation")
	fs.Usage = printPullUsage

	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	if *minScore < 0 || *minScore > 100 {
		fmt.Fprintf(os.Stderr, "Error: --min-score must be an integer between 0 and 100\n")
		exitFunc(2)
		return
	}

	if fs.NArg() < 1 {
		if _, err := os.Stat("okf.lock"); os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "Error: okf.lock not found. Specify a bundle to pull (e.g. 'okf pull <bundle-id>') or run in a workspace with an existing okf.lock.")
			exitFunc(1)
			return
		}
		runRestore(*registryURL, *force, *minScore)
		return
	}

	target := fs.Arg(0)
	client := newRegistryClient(*registryURL)

	fmt.Printf("Resolving %s from %s...\n", target, client.BaseURL)
	manifest, err := client.Resolve(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitFunc(1)
		return
	}

	if manifest.Version == "main" && (strings.HasPrefix(target, "github.com/") || strings.HasPrefix(target, "https://")) {
		fmt.Fprintf(os.Stderr, "Warning: Pulling floating branch 'main'. Pin to @vX.Y.Z for reproducible builds.\n")
	}

	// The manifest id comes from the registry: it must resolve inside .okf/vendor
	// before anything is installed there or rolled back from there.
	vendorDisplay := filepath.Join(".okf", "vendor", filepath.FromSlash(manifest.ID))
	vendorDir, err := registry.ResolveVendorDir(".", manifest.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: refusing to install bundle: %v\n", err)
		exitFunc(1)
		return
	}
	if _, err := os.Stat(vendorDir); err == nil && !*force {
		fmt.Printf("Bundle %q is already installed in %s. Use --force to reinstall.\n", manifest.ID, vendorDisplay)
		return
	}

	fmt.Printf("Downloading %s (version: %s)...\n", manifest.ID, manifest.Version)
	// Extract and validate in a fresh staging directory: rollback discards only
	// what this invocation created, never an existing directory.
	staging, err := registry.NewStaging(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: refusing to install bundle: %v\n", err)
		exitFunc(1)
		return
	}
	if err := client.DownloadAndExtract(manifest, staging.Dir); err != nil {
		_ = staging.Discard()
		fmt.Fprintf(os.Stderr, "Download error: %v\n", err)
		exitFunc(1)
		return
	}

	// Post-install validation: ensure installed bundle conforms to OKF and passes quality gate
	rollback := func() {
		if err := staging.Discard(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: rollback refused: %v\n", err)
		}
	}

	if _, err := os.Stat(filepath.Join(staging.Dir, "index.md")); os.IsNotExist(err) {
		rollback()
		fmt.Fprintf(os.Stderr, "Error: bundle %q is not a valid OKF bundle: missing index.md (installation rolled back)\n", manifest.ID)
		exitFunc(1)
		return
	}

	vb, err := okf.LoadBundle(staging.Dir)
	if err != nil {
		rollback()
		fmt.Fprintf(os.Stderr, "Error: bundle %q failed to load: %v (installation rolled back)\n", manifest.ID, err)
		exitFunc(1)
		return
	}

	diag := okf.Validate(vb, okf.ValidateOptions{
		Strict:   true,
		MinScore: *minScore,
	})
	if !diag.IsConformant || !diag.GatePassed {
		rollback()
		if *minScore > 0 && diag.Scorecard != nil && diag.Scorecard.Score < *minScore {
			fmt.Fprintf(os.Stderr, "Error: bundle %q failed quality gate: score %d is below required minimum %d (Grade: %s); installation rolled back\n",
				manifest.ID, diag.Scorecard.Score, *minScore, diag.Scorecard.Grade)
		} else {
			fmt.Fprintf(os.Stderr, "Error: bundle %q failed strict validation (%d errors, %d broken links); installation rolled back\n",
				manifest.ID, len(diag.Errors), len(diag.BrokenLinks))
		}
		exitFunc(1)
		return
	}

	if err := staging.Commit(".", manifest.ID); err != nil {
		rollback()
		fmt.Fprintf(os.Stderr, "Error: bundle %q could not be installed: %v (installation rolled back)\n", manifest.ID, err)
		exitFunc(1)
		return
	}

	lockPath := "okf.lock"
	lf, err := lock.ReadLockfile(lockPath)
	if err != nil {
		lf = &lock.Lockfile{Version: 1}
	}
	lf.Upsert(lock.BundleEntry{
		ID:          manifest.ID,
		Source:      target,
		Version:     manifest.Version,
		Hash:        manifest.Hash,
		InstalledAt: time.Now().UTC(),
	})
	if err := lock.WriteLockfile(lockPath, lf); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to update okf.lock: %v\n", err)
	}

	if diag.Scorecard != nil {
		fmt.Printf("Bundle quality score: %d/100 (Grade: %s)\n", diag.Scorecard.Score, diag.Scorecard.Grade)
	}
	fmt.Printf("Successfully installed and verified %s in %s\n", manifest.ID, vendorDisplay)
}
