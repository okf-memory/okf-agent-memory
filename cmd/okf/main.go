package main

import (
	"os"
	"runtime/debug"

	"github.com/okf-memory/okf-agent-memory/internal/cli"
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
	code := cli.Execute(os.Args[1:], cli.BuildInfo{
		Version: Version,
		Commit:  Commit,
		Date:    Date,
	})
	if code != 0 {
		os.Exit(code)
	}
}
