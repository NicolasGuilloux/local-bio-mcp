// Command localbio is a CLI and MCP server to drive a local.bio account.
package main

import (
	"fmt"
	"os"

	"github.com/nover/local-bio-mcp/internal/cli"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := cli.Execute(version); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
