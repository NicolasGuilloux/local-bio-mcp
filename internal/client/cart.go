package client

import (
	"context"
	"encoding/json"
	"net/http"
)

// GetCart returns the customer's current server-side basket. The cart is
// embedded in the customer profile (`GET /customers/me` → `cart`); there is no
// dedicated GET cart endpoint.
func (c *Client) GetCart(ctx context.Context) (*Cart, error) {
	var me map[string]json.RawMessage
	if err := c.do(ctx, http.MethodGet, "/customers/me", nil, nil, &me); err != nil {
		return nil, err
	}
	cart := &Cart{Products: []CartProduct{}, RawProducts: []map[string]any{}}
	raw, ok := me["cart"]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return cart, nil
	}
	_ = json.Unmarshal(raw, cart)
	_ = json.Unmarshal(raw, &cart.Raw)
	if cart.Products == nil {
		cart.Products = []CartProduct{}
	}
	if rp, ok := cart.Raw["products"].([]any); ok {
		for _, p := range rp {
			if m, ok := p.(map[string]any); ok {
				cart.RawProducts = append(cart.RawProducts, m)
			}
		}
	}
	return cart, nil
}

// SaveCart persists the cart server-side (`POST /customers/cart`) using its
// payment object and exact product objects. setupPayment is always false (true
// is reserved for the checkout/payment flow, which this CLI does not perform).
func (c *Client) SaveCart(ctx context.Context, cart *Cart) (*Cart, error) {
	return c.SaveCartProducts(ctx, cart.StoreID, cart.Payment, cart.RawProducts)
}

// SaveCartProducts posts an explicit product set (used by the basket's
// diff-correction loop).
func (c *Client) SaveCartProducts(ctx context.Context, storeID string, payment map[string]any, products []map[string]any) (*Cart, error) {
	body := map[string]any{
		"storeId":      storeID,
		"products":     products,
		"setupPayment": false,
	}
	if payment != nil {
		body["payment"] = payment
	}
	var resp json.RawMessage
	if err := c.do(ctx, http.MethodPost, "/customers/cart", nil, body, &resp); err != nil {
		return nil, err
	}
	return decodeCartResponse(resp), nil
}

// CartLineDiff is the server's per-line validation feedback from a 422: for each
// field, Fields[name] = [valueYouSent, expectedValue].
type CartLineDiff struct {
	ProductID string
	Name      string
	Fields    map[string][]any
}

// ParseCartValidation extracts the per-line validation diffs from a 422 error
// body of the form `{"message":[[line, {field:[sent,expected]}], …]}`.
func ParseCartValidation(err error) ([]CartLineDiff, bool) {
	ae, ok := err.(*APIError)
	if !ok || ae.StatusCode != http.StatusUnprocessableEntity {
		return nil, false
	}
	var body struct {
		Message []json.RawMessage `json:"message"`
	}
	if json.Unmarshal([]byte(ae.Body), &body) != nil || len(body.Message) == 0 {
		return nil, false
	}
	var out []CartLineDiff
	for _, item := range body.Message {
		var pair []json.RawMessage
		if json.Unmarshal(item, &pair) != nil || len(pair) < 2 {
			return nil, false
		}
		var line map[string]any
		_ = json.Unmarshal(pair[0], &line)
		var diff map[string][]any
		if json.Unmarshal(pair[1], &diff) != nil {
			continue
		}
		out = append(out, CartLineDiff{
			ProductID: asStr(line["productId"]),
			Name:      asStr(line["name"]),
			Fields:    diff,
		})
	}
	return out, len(out) > 0
}

func asStr(v any) string { s, _ := v.(string); return s }

// decodeCartResponse extracts the cart from a POST /customers/cart response,
// which may be `{cart}`, `{customer:{cart}}` or the cart itself.
func decodeCartResponse(raw json.RawMessage) *Cart {
	var wrapped struct {
		Cart     json.RawMessage `json:"cart"`
		Customer struct {
			Cart json.RawMessage `json:"cart"`
		} `json:"customer"`
	}
	_ = json.Unmarshal(raw, &wrapped)
	for _, candidate := range []json.RawMessage{wrapped.Cart, wrapped.Customer.Cart, raw} {
		if len(candidate) == 0 || string(candidate) == "null" {
			continue
		}
		var cart Cart
		if json.Unmarshal(candidate, &cart) == nil && (cart.Products != nil || cart.StoreID != "") {
			_ = json.Unmarshal(candidate, &cart.Raw)
			if cart.Products == nil {
				cart.Products = []CartProduct{}
			}
			if rp, ok := cart.Raw["products"].([]any); ok {
				for _, p := range rp {
					if m, ok := p.(map[string]any); ok {
						cart.RawProducts = append(cart.RawProducts, m)
					}
				}
			}
			return &cart
		}
	}
	return &Cart{Products: []CartProduct{}, RawProducts: []map[string]any{}}
}
