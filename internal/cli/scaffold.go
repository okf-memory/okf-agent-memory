package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

// ============================================================================
// INIT & BOOTSTRAP
// ============================================================================

func printInitUsage() {
	fmt.Printf(`Initialize a new OKF v0.2 bundle with root index.md and log.md.

Usage:
  okf init [path]

Arguments:
  [path]                 Target directory to initialize (default: 'knowledge' or '.')

Examples:
  okf init knowledge
  okf init .
`)
}

func printBootstrapUsage() {
	fmt.Printf(`Scaffold a complete OKF Agent Memory stack in a project repository.

Usage:
  okf bootstrap [target-dir] [flags]

Arguments:
  [target-dir]           Target repository root directory (default: '.')

Flags:
  --name <name>          Project name (defaults to target directory name)
  --no-skill             Skip installing .agents/skills/okf-memory
  --no-agents-md         Skip installing canonical AGENTS.md
  --overwrite-agents-md  Overwrite existing AGENTS.md instead of smart append
  --no-makefile          Skip installing Makefile
  --no-bundle            Skip installing knowledge/ scaffold (index.md, log.md)

Examples:
  okf bootstrap .
  okf bootstrap /path/to/project --name "My Project"
`)
}

func cmdInit(args []string) {
	if hasHelpFlag(args) {
		printInitUsage()
		return
	}
	bundleDir, _ := defaultBundle(args)
	if err := okf.InitBundle(bundleDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing bundle: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Initialized OKF v0.2 bundle in '%s'\n", bundleDir)
}

func cmdBootstrap(args []string) {
	if hasHelpFlag(args) {
		printBootstrapUsage()
		return
	}
	targetDir, subArgs := splitOptionalPath(args, ".")

	fs := flag.NewFlagSet("bootstrap", flag.ExitOnError)
	name := fs.String("name", "", "Project name (defaults to target directory name)")
	noSkill := fs.Bool("no-skill", false, "Skip installing .agents/skills/okf-memory")
	noAgentsMD := fs.Bool("no-agents-md", false, "Skip installing AGENTS.md")
	overwriteAgentsMD := fs.Bool("overwrite-agents-md", false, "Overwrite existing AGENTS.md instead of smart append")
	noMakefile := fs.Bool("no-makefile", false, "Skip installing Makefile")
	noBundle := fs.Bool("no-bundle", false, "Skip installing knowledge/ scaffold")
	_ = fs.Parse(subArgs)

	opts := okf.BootstrapOptions{
		ProjectName:       *name,
		InstallSkill:      !*noSkill,
		InstallAgentsMD:   !*noAgentsMD,
		OverwriteAgentsMD: *overwriteAgentsMD,
		InstallMakefile:   !*noMakefile,
		InstallBundle:     !*noBundle,
	}

	if err := okf.Bootstrap(targetDir, opts); err != nil {
		fmt.Fprintf(os.Stderr, "Bootstrap error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully bootstrapped OKF Agent Memory in '%s'!\n", targetDir)
	fmt.Println("Created:")
	if opts.InstallBundle {
		fmt.Println("  - knowledge/ (index.md, log.md)")
	}
	if opts.InstallSkill {
		fmt.Println("  - .agents/skills/okf-memory/ (SKILL.md, discovery, update, etc.)")
	}
	if opts.InstallAgentsMD {
		fmt.Println("  - AGENTS.md")
	}
	if opts.InstallMakefile {
		fmt.Println("  - Makefile")
	}
	fmt.Println("\nRun 'okf validate knowledge' or 'make validate' to verify.")
}

// ============================================================================
// MCP SERVER (CLI ADAPTER)
// ============================================================================

func printMCPUsage() {
	fmt.Printf(`Run OKF Agent Memory as a Model Context Protocol (MCP) server over stdio.

Usage:
  okf mcp [bundle]

Arguments:
  [bundle]               Path to OKF bundle directory (default: 'knowledge' or '.')

Examples:
  okf mcp knowledge
  okf mcp .
`)
}

func cmdMCP(args []string) {
	if hasHelpFlag(args) {
		printMCPUsage()
		return
	}
	bundleDir, _ := defaultBundle(args)
	okf.SetMCPVersion(Version)
	if err := okf.RunMCPServer(bundleDir); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}
