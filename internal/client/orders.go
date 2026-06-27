package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// Orders lists the authenticated customer's orders, most recent first.
func (c *Client) Orders(ctx context.Context) ([]Order, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, "/orders/byCustomer", nil, nil, &raw); err != nil {
		return nil, err
	}
	orders := decodeOrders(raw)
	sort.SliceStable(orders, func(i, j int) bool { return orders[i].Date > orders[j].Date })
	return orders, nil
}

// Order returns the detail (with articles) of a single order by its Mongo id.
// This endpoint lives on the legacy LoopBack API.
func (c *Client) Order(ctx context.Context, id string) (*Order, error) {
	var raw json.RawMessage
	if err := c.doLegacy(ctx, http.MethodGet, "/orders/"+id+"/getOrderWithPaymentIntent", nil, nil, &raw); err != nil {
		return nil, err
	}
	var o Order
	_ = json.Unmarshal(raw, &o)
	_ = json.Unmarshal(raw, &o.Raw)
	return &o, nil
}

// FindOrder resolves a user-supplied reference to an order and loads its detail.
// The reference may be: a full Mongo id, a unique id prefix, or a 1-based index
// into the (newest-first) order list.
func (c *Client) FindOrder(ctx context.Context, ref string) (*Order, error) {
	ref = strings.TrimSpace(ref)
	if isObjectID(ref) {
		return c.Order(ctx, ref)
	}
	orders, err := c.Orders(ctx)
	if err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return nil, fmt.Errorf("no orders to show")
	}
	// 1-based index (e.g. `orders 1` = most recent).
	if n, err := strconv.Atoi(ref); err == nil {
		if n < 1 || n > len(orders) {
			return nil, fmt.Errorf("order index %d out of range (1..%d)", n, len(orders))
		}
		return c.Order(ctx, orders[n-1].ID)
	}
	// id / id prefix match.
	var matches []string
	for _, o := range orders {
		if o.ID == ref {
			return c.Order(ctx, o.ID)
		}
		if strings.HasPrefix(o.ID, ref) {
			matches = append(matches, o.ID)
		}
	}
	switch len(matches) {
	case 1:
		return c.Order(ctx, matches[0])
	case 0:
		return nil, fmt.Errorf("no order matching %q (use an index 1..%d or an order id)", ref, len(orders))
	default:
		return nil, fmt.Errorf("ambiguous reference %q matches %d orders", ref, len(matches))
	}
}

// decodeOrders accepts a bare array or `{ "orders": [...] }`.
func decodeOrders(raw json.RawMessage) []Order {
	var arr []Order
	if json.Unmarshal(raw, &arr) == nil && arr != nil {
		return arr
	}
	var wrapped struct {
		Orders []Order `json:"orders"`
	}
	_ = json.Unmarshal(raw, &wrapped)
	return wrapped.Orders
}

// isObjectID reports whether s is a 24-char hex Mongo ObjectId.
func isObjectID(s string) bool {
	if len(s) != 24 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}
