package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func init() {
	if Version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			if info.Main.Version != "" && info.Main.Version != "(devel)" {
				Version = info.Main.Version
			}
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" && Commit == "none" {
					if len(setting.Value) > 7 {
						Commit = setting.Value[:7]
					} else {
						Commit = setting.Value
					}
				}
				if setting.Key == "vcs.time" && Date == "unknown" {
					Date = setting.Value
				}
			}
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "validate":
		cmdValidate(args)
	case "search":
		cmdSearch(args)
	case "show":
		cmdShow(args)
	case "create":
		cmdCreate(args)
	case "update":
		cmdUpdate(args)
	case "relate":
		cmdRelate(args)
	case "init":
		cmdInit(args)
	case "bootstrap":
		cmdBootstrap(args)
	case "mcp":
		cmdMCP(args)
	case "version", "--version", "-v":
		fmt.Printf("okf version %s (OKF v0.2 specification)\n", Version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command '%s'\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`OKF Agent Memory CLI (v%s)

Usage:
  okf <command> [arguments] [flags]

Commands:
  validate [bundle]      Validate an OKF bundle for conformance and graph health
  search <query> [bundle] Search concepts using in-memory BM25 scoring
  show <concept-id>      Display full concept details, frontmatter, and links
  create <id> [bundle]   Create a new concept with automated bookkeeping
  update <id> [bundle]   Update an existing concept
  relate <src> <tgt>     Connect two concepts with a relative link and context
  init [path]            Initialize a new OKF v0.2 bundle (index.md, log.md)
  bootstrap [target-dir] Scaffold complete memory stack (skill, AGENTS.md, knowledge, Makefile)
  mcp [bundle]           Run as a Model Context Protocol (MCP) server over stdio
  version                Print version information
  help                   Show this help message

Flags (general):
  --json                 Emit machine-readable JSON output
  --strict               Gate connectivity warnings as errors in validate
  --drift                Check index.md listing descriptions against concepts

`, Version)
}

func defaultBundle(args []string) (string, []string) {
	var bundleDir string
	var remaining []string

	for i := range args {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") && bundleDir == "" {
			bundleDir = arg
		} else {
			remaining = append(remaining, arg)
		}
	}

	if bundleDir == "" {
		// Look for ./knowledge or default to current directory
		if info, err := os.Stat("knowledge"); err == nil && info.IsDir() {
			bundleDir = "knowledge"
		} else {
			bundleDir = "."
		}
	}

	return bundleDir, remaining
}

func cmdValidate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	strict := fs.Bool("strict", false, "Fail on broken links, orphans, and provenance gaps")
	drift := fs.Bool("drift", false, "Check for drift between index.md and concept descriptions")
	jsonOut := fs.Bool("json", false, "Output results as JSON")

	bundleDir, flagArgs := defaultBundle(args)
	_ = fs.Parse(flagArgs)

	b, err := okf.LoadBundle(bundleDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading bundle: %v\n", err)
		os.Exit(2)
	}

	res := okf.Validate(b, okf.ValidateOptions{Strict: *strict, Drift: *drift})

	if *jsonOut {
		data, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(data))
		if !res.GatePassed {
			os.Exit(1)
		}
		return
	}

	for _, w := range res.Warnings {
		fmt.Printf("warn  %s\n", w)
	}
	for _, g := range res.GateFindings {
		prefix := "warn "
		if *strict {
			prefix = "gate "
		}
		fmt.Printf("%s %s\n", prefix, g)
	}
	for _, bl := range res.BrokenLinks {
		prefix := "warn "
		if *strict {
			prefix = "gate "
		}
		fmt.Printf("%s %s: broken concept link -> %s (%s)\n", prefix, bl.SourceConcept, bl.TargetHref, bl.Reason)
	}
	for _, o := range res.Orphans {
		prefix := "warn "
		if *strict {
			prefix = "gate "
		}
		fmt.Printf("%s %s.md: orphan (no concept links in or out)\n", prefix, o)
	}
	for _, e := range res.Errors {
		fmt.Printf("error %s\n", e)
	}

	verStr := "no declared version"
	if res.DeclaredVer != "" {
		verStr = "v" + res.DeclaredVer
	}

	strictStr := ""
	if *strict {
		strictStr = " [--strict]"
	}

	fmt.Printf("\nOKF v0.2 check of \"%s\" (%s): %d concept(s), %d error(s), %d warning(s); %d broken link(s), %d orphan(s), %d stale%s. ",
		bundleDir, verStr, res.ConceptCount, len(res.Errors), len(res.Warnings)+len(res.GateFindings), len(res.BrokenLinks), len(res.Orphans), res.StaleCount, strictStr)

	if !res.IsConformant {
		fmt.Println("NOT conformant.")
		os.Exit(1)
	} else if !res.GatePassed {
		fmt.Println("Conformant, but the producer gate failed.")
		os.Exit(1)
	} else {
		fmt.Println("Conformant.")
	}
}

func cmdSearch(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: okf search <query> [bundle] [--limit N] [--json]")
		os.Exit(1)
	}

	query := args[0]
	var subArgs []string
	if len(args) > 1 {
		subArgs = args[1:]
	}

	fs := flag.NewFlagSet("search", flag.ExitOnError)
	limit := fs.Int("limit", 10, "Maximum number of search results")
	jsonOut := fs.Bool("json", false, "Output results as JSON")

	bundleDir, flagArgs := defaultBundle(subArgs)
	_ = fs.Parse(flagArgs)

	b, err := okf.LoadBundle(bundleDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading bundle: %v\n", err)
		os.Exit(2)
	}

	results := b.Search(query, *limit)

	if *jsonOut {
		data, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(data))
		return
	}

	if len(results) == 0 {
		fmt.Printf("No matching concepts found for query: '%s'\n", query)
		return
	}

	fmt.Printf("Found %d matching concept(s) in '%s':\n\n", len(results), bundleDir)
	for i, r := range results {
		fmt.Printf("%2d. [%.2f] %s (%s)\n    %s\n    Matches: %s\n\n",
			i+1, r.Score, r.ConceptID, r.Type, r.Description, strings.Join(r.MatchedOn, ", "))
	}
}

func cmdShow(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: okf show <concept-id> [bundle] [--json] [--raw]")
		os.Exit(1)
	}

	conceptID := strings.TrimSuffix(args[0], ".md")
	var subArgs []string
	if len(args) > 1 {
		subArgs = args[1:]
	}

	fs := flag.NewFlagSet("show", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Output concept as JSON")
	rawOut := fs.Bool("raw", false, "Output raw markdown file content")

	bundleDir, flagArgs := defaultBundle(subArgs)
	_ = fs.Parse(flagArgs)

	b, err := okf.LoadBundle(bundleDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading bundle: %v\n", err)
		os.Exit(2)
	}

	c, ok := b.Concepts[conceptID]
	if !ok {
		fmt.Fprintf(os.Stderr, "Concept '%s' not found in '%s'\n", conceptID, bundleDir)
		os.Exit(1)
	}

	if *rawOut {
		fmt.Print(c.RawContent)
		return
	}

	if *jsonOut {
		data, _ := json.MarshalIndent(c, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Printf("ID:          %s\n", c.ID)
	fmt.Printf("Type:        %s\n", c.Type)
	fmt.Printf("Title:       %s\n", c.Title)
	fmt.Printf("Description: %s\n", c.Description)
	if len(c.Tags) > 0 {
		fmt.Printf("Tags:        %s\n", strings.Join(c.Tags, ", "))
	}
	if c.Generated != nil {
		fmt.Printf("Generated:   %s by %s\n", c.Generated.At, c.Generated.By)
	}
	if len(b.InboundGraph[c.ID]) > 0 {
		fmt.Printf("Inbound:     %s\n", strings.Join(b.InboundGraph[c.ID], ", "))
	}
	if len(b.Graph[c.ID]) > 0 {
		fmt.Printf("Outbound:    %s\n", strings.Join(b.Graph[c.ID], ", "))
	}
	fmt.Println("\n--- Body ---")
	fmt.Println(strings.TrimSpace(c.Body))
}

func cmdCreate(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: okf create <concept-id> [bundle] --type <type> --title <title> --desc <desc>")
		os.Exit(1)
	}

	conceptID := strings.TrimSuffix(args[0], ".md")
	var subArgs []string
	if len(args) > 1 {
		subArgs = args[1:]
	}

	fs := flag.NewFlagSet("create", flag.ExitOnError)
	cType := fs.String("type", "Fact", "Concept type (required)")
	title := fs.String("title", "", "Concept title")
	desc := fs.String("desc", "", "Concept description (one sentence)")
	body := fs.String("body", "", "Concept body content")
	tagsStr := fs.String("tags", "", "Comma-separated tags")
	actor := fs.String("actor", "agent/cli", "Author actor string")
	noLog := fs.Bool("no-log", false, "Skip appending to log.md")
	noIndex := fs.Bool("no-index", false, "Skip updating parent index.md")
	jsonOut := fs.Bool("json", false, "Emit JSON result")

	bundleDir, flagArgs := defaultBundle(subArgs)
	_ = fs.Parse(flagArgs)

	if *cType == "" {
		fmt.Fprintln(os.Stderr, "Error: --type is required")
		os.Exit(1)
	}

	if *title == "" {
		*title = filepath.Base(conceptID)
	}

	var tags []string
	if *tagsStr != "" {
		for _, t := range strings.Split(*tagsStr, ",") {
			trimmed := strings.TrimSpace(t)
			if trimmed != "" {
				tags = append(tags, trimmed)
			}
		}
	}

	c := &okf.Concept{
		ID:          conceptID,
		Path:        conceptID + ".md",
		Type:        *cType,
		Title:       *title,
		Description: *desc,
		Tags:        tags,
		Body:        *body,
	}

	err := okf.SaveConcept(bundleDir, c, true, !*noLog, !*noIndex, *actor)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating concept: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		data, _ := json.Marshal(map[string]string{
			"status":     "success",
			"concept_id": conceptID,
			"path":       c.Path,
		})
		fmt.Println(string(data))
	} else {
		fmt.Printf("Created concept '%s' (%s) in '%s'\n", c.Path, c.Title, bundleDir)
	}
}

func cmdUpdate(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: okf update <concept-id> [bundle] [--title <title>] [--desc <desc>] [--body <body>]")
		os.Exit(1)
	}

	conceptID := strings.TrimSuffix(args[0], ".md")
	var subArgs []string
	if len(args) > 1 {
		subArgs = args[1:]
	}

	fs := flag.NewFlagSet("update", flag.ExitOnError)
	title := fs.String("title", "", "Updated concept title")
	desc := fs.String("desc", "", "Updated description")
	body := fs.String("body", "", "Updated body content")
	actor := fs.String("actor", "agent/cli", "Author actor string")
	noLog := fs.Bool("no-log", false, "Skip appending to log.md")
	noIndex := fs.Bool("no-index", false, "Skip updating parent index.md")
	jsonOut := fs.Bool("json", false, "Emit JSON result")

	bundleDir, flagArgs := defaultBundle(subArgs)
	_ = fs.Parse(flagArgs)

	b, err := okf.LoadBundle(bundleDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading bundle: %v\n", err)
		os.Exit(2)
	}

	c, ok := b.Concepts[conceptID]
	if !ok {
		fmt.Fprintf(os.Stderr, "Concept '%s' not found\n", conceptID)
		os.Exit(1)
	}

	if *title != "" {
		c.Title = *title
	}
	if *desc != "" {
		c.Description = *desc
	}
	if *body != "" {
		c.Body = *body
	}

	err = okf.SaveConcept(bundleDir, c, false, !*noLog, !*noIndex, *actor)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error updating concept: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		data, _ := json.Marshal(map[string]string{
			"status":     "success",
			"concept_id": conceptID,
			"path":       c.Path,
		})
		fmt.Println(string(data))
	} else {
		fmt.Printf("Updated concept '%s' in '%s'\n", c.Path, bundleDir)
	}
}

func cmdRelate(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: okf relate <source-concept-id> <target-concept-id> [bundle] [--desc <context>] [--actor <actor>] [--json]")
		os.Exit(1)
	}

	sourceID := args[0]
	targetID := args[1]
	var subArgs []string
	if len(args) > 2 {
		subArgs = args[2:]
	}

	fs := flag.NewFlagSet("relate", flag.ExitOnError)
	desc := fs.String("desc", "", "Description explaining the relationship")
	actor := fs.String("actor", "agent/cli", "Author actor string")
	jsonOut := fs.Bool("json", false, "Emit JSON result")

	bundleDir, flagArgs := defaultBundle(subArgs)
	_ = fs.Parse(flagArgs)

	err := okf.RelateConcepts(bundleDir, sourceID, targetID, *desc, *actor)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error relating concepts: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		data, _ := json.Marshal(map[string]string{
			"status": "success",
			"source": sourceID,
			"target": targetID,
		})
		fmt.Println(string(data))
	} else {
		fmt.Printf("Linked '%s' -> '%s' in '%s'\n", sourceID, targetID, bundleDir)
	}
}

func cmdInit(args []string) {
	bundleDir, _ := defaultBundle(args)
	if err := okf.InitBundle(bundleDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing bundle: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Initialized OKF v0.2 bundle in '%s'\n", bundleDir)
}

func cmdBootstrap(args []string) {
	targetDir := "."
	var subArgs []string

	for i := range args {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") && targetDir == "." {
			targetDir = arg
		} else {
			subArgs = append(subArgs, arg)
		}
	}

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

func cmdMCP(args []string) {
	bundleDir, _ := defaultBundle(args)
	if err := RunMCPServer(bundleDir); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}
