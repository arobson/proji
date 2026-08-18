// Command proji is a git simplification layer for beginners working on
// instructor-provided assignments.
package main

import (
	"os"

	"github.com/arobson/proji/internal/cli"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	root := cli.NewRootCmd(cli.DefaultDeps())
	root.Version = version
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
