package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

func printChangesUsage() {
	fmt.Printf(`Query concepts created, updated, related, deprecated, or deleted within a time range.

Usage:
  okf changes [bundle] --since <date|duration> [--until <date|duration>] [--limit <N>] [--json]

Arguments:
  [bundle]               Path to OKF bundle directory (default: 'knowledge' or '.')

Flags:
  --since <value>        Start of time range (required): YYYY-MM-DD, ISO 8601 timestamp, or relative duration (e.g. '1d', '2w', '3m')
  --until <value>        End of time range (optional, defaults to now): YYYY-MM-DD, ISO 8601 timestamp, or relative duration
  --limit <N>            Maximum number of concepts to return [default: 50]
  --json                 Emit machine-readable JSON output matching MCP okf_changes

Examples:
  okf changes --since 7d
  okf changes knowledge --since 2026-11-01 --until 2026-11-07 --json
  okf changes --since 24h --limit 20
`)
}

func cmdChanges(args []string) {
	if hasHelpFlag(args) {
		printChangesUsage()
		return
	}

	fs := flag.NewFlagSet("changes", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	since := fs.String("since", "", "Start of time range (YYYY-MM-DD, ISO timestamp, or duration like 1d, 2w)")
	until := fs.String("until", "", "End of time range (YYYY-MM-DD, ISO timestamp, or duration; defaults to now)")
	limit := fs.Int("limit", 50, "Maximum number of concepts to return")
	jsonOut := fs.Bool("json", false, "Emit machine-readable JSON output")

	bundleDir, flagArgs := defaultBundle(args)
	if err := fs.Parse(flagArgs); err != nil {
		os.Exit(1)
	}

	if strings.TrimSpace(*since) == "" {
		fmt.Fprintln(os.Stderr, "Error: --since is required (e.g. --since 7d or --since 2026-11-01)")
		os.Exit(1)
	}
	if *limit <= 0 {
		fmt.Fprintln(os.Stderr, "Error: --limit must be a positive integer")
		os.Exit(1)
	}

	b, err := okf.LoadBundle(bundleDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading bundle: %v\n", err)
		os.Exit(2)
	}

	res, err := okf.QueryChanges(b, okf.ChangesOptions{
		Since: *since,
		Until: *until,
		Limit: *limit,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		data, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(data))
		return
	}

	if len(res.Changes) == 0 {
		fmt.Printf("No concept changes found in '%s' between %s and %s.\n", bundleDir, res.Since, res.Until)
		return
	}

	fmt.Printf("Found %d changed concept(s) in '%s' (%s .. %s):\n\n", res.Total, bundleDir, res.Since, res.Until)
	for i, ch := range res.Changes {
		badge := ""
		if ch.Deleted {
			badge = " [deleted]"
		} else if ch.Status == "deprecated" {
			badge = " [deprecated]"
		}
		fmt.Printf("%2d. %s — %s%s\n", i+1, ch.ConceptID, ch.Title, badge)
		if ch.Description != "" {
			fmt.Printf("    %s\n", ch.Description)
		}
		for _, op := range ch.Operations {
			line := fmt.Sprintf("    - [%s] %s", op.At, op.Op)
			if op.Target != "" {
				line += fmt.Sprintf(" -> %s", op.Target)
			}
			if op.Note != "" {
				line += fmt.Sprintf(" — %s", op.Note)
			}
			fmt.Println(line)
		}
		fmt.Println()
	}
}
