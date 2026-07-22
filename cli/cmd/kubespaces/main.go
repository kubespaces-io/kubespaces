// Command kubespaces is the KubeSpaces CLI.
package main

import (
	"fmt"
	"os"

	"github.com/kubespaces-io/kubespaces/cli/internal/cli"
)

// Set via -ldflags "-X main.version=... -X main.commit=... -X main.date=...".
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	root := cli.NewRootCmd(cli.BuildInfo{Version: version, Commit: commit, Date: date})
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		if cli.IsUsageError(err) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
