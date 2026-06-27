package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/nover/local-bio-mcp/internal/basket"
	"github.com/nover/local-bio-mcp/internal/config"
)

// registerTools wires every tool onto the server.
func registerTools(s *server.MCPServer, d *deps) {
	s.AddTool(mcp.NewTool("login",
		mcp.WithDescription("Log in to local.bio and persist the session token for subsequent calls."),
		mcp.WithString("email", mcp.Required(), mcp.Description("Account email address.")),
		mcp.WithString("password", mcp.Required(), mcp.Description("Account password.")),
	), d.handleLogin)

	s.AddTool(mcp.NewTool("logout",
		mcp.WithDescription("Log out and clear the stored session token."),
	), d.handleLogout)

	s.AddTool(mcp.NewTool("account_info",
		mcp.WithDescription("Get the currently logged-in account profile."),
	), d.handleInfo)

	s.AddTool(mcp.NewTool("store_search",
		mcp.WithDescription("Search pickup points (stores) by city or postal code."),
		mcp.WithString("query", mcp.Required(), mcp.Description("City name or postal code, e.g. 'Lyon' or '69007'.")),
	), d.handleStoreSearch)

	s.AddTool(mcp.NewTool("store_set",
		mcp.WithDescription("Select the active pickup point by its reference (the 'url'/ref from store_search)."),
		mcp.WithString("ref", mcp.Required(), mcp.Description("Store reference/slug.")),
	), d.handleStoreSet)

	s.AddTool(mcp.NewTool("product_search",
		mcp.WithDescription("List the products available for the selected store. With no query, returns the full store catalogue; with a query, filters by name/description/category."),
		mcp.WithString("query", mcp.Description("Optional product search terms. Omit to list everything available for the store.")),
		mcp.WithBoolean("include_unavailable", mcp.Description("Include inactive/out-of-stock products (default false).")),
	), d.handleProductSearch)

	s.AddTool(mcp.NewTool("basket_get",
		mcp.WithDescription("Show the current basket contents (local, per selected store)."),
	), d.handleBasketGet)

	s.AddTool(mcp.NewTool("basket_add",
		mcp.WithDescription("Add a product to the basket by its product id (the id from product_search)."),
		mcp.WithString("ean", mcp.Required(), mcp.Description("Product id.")),
		mcp.WithNumber("quantity", mcp.Description("Quantity to add (default 1).")),
	), d.handleBasketAdd)

	s.AddTool(mcp.NewTool("basket_remove",
		mcp.WithDescription("Remove a product from the basket by its product id. Omit quantity to remove all units."),
		mcp.WithString("ean", mcp.Required(), mcp.Description("Product id.")),
		mcp.WithNumber("quantity", mcp.Description("Units to remove (default: remove all).")),
	), d.handleBasketRemove)

	s.AddTool(mcp.NewTool("orders_list",
		mcp.WithDescription("List the customer's previous orders."),
	), d.handleOrdersList)

	s.AddTool(mcp.NewTool("order_detail",
		mcp.WithDescription("Show a single order with its articles. Reference it by 1-based index from orders_list (1 = most recent) or by order id."),
		mcp.WithString("ref", mcp.Required(), mcp.Description("Order index (1..N) or order id.")),
	), d.handleOrderDetail)
}

// --- helpers ---------------------------------------------------------------

func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

func errResult(err error) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError(err.Error()), nil
}

func argString(req mcp.CallToolRequest, key string) string {
	if v, ok := req.GetArguments()[key].(string); ok {
		return v
	}
	return ""
}

func argBool(req mcp.CallToolRequest, key string) bool {
	b, _ := req.GetArguments()[key].(bool)
	return b
}

func argInt(req mcp.CallToolRequest, key string, def int) int {
	switch v := req.GetArguments()[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return def
}

// --- handlers --------------------------------------------------------------

func (d *deps) handleLogin(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cfg, err := d.load()
	if err != nil {
		return errResult(err)
	}
	email := argString(req, "email")
	password := argString(req, "password")
	if email == "" || password == "" {
		return errResult(fmt.Errorf("email and password are required"))
	}
	lr, err := d.client(cfg).Login(ctx, email, password)
	if err != nil {
		return errResult(err)
	}
	token := lr.TokenValue()
	if token == "" {
		return errResult(fmt.Errorf("login succeeded but no token returned"))
	}
	cfg.Token = token
	cfg.Email = email
	if err := cfg.Save(); err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]any{"loggedIn": true, "email": email})
}

func (d *deps) handleLogout(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cfg, err := d.load()
	if err != nil {
		return errResult(err)
	}
	if cfg.Token != "" {
		_ = d.client(cfg).Logout(ctx)
	}
	cfg.Clear()
	if err := cfg.Save(); err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]any{"loggedOut": true})
}

func (d *deps) handleInfo(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cfg, err := d.load()
	if err != nil {
		return errResult(err)
	}
	if cfg.Token == "" {
		return errResult(fmt.Errorf("not logged in; call login first"))
	}
	acc, err := d.client(cfg).Me(ctx)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(acc.Raw)
}

func (d *deps) handleStoreSearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cfg, err := d.load()
	if err != nil {
		return errResult(err)
	}
	query := argString(req, "query")
	if query == "" {
		return errResult(fmt.Errorf("query is required"))
	}
	place, err := d.geocoder.Lookup(ctx, query)
	if err != nil {
		return errResult(err)
	}
	res, err := d.client(cfg).SearchStores(ctx, place.Lat, place.Lng)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]any{"place": place, "stores": res.Stores})
}

func (d *deps) handleStoreSet(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cfg, err := d.load()
	if err != nil {
		return errResult(err)
	}
	ref := argString(req, "ref")
	if ref == "" {
		return errResult(fmt.Errorf("ref is required"))
	}
	cfg.StoreID = ref
	if err := cfg.Save(); err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]any{"storeId": ref})
}

func (d *deps) handleProductSearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cfg, err := d.load()
	if err != nil {
		return errResult(err)
	}
	if cfg.StoreID == "" {
		return errResult(fmt.Errorf("no store selected; call store_set first"))
	}
	query := argString(req, "query")
	availableOnly := !argBool(req, "include_unavailable")
	products, err := d.client(cfg).SearchProducts(ctx, query, cfg.StoreID, availableOnly)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]any{"query": query, "storeId": cfg.StoreID, "count": len(products), "products": products})
}

func (d *deps) handleBasketGet(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cfg, err := d.requireAuth()
	if err != nil {
		return errResult(err)
	}
	cart, err := basket.New(d.client(cfg), cfg).Get(ctx)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(cart)
}

func (d *deps) handleBasketAdd(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cfg, err := d.requireAuth()
	if err != nil {
		return errResult(err)
	}
	ean := argString(req, "ean")
	if ean == "" {
		return errResult(fmt.Errorf("ean (product id) is required"))
	}
	cart, err := basket.New(d.client(cfg), cfg).Add(ctx, ean, argInt(req, "quantity", 1))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(cart)
}

func (d *deps) handleBasketRemove(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cfg, err := d.requireAuth()
	if err != nil {
		return errResult(err)
	}
	ean := argString(req, "ean")
	if ean == "" {
		return errResult(fmt.Errorf("ean (product id) is required"))
	}
	cart, err := basket.New(d.client(cfg), cfg).Remove(ctx, ean, argInt(req, "quantity", 0))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(cart)
}

func (d *deps) handleOrdersList(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cfg, err := d.requireAuth()
	if err != nil {
		return errResult(err)
	}
	orders, err := d.client(cfg).Orders(ctx)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(orders)
}

func (d *deps) handleOrderDetail(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cfg, err := d.requireAuth()
	if err != nil {
		return errResult(err)
	}
	ref := argString(req, "ref")
	if ref == "" {
		return errResult(fmt.Errorf("ref is required"))
	}
	o, err := d.client(cfg).FindOrder(ctx, ref)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(o.Raw)
}

func (d *deps) requireAuth() (*config.Config, error) {
	cfg, err := d.load()
	if err != nil {
		return nil, err
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("not logged in; call login first")
	}
	return cfg, nil
}
