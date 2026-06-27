package cli

import (
	"fmt"

	"github.com/nover/local-bio-mcp/internal/mcp"
	"github.com/spf13/cobra"
)

func newMCPCmd(_ *app, version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP server (stdio transport)",
		Long:  "Start the local.bio MCP server. With no subcommand it uses the stdio transport; `mcp http [addr]` serves Streamable HTTP.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return mcp.ServeStdio(version)
		},
	}

	httpCmd := &cobra.Command{
		Use:   "http [addr]",
		Short: "Start MCP server (Streamable HTTP, default :8080)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			addr := ":8080"
			if len(args) == 1 {
				addr = args[0]
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "local.bio MCP server (Streamable HTTP) listening on %s\n", addr)
			return mcp.ServeHTTP(version, addr)
		},
	}
	cmd.AddCommand(httpCmd)
	return cmd
}
