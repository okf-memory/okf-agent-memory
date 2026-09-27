package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

// ============================================================================
// VALIDATE
// ============================================================================

func printValidateUsage() {
	fmt.Printf(`Validate an OKF knowledge bundle and agent workspace for conformance and graph health.

Usage:
  okf validate [bundle] [flags]

Arguments:
  [bundle]               Path to OKF bundle directory or workspace root (default: 'knowledge' or '.')

Flags:
  --strict               Gate connectivity warnings, broken links, orphans, and trust gaps as errors
  --drift                Check descriptions and code_refs for drift between index.md and concepts
  --stale                Gate expired review dates (stale_after) as errors
  --stale-within <dur>   Gate concepts becoming stale within relative duration (e.g. '14d', '2w', '3m')
  --agents               Validate AGENTS.md against AAG rules (AAG-001 to AAG-005) and SSoT tool symlinks
  --json                 Output validation results as structured JSON

Examples:
  okf validate knowledge --strict --drift
  okf validate --agents --strict .
  okf validate knowledge --stale-within 14d
  okf validate knowledge --json
`)
}

func cmdValidate(args []string) {
	if hasHelpFlag(args) {
		printValidateUsage()
		return
	}
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	strict := fs.Bool("strict", false, "Fail on broken links, orphans, and provenance gaps")
	drift := fs.Bool("drift", false, "Check for drift between index.md and concept descriptions")
	stale := fs.Bool("stale", false, "Fail if any concepts are stale (past stale_after)")
	staleWithinStr := fs.String("stale-within", "", "Gate concepts becoming stale within relative duration (e.g. '14d', '2w')")
	agents := fs.Bool("agents", false, "Validate AGENTS.md against AAG rules and SSoT tool symlinks")
	jsonOut := fs.Bool("json", false, "Output results as JSON")

	bundleDir, flagArgs := defaultBundle(args)
	_ = fs.Parse(flagArgs)
	if len(fs.Args()) > 0 {
		bundleDir = fs.Args()[0]
	}

	var staleWithin time.Duration
	if *staleWithinStr != "" {
		d, err := okf.ParseRelativeDuration(*staleWithinStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid --stale-within duration: %v\n", err)
			os.Exit(1)
		}
		staleWithin = d
	}

	if *agents {
		agentsRoot := filepath.Clean(bundleDir)
		// #nosec G703 -- agentsRoot is sanitized and checked for existence of AGENTS.md
		if info, err := os.Stat(filepath.Join(agentsRoot, "AGENTS.md")); err != nil || !info.Mode().IsRegular() {
			parent := filepath.Dir(agentsRoot)
			// #nosec G703 -- parent is derived from sanitized path
			if pInfo, pErr := os.Stat(filepath.Join(parent, "AGENTS.md")); pErr == nil && pInfo.Mode().IsRegular() {
				agentsRoot = parent
			}
		}
		if err := runAgentsCheck([]string{"--root", agentsRoot, fmt.Sprintf("--strict=%t", *strict)}); err != nil {
			fmt.Fprintf(os.Stderr, "\nAgents governance check failed: %v\n", err)
			os.Exit(1)
		}
	}

	b, err := okf.LoadBundle(bundleDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading bundle: %v\n", err)
		os.Exit(2)
	}

	res := okf.Validate(b, okf.ValidateOptions{
		Strict:      *strict,
		Drift:       *drift,
		Stale:       *stale,
		StaleWithin: staleWithin,
	})

	if *jsonOut {
		data, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(data))
		if !res.GatePassed {
			os.Exit(1)
		}
		return
	}

	printValidationOutput(os.Stdout, res, bundleDir, *strict, *stale, *staleWithinStr)

	if !res.IsConformant || !res.GatePassed {
		os.Exit(1)
	}
}

func printValidationOutput(w io.Writer, res *okf.ValidationResult, bundleDir string, strict, stale bool, staleWithinOpt ...string) {
	p := func(format string, a ...any) {
		_, _ = fmt.Fprintf(w, format, a...)
	}
	pln := func(a ...any) {
		_, _ = fmt.Fprintln(w, a...)
	}

	for _, wMsg := range res.Warnings {
		p("warn  %s\n", wMsg)
	}
	for _, g := range res.GateFindings {
		prefix := "warn  "
		if strict {
			prefix = "gate  "
		}
		p("%s%s\n", prefix, g)
	}
	for _, bl := range res.BrokenLinks {
		prefix := "warn  "
		if strict {
			prefix = "gate  "
		}
		p("%s%s: broken concept link -> %s (%s)\n", prefix, bl.SourceConcept, bl.TargetHref, bl.Reason)
	}
	for _, o := range res.Orphans {
		prefix := "warn  "
		if strict {
			prefix = "gate  "
		}
		p("%s%s.md: orphan (no concept links in or out)\n", prefix, o)
	}
	for _, e := range res.Errors {
		p("error %s\n", e)
	}

	verStr := "no declared version"
	if res.DeclaredVer != "" {
		verStr = "v" + res.DeclaredVer
	}

	flagSummary := ""
	if strict {
		flagSummary += " [--strict]"
	}
	if stale {
		flagSummary += " [--stale]"
	}
	if len(staleWithinOpt) > 0 && staleWithinOpt[0] != "" {
		flagSummary += fmt.Sprintf(" [--stale-within %s]", staleWithinOpt[0])
	}

	gateCount := len(res.GateFindings) + len(res.BrokenLinks) + len(res.Orphans)
	warnCount := len(res.Warnings)
	if strict {
		p("\nOKF v0.2 check of \"%s\" (%s): %d concept(s), %d error(s), %d gate finding(s), %d warning(s); %d broken link(s), %d orphan(s), %d stale%s. ",
			bundleDir, verStr, res.ConceptCount, len(res.Errors), gateCount, warnCount, len(res.BrokenLinks), len(res.Orphans), res.StaleCount, flagSummary)
	} else {
		warnCount += gateCount
		p("\nOKF v0.2 check of \"%s\" (%s): %d concept(s), %d error(s), %d warning(s); %d broken link(s), %d orphan(s), %d stale%s. ",
			bundleDir, verStr, res.ConceptCount, len(res.Errors), warnCount, len(res.BrokenLinks), len(res.Orphans), res.StaleCount, flagSummary)
	}

	switch {
	case !res.IsConformant:
		pln("NOT conformant.")
	case !res.GatePassed:
		pln("Conformant, but the producer gate failed.")
	default:
		pln("Conformant.")
	}
}
