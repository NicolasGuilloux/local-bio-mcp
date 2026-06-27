package cli

import (
	"fmt"
	"strings"

	"github.com/nover/local-bio-mcp/internal/client"
	"github.com/spf13/cobra"
)

func newOrdersCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "orders [ref]",
		Short: "List previous orders, or show one (by index 1..N or order id)",
		Long: "With no argument, lists your previous orders (most recent first).\n" +
			"With a reference, shows that order and its articles. The reference is\n" +
			"either a 1-based index from the list (e.g. `orders 1` = most recent) or\n" +
			"an order id (full or unique prefix).",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if a.cfg.Token == "" {
				return fmt.Errorf("not logged in (run `localbio login`)")
			}
			c := a.client()
			if len(args) == 1 {
				return showOrder(cmd, a, c, args[0])
			}
			return listOrders(cmd, a, c)
		},
	}
}

func listOrders(cmd *cobra.Command, a *app, c *client.Client) error {
	orders, err := c.Orders(cmd.Context())
	if err != nil {
		return err
	}
	if a.jsonOutput() {
		return emitJSON(cmd.OutOrStdout(), orders)
	}
	out := cmd.OutOrStdout()
	if len(orders) == 0 {
		fmt.Fprintln(out, "No orders yet.")
		return nil
	}
	rows := make([][]string, 0, len(orders))
	for i, o := range orders {
		rows = append(rows, []string{
			fmt.Sprintf("%d", i+1),
			dateShort(o.Date),
			o.StoreID,
			qty(itemCount(o)),
			money(o.Total()),
			o.ID,
		})
	}
	table(out, []string{"#", "DATE", "STORE", "ITEMS", "TOTAL", "ID"}, rows)
	fmt.Fprintf(out, "\n%d order(s). Use `localbio orders <#|id>` for details.\n", len(orders))
	return nil
}

func showOrder(cmd *cobra.Command, a *app, c *client.Client, ref string) error {
	o, err := c.FindOrder(cmd.Context(), ref)
	if err != nil {
		return err
	}
	if a.jsonOutput() {
		return emitJSON(cmd.OutOrStdout(), o.Raw)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Order %s\n", o.ID)
	if o.Date != "" {
		fmt.Fprintf(out, "Date:  %s\n", dateShort(o.Date))
	}
	if o.StoreID != "" {
		fmt.Fprintf(out, "Store: %s\n", o.StoreID)
	}
	fmt.Fprintln(out)

	rows := make([][]string, 0, len(o.Products))
	for _, p := range o.Products {
		rows = append(rows, []string{
			p.Name,
			p.Packaging.Name,
			qty(p.Quantity),
			money(p.Packaging.Price),
			money(p.LineTotal()),
		})
	}
	if len(rows) > 0 {
		table(out, []string{"ARTICLE", "UNIT", "QTY", "PRICE", "LINE"}, rows)
	}

	fmt.Fprintf(out, "\nItems: %s\n", money(o.ItemsTotal()))
	if o.Payment.AppTip > 0 {
		fmt.Fprintf(out, "Tip (app):   %s\n", money(o.Payment.AppTip))
	}
	if o.Payment.StoreTip > 0 {
		fmt.Fprintf(out, "Tip (store): %s\n", money(o.Payment.StoreTip))
	}
	fmt.Fprintf(out, "Total: %s\n", money(o.Total()))
	return nil
}

func itemCount(o client.Order) float64 {
	n := 0.0
	for _, p := range o.Products {
		n += p.Quantity
	}
	return n
}

// dateShort turns an ISO timestamp into YYYY-MM-DD.
func dateShort(s string) string {
	if i := strings.IndexByte(s, 'T'); i > 0 {
		return s[:i]
	}
	return s
}
