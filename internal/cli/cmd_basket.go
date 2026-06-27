package cli

import (
	"fmt"
	"strconv"

	"github.com/nover/local-bio-mcp/internal/basket"
	"github.com/nover/local-bio-mcp/internal/client"
	"github.com/spf13/cobra"
)

func newBasketCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "basket",
		Short: "Inspect and manage your basket",
		Long: "Manage your server-side basket (shared with the website and mobile app). " +
			"Products are referenced by their product id (the ID column of `search`).",
	}
	cmd.AddCommand(newBasketGetCmd(a), newBasketAddCmd(a), newBasketRemoveCmd(a))
	return cmd
}

func (a *app) basketService() *basket.Service {
	return basket.New(a.client(), a.cfg)
}

func newBasketGetCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Show current basket contents",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireAuth(a); err != nil {
				return err
			}
			cart, err := a.basketService().Get(cmd.Context())
			if err != nil {
				return err
			}
			return printBasket(cmd, a, cart)
		},
	}
}

func newBasketAddCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "add <ean> [qty]",
		Short: "Add a product to your basket (default qty: 1)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(a); err != nil {
				return err
			}
			qty, err := parseQty(args, 1)
			if err != nil {
				return err
			}
			cart, err := a.basketService().Add(cmd.Context(), args[0], qty)
			if err != nil {
				return err
			}
			return printBasket(cmd, a, cart)
		},
	}
}

func newBasketRemoveCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <ean> [qty]",
		Short: "Remove a product (default: remove all)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(a); err != nil {
				return err
			}
			qty, err := parseQty(args, 0) // 0 => remove all
			if err != nil {
				return err
			}
			cart, err := a.basketService().Remove(cmd.Context(), args[0], qty)
			if err != nil {
				return err
			}
			return printBasket(cmd, a, cart)
		},
	}
}

func requireAuth(a *app) error {
	if a.cfg.Token == "" {
		return fmt.Errorf("not logged in (run `localbio login`)")
	}
	return nil
}

func parseQty(args []string, def int) (int, error) {
	if len(args) < 2 {
		return def, nil
	}
	q, err := strconv.Atoi(args[1])
	if err != nil {
		return 0, fmt.Errorf("invalid quantity %q", args[1])
	}
	return q, nil
}

func printBasket(cmd *cobra.Command, a *app, cart *client.Cart) error {
	if a.jsonOutput() {
		if cart.Raw != nil {
			return emitJSON(cmd.OutOrStdout(), cart.Raw)
		}
		return emitJSON(cmd.OutOrStdout(), cart)
	}
	out := cmd.OutOrStdout()
	if len(cart.Products) == 0 {
		fmt.Fprintln(out, "Basket is empty.")
		return nil
	}
	fmt.Fprintf(out, "Basket for store %s:\n\n", cart.StoreID)
	rows := make([][]string, 0, len(cart.Products))
	for _, l := range cart.Products {
		rows = append(rows, []string{
			l.ProductID,
			l.Name,
			qty(l.Quantity),
			money(l.Price),
			money(l.LineTotal()),
		})
	}
	table(out, []string{"ID", "NAME", "QTY", "PRICE", "LINE"}, rows)
	fmt.Fprintf(out, "\nTotal: %s\n", money(cart.Total()))
	return nil
}
