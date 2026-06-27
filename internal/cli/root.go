// Package cli wires the cobra command tree for the localbio binary.
package cli

import (
	"github.com/spf13/cobra"
)

// Execute builds and runs the root command.
func Execute(version string) error {
	a, err := newApp()
	if err != nil {
		return err
	}

	root := &cobra.Command{
		Use:           "localbio",
		Short:         "CLI & MCP server for local.bio",
		Long:          "localbio drives a local.bio account: login, pick a pickup point, search products, manage your basket and review your orders. It also exposes the same features as an MCP server.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			if f, _ := cmd.Flags().GetString("format"); f != "" {
				a.format = f
			}
		},
	}
	root.PersistentFlags().String("format", "text", "output format: text|json")
	root.CompletionOptions.DisableDefaultCmd = true

	root.AddCommand(
		newLoginCmd(a),
		newLogoutCmd(a),
		newInfoCmd(a),
		newStoreCmd(a),
		newOrdersCmd(a),
		newSearchCmd(a),
		newBasketCmd(a),
		newMCPCmd(a, version),
	)
	return root.Execute()
}
