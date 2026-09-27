package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
	"github.com/okf-memory/okf-agent-memory/pkg/okf/aag"
)

func isAgentsHelpArg(arg string) bool {
	return arg == "--help" || arg == "-h" || arg == "help"
}

func hasAgentsHelpFlag(args []string) bool {
	for _, a := range args {
		if isAgentsHelpArg(a) {
			return true
		}
	}
	return false
}

func cmdAgents(args []string) {
	if len(args) < 1 || hasAgentsHelpFlag(args) && len(args) == 1 {
		printAgentsUsage()
		if len(args) < 1 {
			os.Exit(1)
		}
		return
	}

	subcmd := args[0]
	subArgs := args[1:]

	switch subcmd {
	case "lint":
		if hasAgentsHelpFlag(subArgs) {
			printAgentsLintUsage()
			return
		}
		if err := runAgentsLint(subArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "init":
		if hasAgentsHelpFlag(subArgs) {
			printAgentsInitUsage()
			return
		}
		if err := runAgentsInit(subArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "link":
		if hasAgentsHelpFlag(subArgs) {
			printAgentsLinkUsage()
			return
		}
		if err := runAgentsLink(subArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "check":
		if hasAgentsHelpFlag(subArgs) {
			printAgentsCheckUsage()
			return
		}
		if err := runAgentsCheck(subArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "help", "--help", "-h":
		printAgentsUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown agents command '%s'\n\n", subcmd)
		printAgentsUsage()
		os.Exit(1)
	}
}

func printAgentsLintUsage() {
	fmt.Print(`Lint an AGENTS.md file against AAG rules (AAG-001 to AAG-005) and token budget limits.

Usage:
  okf agents lint [file] [flags]

Arguments:
  [file]                 Path to AGENTS.md file to lint (default: AGENTS.md)

Flags:
  --strict               Treat warnings as fatal errors (exit code 1)
  --budget <int>         Maximum token budget cap (default: 400)
  --json                 Output findings as structured JSON

Examples:
  okf agents lint
  okf agents lint path/to/AGENTS.md --strict
  okf agents lint --budget 300 --json
`)
}

func printAgentsInitUsage() {
	fmt.Print(`Scaffold a curated, domain-specific AGENTS.md codex for a new or existing repository.

Usage:
  okf agents init [flags] [root-dir]

Flags:
  --domain <name>        Domain profile: software, research, legal, coaching, books (default: software)
  --name <name>          Project name (defaults to target directory name)
  --root <path>          Target repository root directory (default: .)
  --force                Overwrite existing AGENTS.md if present

Examples:
  okf agents init --domain=software
  okf agents init --domain=coaching --name="Executive Coaching Practice"
  okf agents init --domain=books --force
`)
}

func printAgentsLinkUsage() {
	fmt.Print(`Create and maintain SSoT tool symlinks from canonical AGENTS.md to editor config files.

Links created:
  - CLAUDE.md                      -> AGENTS.md (Claude Code)
  - .cursorrules                   -> AGENTS.md (Cursor)
  - .windsurfrules                 -> AGENTS.md (Windsurf)
  - .github/copilot-instructions.md -> ../AGENTS.md (GitHub Copilot)

Usage:
  okf agents link [root-dir] [flags]

Flags:
  --check                Verify symlink status without modifying files
  --force                Overwrite existing non-symlink files with symlinks
  --root <path>          Target repository root directory (default: .)
  --json                 Output symlink status as JSON

Examples:
  okf agents link
  okf agents link --check
  okf agents link --force
`)
}

func printAgentsCheckUsage() {
	fmt.Print(`Run an all-in-one CI validation check: AAG rule linting + SSoT tool symlink integrity.

Usage:
  okf agents check [flags]

Flags:
  --root <path>          Repository root containing AGENTS.md (default: .)
  --strict               Treat warnings as fatal errors
  --budget <int>         Token budget cap (default: 400)
  --json                 Output verification report as JSON

Examples:
  okf agents check
  okf agents check --strict
`)
}

func printAgentsUsage() {
	fmt.Printf(`OKF Agents Management & AAG Suite

Usage:
  okf agents <command> [arguments] [flags]

Commands:
  lint [file]    Lint AGENTS.md against AAG-001 to AAG-005 and token budget
  init           Scaffold domain-specific AGENTS.md codex (--domain=software|research|legal|...)
  link           Create SSoT symlinks (CLAUDE.md, .cursorrules, .windsurfrules, Copilot)
  check          Run all-in-one CI validation (lint + symlink integrity)

Flags:
  --domain <name>  Domain profile (software, research, legal, coaching, books)
  --budget <int>   Token budget cap (default: 400 for file, 150 for micro-syntax)
  --strict         Treat warnings as fatal errors
  --force          Overwrite existing files or symlinks
  --check          Verify symlink integrity without mutating
  --json           Emit machine-readable JSON output
`)
}

func runAgentsLint(args []string) error {
	fs := flag.NewFlagSet("agents lint", flag.ContinueOnError)
	strict := fs.Bool("strict", false, "Treat warnings as errors")
	budget := fs.Int("budget", aag.DefaultBudgetLimit, "Max token budget")
	jsonOut := fs.Bool("json", false, "Output results as JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}

	targetFile := "AGENTS.md"
	if len(fs.Args()) > 0 {
		targetFile = fs.Args()[0]
	}

	res, err := aag.LintFile(targetFile, aag.LinterOptions{
		BudgetLimit: *budget,
		Strict:      *strict,
	})
	if err != nil {
		return fmt.Errorf("failed to lint %s: %w", targetFile, err)
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	fmt.Printf("AAG Linter: %s\n", targetFile)
	fmt.Printf("Token Stats: %d estimated tokens (Budget: %d tokens)\n\n",
		res.TokenStats.EstimatedTokens, res.TokenStats.BudgetLimit)

	if len(res.Findings) == 0 {
		fmt.Printf("✓ All AAG rules passed (0 errors, 0 warnings). 100%% conformant.\n")
		return nil
	}

	for _, f := range res.Findings {
		prefix := "WARN"
		if f.Severity == aag.SeverityError {
			prefix = "ERROR"
		}
		fmt.Printf("[%s] %s (line %d): %s\n", prefix, f.RuleID, f.Line, f.Message)
		if f.Snippet != "" {
			fmt.Printf("       > %s\n", f.Snippet)
		}
	}

	fmt.Printf("\nSummary: %d error(s), %d warning(s)\n", res.ErrorCount, res.WarnCount)
	if !res.Passed {
		return fmt.Errorf("AAG linting failed with %d error(s)", res.ErrorCount)
	}
	return nil
}

func runAgentsInit(args []string) error {
	fs := flag.NewFlagSet("agents init", flag.ContinueOnError)
	domain := fs.String("domain", "software", "Domain codex profile")
	name := fs.String("name", "", "Project name")
	root := fs.String("root", ".", "Target repository root directory")
	force := fs.Bool("force", false, "Overwrite existing AGENTS.md")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) > 0 && *root == "." {
		*root = fs.Args()[0]
	}

	targetPath := filepath.Join(*root, "AGENTS.md")
	if _, err := os.Stat(targetPath); err == nil && !*force {
		return fmt.Errorf("AGENTS.md already exists at %s (use --force to overwrite)", targetPath)
	}

	content, err := okf.GenerateAgentsMarkdown(*name, *domain)
	if err != nil {
		return fmt.Errorf("failed to generate AGENTS.md: %w", err)
	}

	if err := os.MkdirAll(*root, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", *root, err)
	}

	if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", targetPath, err)
	}

	fmt.Printf("✓ Created %s with %q domain codex.\n", targetPath, *domain)
	return nil
}

func runAgentsLink(args []string) error {
	fs := flag.NewFlagSet("agents link", flag.ContinueOnError)
	root := fs.String("root", ".", "Target repository root directory")
	force := fs.Bool("force", false, "Overwrite existing non-symlink files")
	checkOnly := fs.Bool("check", false, "Check symlink status without modifying")
	jsonOut := fs.Bool("json", false, "Output results as JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) > 0 && *root == "." {
		*root = fs.Args()[0]
	}

	if *checkOnly {
		statuses, err := okf.CheckToolSymlinks(*root)
		if err != nil {
			return err
		}

		if *jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(statuses)
		}

		hasErrors := false
		fmt.Printf("Checking SSoT tool symlinks in %s:\n\n", *root)
		for _, st := range statuses {
			if st.IsValid {
				fmt.Printf("✓ %-16s %s -> %s\n", st.ToolName, st.ToolPath, st.ActualTarget)
			} else {
				hasErrors = true
				fmt.Printf("✗ %-16s %s (error: %s)\n", st.ToolName, st.ToolPath, st.ErrorMessage)
			}
		}

		if hasErrors {
			return fmt.Errorf("one or more tool symlinks are missing or invalid (run 'okf agents link' to repair)")
		}
		fmt.Printf("\nAll SSoT tool symlinks are valid and drift-free.\n")
		return nil
	}

	results, err := okf.CreateToolSymlinks(*root, *force)
	if err != nil {
		return err
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	fmt.Printf("Created SSoT tool symlinks in %s:\n\n", *root)
	hasErrors := false
	for _, res := range results {
		if res.Error != "" {
			hasErrors = true
			fmt.Printf("✗ %-16s %s (error: %s)\n", res.ToolName, res.Path, res.Error)
		} else if res.Created {
			action := "created"
			if res.Overwritten {
				action = "overwritten"
			}
			fmt.Printf("✓ %-16s %s -> %s (%s)\n", res.ToolName, res.Path, res.Target, action)
		} else {
			fmt.Printf("✓ %-16s %s -> %s (already valid)\n", res.ToolName, res.Path, res.Target)
		}
	}

	if hasErrors {
		return fmt.Errorf("failed to create one or more symlinks")
	}
	return nil
}

func runAgentsCheck(args []string) error {
	fs := flag.NewFlagSet("agents check", flag.ContinueOnError)
	root := fs.String("root", ".", "Target repository root directory")
	strict := fs.Bool("strict", true, "Treat warnings as errors")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) > 0 && *root == "." {
		*root = fs.Args()[0]
	}

	agentsFile := filepath.Join(*root, "AGENTS.md")
	fmt.Printf("==> 1. Linting AAG rules in %s...\n", agentsFile)
	if err := runAgentsLint([]string{agentsFile, fmt.Sprintf("--strict=%t", *strict)}); err != nil {
		return err
	}

	fmt.Printf("\n==> 2. Verifying SSoT tool symlinks in %s...\n", *root)
	if err := runAgentsLink([]string{"--root", *root, "--check"}); err != nil {
		return err
	}

	fmt.Printf("\n✓ All agent governance and symlink integrity checks passed!\n")
	return nil
}
