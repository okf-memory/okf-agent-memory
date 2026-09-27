package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

// ============================================================================
// SEARCH & SHOW
// ============================================================================

func printSearchUsage() {
	fmt.Printf(`Search concepts using in-memory BM25 scoring, frontmatter filters, or code path.

Usage:
  okf search [query] [bundle] [flags]
  okf search --for-path <path> [bundle] [flags]
  okf search --filter <expr> [bundle] [flags]
  okf search --stale-within <duration> [bundle] [flags]

Arguments:
  [query]                Search terms to match against concept titles, descriptions, and bodies
  [bundle]               Path to OKF bundle directory (default: 'knowledge' or '.')

Flags:
  --for-path <path>      Discover concepts governing a specific file path via code_refs
  --filter <expr>        Filter by frontmatter key-value predicates (e.g. 'type=Decision', 'verified.by=human')
  --stale-within <dur>   Filter concepts becoming stale within relative duration (e.g. '14d', '2w', '3m')
  --limit <N>            Maximum number of search results to return (default: 10)
  --json                 Output results as machine-readable JSON array with BM25 scores

Examples:
  okf search "authentication jwt" knowledge --limit 3
  okf search --filter "verified.by=human" knowledge
  okf search --stale-within 14d knowledge
  okf search --for-path pkg/auth/service.go knowledge --json
  okf search "database connection" --filter "status=stable" --limit 5
`)
}

func printShowUsage() {
	fmt.Printf(`Display concept details, metadata, relationships, and body content.

Usage:
  okf show <concept-id> [bundle] [flags]

Arguments:
  <concept-id>           Unique concept identifier (e.g. 'architecture/database' or 'decisions/adr-001')
  [bundle]               Path to OKF bundle directory (default: 'knowledge' or '.')

Flags:
  --raw                  Output verbatim raw markdown file content
  --json                 Output parsed concept metadata and body as JSON

Examples:
  okf show architecture/database knowledge
  okf show decisions/adr-001 --raw
  okf show architecture/auth --json
`)
}

func cmdSearch(args []string) {
	if hasHelpFlag(args) {
		printSearchUsage()
		return
	}
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	limit := fs.Int("limit", 10, "Maximum number of search results")
	forPath := fs.String("for-path", "", "Filter concepts governing a specific file path via code_refs")
	filter := fs.String("filter", "", "Filter concepts by frontmatter key-value predicates")
	staleWithinStr := fs.String("stale-within", "", "Filter concepts becoming stale within relative duration")
	jsonOut := fs.Bool("json", false, "Output results as JSON")

	var flagArgs []string
	var positional []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			name := strings.TrimLeft(arg, "-")
			if (name == "limit" || name == "for-path" || name == "filter" || name == "stale-within") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		} else {
			positional = append(positional, arg)
		}
	}

	_ = fs.Parse(flagArgs)

	fallback := "."
	if info, err := os.Stat("knowledge"); err == nil && info.IsDir() {
		fallback = "knowledge"
	}
	bundleDir := fallback

	var query string
	if *forPath != "" || *filter != "" || *staleWithinStr != "" {
		if len(positional) == 1 {
			cleanCandidate := filepath.Clean(positional[0])
			// #nosec G703 -- CLI argument used for bundle directory existence check
			if info, err := os.Stat(cleanCandidate); err == nil && info.IsDir() {
				bundleDir = cleanCandidate
			} else {
				query = positional[0]
			}
		} else if len(positional) >= 2 {
			query = positional[0]
			bundleDir = positional[1]
		}
	} else {
		if len(positional) >= 1 {
			query = positional[0]
		}
		if len(positional) >= 2 {
			bundleDir = positional[1]
		}
	}

	if query == "" && *forPath == "" && *filter == "" && *staleWithinStr == "" {
		fmt.Fprintln(os.Stderr, "Usage: okf search [query] [bundle] [--for-path <path>] [--filter <expr>] [--stale-within <duration>] [--limit N] [--json]")
		os.Exit(1)
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

	b, err := okf.LoadBundle(bundleDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading bundle: %v\n", err)
		os.Exit(2)
	}

	results, err := b.SearchAdvanced(okf.SearchOptions{
		Query:       query,
		TargetPath:  *forPath,
		Limit:       *limit,
		Filter:      *filter,
		StaleWithin: staleWithin,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Search error: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		data, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(data))
		return
	}

	if len(results) == 0 {
		var criteria []string
		if query != "" {
			criteria = append(criteria, fmt.Sprintf("query '%s'", query))
		}
		if *forPath != "" {
			criteria = append(criteria, fmt.Sprintf("path '%s'", *forPath))
		}
		if *filter != "" {
			criteria = append(criteria, fmt.Sprintf("filter '%s'", *filter))
		}
		if *staleWithinStr != "" {
			criteria = append(criteria, fmt.Sprintf("stale within %s", *staleWithinStr))
		}
		fmt.Printf("No matching concepts found for %s in '%s'\n", strings.Join(criteria, ", "), bundleDir)
		return
	}

	if *forPath != "" {
		fmt.Printf("Found %d matching concept(s) governing '%s' in '%s':\n\n", len(results), *forPath, bundleDir)
	} else {
		fmt.Printf("Found %d matching concept(s) in '%s':\n\n", len(results), bundleDir)
	}

	for i, r := range results {
		govBadge := fmt.Sprintf("[%s]", r.Governance)
		fmt.Printf("%2d. %-12s [%.2f] %s (%s)\n    %s\n    Matches: %s\n\n",
			i+1, govBadge, r.Score, r.ConceptID, r.Type, r.Description, strings.Join(r.MatchedOn, ", "))
	}
}

func cmdShow(args []string) {
	if len(args) == 0 || hasHelpFlag(args) {
		printShowUsage()
		if len(args) == 0 {
			os.Exit(1)
		}
		return
	}

	rawID := strings.TrimSpace(args[0])
	if err := okf.ValidateConceptID(rawID); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	conceptID := strings.TrimSuffix(rawID, ".md")
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
