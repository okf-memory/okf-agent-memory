package cli

import (
	"fmt"
	"os"
	"strings"
)

// BuildInfo holds metadata about the binary build.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Execute executes the CLI with the provided arguments and build metadata.
func Execute(args []string, info BuildInfo) int {
	if info.Version != "" {
		Version = info.Version
	}
	if info.Commit != "" {
		Commit = info.Commit
	}
	if info.Date != "" {
		Date = info.Date
	}

	if len(args) < 1 {
		printUsage()
		return 1
	}

	cmdName := args[0]
	cmdArgs := args[1:]

	switch cmdName {
	case "version", "--version", "-v":
		fmt.Printf("okf version %s (OKF v0.2 specification)\n", Version)
		return 0
	case "help", "--help", "-h":
		if len(cmdArgs) > 0 {
			subName := cmdArgs[0]
			if c, found := FindCommand(subName); found && c.PrintUsage != nil {
				c.PrintUsage()
				return 0
			}
			printUsage()
			return 0
		}
		printUsage()
		return 0
	}

	c, found := FindCommand(cmdName)
	if !found {
		fmt.Fprintf(os.Stderr, "Unknown command '%s'\n\n", cmdName)
		printUsage()
		return 1
	}

	c.Run(cmdArgs)
	return 0
}

func isHelpArg(arg string) bool {
	return arg == "--help" || arg == "-h" || arg == "help"
}

func hasHelpFlag(args []string) bool {
	for _, a := range args {
		if isHelpArg(a) {
			return true
		}
	}
	return false
}

func defaultBundle(args []string) (string, []string) {
	// Look for ./knowledge or default to current directory.
	fallback := "."
	if info, err := os.Stat("knowledge"); err == nil && info.IsDir() {
		fallback = "knowledge"
	}
	return splitOptionalPath(args, fallback)
}

// splitOptionalPath consumes an optional positional path only when it is the
// first argument. Everything else belongs to the command's FlagSet, including
// values for flags such as "--limit 3" and "--type Fact".
func splitOptionalPath(args []string, fallback string) (string, []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0], args[1:]
	}
	return fallback, args
}
