package cli

import (
	"fmt"

	"github.com/nover/local-bio-mcp/internal/client"
	"github.com/spf13/cobra"
)

func newStoreCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "store",
		Short: "Select or search pickup points (points de retrait)",
	}
	cmd.AddCommand(newStoreSetCmd(a), newStoreSearchCmd(a), newStoreShowCmd(a))
	return cmd
}

func newStoreSetCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "set <ref>",
		Short: "Select your store",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			// Validate the reference exists.
			detail, err := a.client().LoadStore(cmd.Context(), ref)
			if err != nil {
				return err
			}
			a.cfg.StoreID = ref
			if err := a.cfg.Save(); err != nil {
				return err
			}
			if a.jsonOutput() {
				return emitJSON(cmd.OutOrStdout(), map[string]any{"storeId": ref, "store": detail})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Store set to %s\n", ref)
			return nil
		},
	}
}

func newStoreSearchCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search stores by city or postal code",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := joinArgs(args)
			place, err := a.geocoder.Lookup(cmd.Context(), query)
			if err != nil {
				return err
			}
			res, err := a.client().SearchStores(cmd.Context(), place.Lat, place.Lng)
			if err != nil {
				return err
			}
			if a.jsonOutput() {
				return emitJSON(cmd.OutOrStdout(), map[string]any{
					"query":  query,
					"place":  place,
					"stores": res.Stores,
				})
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Stores near %s:\n\n", place.Label)
			rows := make([][]string, 0, len(res.Stores))
			for _, s := range res.Stores {
				rows = append(rows, []string{s.URL, s.Name, storeCity(s), s.Type})
			}
			if len(rows) == 0 {
				fmt.Fprintln(out, "No store found.")
				return nil
			}
			table(out, []string{"REF", "NAME", "CITY", "TYPE"}, rows)
			return nil
		},
	}
}

func newStoreShowCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "show [ref]",
		Short: "Show details of a store (defaults to the selected one)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := a.cfg.StoreID
			if len(args) == 1 {
				ref = args[0]
			}
			if ref == "" {
				return fmt.Errorf("no store selected (run `localbio store set <ref>`)")
			}
			detail, err := a.client().LoadStore(cmd.Context(), ref)
			if err != nil {
				return err
			}
			if a.jsonOutput() {
				return emitJSON(cmd.OutOrStdout(), detail)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Store %s\n", ref)
			return emitJSON(cmd.OutOrStdout(), detail)
		},
	}
}

func storeCity(s client.Store) string {
	if s.Address.City != "" {
		return s.Address.City
	}
	return s.Address.Postcode
}
