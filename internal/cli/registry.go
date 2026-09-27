package cli

import (
	"fmt"
	"io"
	"os"
)

// Command defines a CLI subcommand with usage and execution handlers.
type Command struct {
	Name       string
	Aliases    []string
	Summary    string
	PrintUsage func()
	Run        func(args []string)
}

var (
	commands []*Command
	registry = make(map[string]*Command)
)

// Register registers a command into the global CLI command registry.
func Register(c *Command) {
	commands = append(commands, c)
	registry[c.Name] = c
	for _, alias := range c.Aliases {
		registry[alias] = c
	}
}

// FindCommand looks up a command by name or alias.
func FindCommand(name string) (*Command, bool) {
	c, ok := registry[name]
	return c, ok
}

func renderUsageTo(w io.Writer) {
	p := func(format string, a ...any) {
		_, _ = fmt.Fprintf(w, format, a...)
	}

	p("OKF Agent Memory CLI (v%s)\n\n", Version)
	p("Usage:\n  okf <command> [arguments] [flags]\n\n")
	p("Commands:\n")
	for _, c := range commands {
		p("  %-22s %s\n", c.Name, c.Summary)
	}
	p("  %-22s %s\n", "version", "Print version information")
	p("  %-22s %s\n\n", "help", "Show this help message")

	p(`Flags:
  --for-path <path>      Filter concepts governing a file path via code_refs (search)
  --filter <expr>        Filter concepts by frontmatter key-value predicates (search)
  --stale-within <dur>   Filter or gate concepts becoming stale within relative duration
  --json                 Emit machine-readable JSON output
  --strict               Gate connectivity warnings and trust gaps as errors in validate
  --drift                Check descriptions and code_refs for drift in validate
  --stale                Gate expired review dates (stale_after) as errors in validate

`)
}

func printUsage() {
	renderUsageTo(os.Stdout)
}

func init() {
	Register(&Command{
		Name:       "validate",
		Summary:    "Validate an OKF bundle for conformance and graph health",
		PrintUsage: printValidateUsage,
		Run:        cmdValidate,
	})
	Register(&Command{
		Name:       "search",
		Summary:    "Search concepts using in-memory BM25 scoring or frontmatter filter",
		PrintUsage: printSearchUsage,
		Run:        cmdSearch,
	})
	Register(&Command{
		Name:       "show",
		Summary:    "Display full concept details, frontmatter, and links",
		PrintUsage: printShowUsage,
		Run:        cmdShow,
	})
	Register(&Command{
		Name:       "create",
		Summary:    "Create a new concept with automated bookkeeping",
		PrintUsage: printCreateUsage,
		Run:        cmdCreate,
	})
	Register(&Command{
		Name:       "update",
		Summary:    "Update an existing concept",
		PrintUsage: printUpdateUsage,
		Run:        cmdUpdate,
	})
	Register(&Command{
		Name:       "relate",
		Summary:    "Connect two concepts with a relative link and context",
		PrintUsage: printRelateUsage,
		Run:        cmdRelate,
	})
	Register(&Command{
		Name:       "init",
		Summary:    "Initialize a new OKF v0.2 bundle (index.md, log.md)",
		PrintUsage: printInitUsage,
		Run:        cmdInit,
	})
	Register(&Command{
		Name:       "bootstrap",
		Summary:    "Scaffold complete memory stack (skill, AGENTS.md, knowledge, Makefile)",
		PrintUsage: printBootstrapUsage,
		Run:        cmdBootstrap,
	})
	Register(&Command{
		Name:       "agents",
		Summary:    "Manage AGENTS.md, lint AAG rules, and maintain SSoT tool symlinks",
		PrintUsage: printAgentsUsage,
		Run:        cmdAgents,
	})
	Register(&Command{
		Name:       "mcp",
		Summary:    "Run as a Model Context Protocol (MCP) server over stdio",
		PrintUsage: printMCPUsage,
		Run:        cmdMCP,
	})
	Register(&Command{
		Name:       "hub",
		Summary:    "Zero-knowledge sync and vault management (push, pull, sync, serve)",
		PrintUsage: printHubUsage,
		Run:        cmdHub,
	})
}
