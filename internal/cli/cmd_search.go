package cli

import (
	"fmt"

	"github.com/nover/local-bio-mcp/internal/client"
	"github.com/spf13/cobra"
)

func newSearchCmd(a *app) *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search products, or list all products available for the selected store",
		Long: "Search products in the selected store's catalogue. With no query, " +
			"lists every product available for the store. Use --all to include " +
			"products that are currently inactive/unavailable.",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if a.cfg.StoreID == "" {
				return fmt.Errorf("no store selected (run `localbio store set <ref>`)")
			}
			query := joinArgs(args)
			products, err := a.client().SearchProducts(cmd.Context(), query, a.cfg.StoreID, !all)
			if err != nil {
				return err
			}
			if a.jsonOutput() {
				return emitJSON(cmd.OutOrStdout(), products)
			}
			out := cmd.OutOrStdout()
			if len(products) == 0 {
				if query == "" {
					fmt.Fprintln(out, "No products available for this store.")
				} else {
					fmt.Fprintf(out, "No product matching %q.\n", query)
				}
				return nil
			}
			rows := make([][]string, 0, len(products))
			for _, p := range products {
				rows = append(rows, []string{
					p.ID,
					p.Name,
					categoryLabel(p.CategoryID),
					money(p.Price()),
					p.Unit(),
					availability(p),
				})
			}
			table(out, []string{"ID", "NAME", "CATEGORY", "PRICE", "UNIT", "AVAILABILITY"}, rows)
			fmt.Fprintf(out, "\n%d product(s).\n", len(products))
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "include inactive/unavailable products")
	return cmd
}

func availability(p client.Product) string {
	switch {
	case !p.Active:
		return "inactive"
	case !p.InStock():
		return "out of stock"
	default:
		return "available"
	}
}
