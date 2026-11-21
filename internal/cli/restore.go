package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/okf-memory/okf-agent-memory/pkg/lock"
	"github.com/okf-memory/okf-agent-memory/pkg/okf"
	"github.com/okf-memory/okf-agent-memory/pkg/registry"
)

func printRestoreUsage() {
	fmt.Println(`Usage:
  okf restore [flags]

Restores all vendor bundles specified in okf.lock.

Flags:
  --registry <url>   Override canonical registry endpoint (default: https://registry.okf-memory.dev)
  --min-score <N>    Gate minimum bundle quality score (0-100; aborts installation if below)
  --force            Reinstall all bundles even if already present

Examples:
  okf restore
  okf restore --min-score 80
  okf restore --force`)
}

func cmdRestore(args []string) {
	fs := flag.NewFlagSet("restore", flag.ExitOnError)
	registryURL := fs.String("registry", registry.DefaultRegistryURL, "Registry endpoint")
	minScore := fs.Int("min-score", 0, "Gate minimum bundle quality score (0-100)")
	force := fs.Bool("force", false, "Reinstall existing vendor bundles")
	fs.Usage = printRestoreUsage

	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	if *minScore < 0 || *minScore > 100 {
		fmt.Fprintf(os.Stderr, "Error: --min-score must be an integer between 0 and 100\n")
		exitFunc(2)
		return
	}

	if _, err := os.Stat("okf.lock"); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "Error: okf.lock not found. Run 'okf pull <bundle-id>' to install a bundle.")
		exitFunc(1)
		return
	}

	runRestore(*registryURL, *force, *minScore)
}

func runRestore(registryURL string, force bool, minScores ...int) {
	minScore := 0
	if len(minScores) > 0 {
		minScore = minScores[0]
	}

	lf, err := lock.ReadLockfile("okf.lock")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading okf.lock: %v\n", err)
		exitFunc(1)
		return
	}

	if lf == nil || len(lf.Bundles) == 0 {
		fmt.Println("No vendor bundles specified in okf.lock.")
		return
	}

	client := newRegistryClient(registryURL)
	restoredCount := 0
	skippedCount := 0

	for _, entry := range lf.Bundles {
		// okf.lock may come from a repository: its ids must resolve inside
		// .okf/vendor before anything is installed there or rolled back from there.
		vendorDisplay := filepath.Join(".okf", "vendor", filepath.FromSlash(entry.ID))
		vendorDir, err := registry.ResolveVendorDir(".", entry.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: refusing to restore okf.lock entry: %v\n", err)
			exitFunc(1)
			return
		}
		indexFile := filepath.Join(vendorDir, "index.md")

		if _, err := os.Stat(indexFile); err == nil && !force {
			fmt.Printf("Bundle %q is already installed in %s (skipping)\n", entry.ID, vendorDisplay)
			skippedCount++
			continue
		}

		source := entry.Source
		if source == "" {
			source = entry.ID
		}
		if entry.Version != "" && !strings.Contains(source, "@") && entry.Version != "main" {
			source = fmt.Sprintf("%s@%s", source, entry.Version)
		}

		fmt.Printf("Resolving %s from %s...\n", entry.ID, client.BaseURL)
		manifest, err := client.Resolve(source)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving %s: %v\n", entry.ID, err)
			exitFunc(1)
			return
		}

		if entry.Hash != "" {
			manifest.Hash = entry.Hash
		}

		fmt.Printf("Downloading %s (version: %s)...\n", manifest.ID, manifest.Version)
		// Extract and validate in a fresh staging directory: rollback discards
		// only what this invocation created, never an existing directory.
		staging, err := registry.NewStaging(".")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: refusing to restore %s: %v\n", entry.ID, err)
			exitFunc(1)
			return
		}
		if err := client.DownloadAndExtract(manifest, staging.Dir); err != nil {
			_ = staging.Discard()
			fmt.Fprintf(os.Stderr, "Download error for %s: %v\n", manifest.ID, err)
			exitFunc(1)
			return
		}

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
			MinScore: minScore,
		})
		if !diag.IsConformant || !diag.GatePassed {
			rollback()
			if minScore > 0 && diag.Scorecard != nil && diag.Scorecard.Score < minScore {
				fmt.Fprintf(os.Stderr, "Error: bundle %q failed quality gate: score %d is below required minimum %d (Grade: %s); installation rolled back\n",
					manifest.ID, diag.Scorecard.Score, minScore, diag.Scorecard.Grade)
			} else {
				fmt.Fprintf(os.Stderr, "Error: bundle %q failed strict validation (%d errors, %d broken links); installation rolled back\n",
					manifest.ID, len(diag.Errors), len(diag.BrokenLinks))
			}
			exitFunc(1)
			return
		}

		if err := staging.Commit(".", entry.ID); err != nil {
			rollback()
			fmt.Fprintf(os.Stderr, "Error: bundle %q could not be restored: %v (installation rolled back)\n", manifest.ID, err)
			exitFunc(1)
			return
		}

		if diag.Scorecard != nil {
			fmt.Printf("Bundle quality score: %d/100 (Grade: %s)\n", diag.Scorecard.Score, diag.Scorecard.Grade)
		}
		fmt.Printf("Successfully restored %s in %s\n", manifest.ID, vendorDisplay)
		restoredCount++
	}

	fmt.Printf("Restore complete: %d restored, %d skipped.\n", restoredCount, skippedCount)
}
