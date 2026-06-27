// Package basket implements the local.bio basket on top of the customer's
// server-side cart (`me.cart` for reads, `POST /customers/cart` for writes), so
// changes are shared with the website and mobile app.
//
// The write endpoint validates each line against the live offer and, on a
// mismatch, returns a 422 whose body is a per-line diff of
// `{field: [valueYouSent, expectedValue]}`. We use that as a correction oracle:
// build a best-guess line, post it, apply the server's expected values, retry.
// Existing lines are the server's own objects and round-trip unchanged. We never
// send `payed`/`setupPayment:true`, so no payment is ever initiated.
package basket

import (
	"context"
	"fmt"

	"github.com/nover/local-bio-mcp/internal/client"
	"github.com/nover/local-bio-mcp/internal/config"
)

const maxCorrectionPasses = 6

// Service mutates the customer's server-side cart.
type Service struct {
	Client *client.Client
	Cfg    *config.Config
}

// New builds a basket Service.
func New(c *client.Client, cfg *config.Config) *Service {
	return &Service{Client: c, Cfg: cfg}
}

// Get returns the current server-side cart.
func (s *Service) Get(ctx context.Context) (*client.Cart, error) {
	return s.Client.GetCart(ctx)
}

// Add inserts or increments a product line (by product id) and saves the cart.
func (s *Service) Add(ctx context.Context, productID string, qty int) (*client.Cart, error) {
	if qty <= 0 {
		qty = 1
	}
	cart, err := s.Client.GetCart(ctx)
	if err != nil {
		return nil, err
	}
	store := s.cartStore(cart)
	if store == "" {
		return nil, fmt.Errorf("no store selected (run `localbio store set <ref>`)")
	}
	payment, err := s.payment(ctx, cart)
	if err != nil {
		return nil, err
	}

	// Existing line → bump quantity on the server's own object and re-post.
	for _, p := range cart.RawProducts {
		if asString(p["productId"]) == productID {
			p["quantity"] = asFloat(p["quantity"]) + float64(qty)
			return s.Client.SaveCartProducts(ctx, store, payment, cart.RawProducts)
		}
	}

	// New line → best-guess + diff-correction loop.
	line, err := s.guessLine(ctx, store, productID, qty)
	if err != nil {
		return nil, err
	}
	products := append(cart.RawProducts, line)
	return s.saveWithCorrections(ctx, store, payment, products, line)
}

// Remove removes qty units of a product (qty <= 0 removes the whole line).
func (s *Service) Remove(ctx context.Context, productID string, qty int) (*client.Cart, error) {
	cart, err := s.Client.GetCart(ctx)
	if err != nil {
		return nil, err
	}
	out := cart.RawProducts[:0:0]
	found := false
	for _, p := range cart.RawProducts {
		if asString(p["productId"]) == productID {
			found = true
			if qty > 0 && asFloat(p["quantity"])-float64(qty) > 0 {
				p["quantity"] = asFloat(p["quantity"]) - float64(qty)
				out = append(out, p)
			}
			continue
		}
		out = append(out, p)
	}
	if !found {
		return nil, fmt.Errorf("product %q is not in the basket", productID)
	}
	payment, err := s.payment(ctx, cart)
	if err != nil {
		return nil, err
	}
	return s.Client.SaveCartProducts(ctx, s.cartStore(cart), payment, out)
}

// saveWithCorrections posts the cart, applying the server's expected values to
// the new line until it validates (or a non-correctable field like stock fails).
func (s *Service) saveWithCorrections(ctx context.Context, store string, payment map[string]any, products []map[string]any, line map[string]any) (*client.Cart, error) {
	productID := asString(line["productId"])
	name := asString(line["name"])
	for pass := 0; pass < maxCorrectionPasses; pass++ {
		cart, err := s.Client.SaveCartProducts(ctx, store, payment, products)
		if err == nil {
			return cart, nil
		}
		diffs, ok := client.ParseCartValidation(err)
		if !ok {
			return nil, err
		}
		corrected := false
		for _, d := range diffs {
			if d.ProductID != productID {
				continue // only correct our own line; existing lines are valid
			}
			for field, pair := range d.Fields {
				if field == "stock" {
					return nil, fmt.Errorf("%q is out of stock for the available delivery date", name)
				}
				if len(pair) < 2 {
					continue
				}
				if pair[1] == nil {
					delete(line, field)
				} else {
					line[field] = pair[1]
				}
				corrected = true
			}
		}
		if !corrected {
			return nil, err
		}
	}
	return nil, fmt.Errorf("could not add %q: the server kept rejecting the line", name)
}

// guessLine builds an initial cart line for a brand-new product from catalogue
// data. The diff-correction loop fixes server-computed fields (price, status…).
func (s *Service) guessLine(ctx context.Context, store, productID string, qty int) (map[string]any, error) {
	prod, err := s.Client.CatalogueProduct(ctx, store, productID)
	if err != nil {
		return nil, err
	}
	di, err := s.Client.ProducerDeliveryInfo(ctx, prod.ProducerID)
	if err != nil {
		return nil, err
	}
	status := "onsessionPayment"
	if di.Deferred {
		status = "waitingPayment"
	}
	line := map[string]any{
		"productId":             prod.ID,
		"quantity":              qty,
		"name":                  prod.Name,
		"categoryId":            prod.CategoryID,
		"producerId":            prod.ProducerID,
		"img":                   prod.Img,
		"about":                 false,
		"delivery":              di.Date,
		"tax":                   5.5,
		"status":                status,
		"alreadyTipped":         true,
		"paymentCostDiscounted": true,
	}
	// Packaging-priced items must carry their packaging object + price; weight
	// items (no packaging) let the diff-correction fill the product-level price.
	if len(prod.Packaging) > 0 {
		pk := prod.Packaging[0]
		line["price"] = pk.Price
		line["packaging"] = map[string]any{
			"id": pk.ID, "name": pk.Name, "price": pk.Price, "stock": pk.Stock, "weight": pk.Weight,
		}
	} else {
		line["price"] = 0
	}
	return line, nil
}

// payment returns the cart's payment object, building a default from the
// customer profile when the cart is empty.
func (s *Service) payment(ctx context.Context, cart *client.Cart) (map[string]any, error) {
	if cart.Payment != nil {
		return cart.Payment, nil
	}
	acc, err := s.Client.Me(ctx)
	if err != nil {
		return nil, err
	}
	pm := map[string]any{"appTip": 0, "storeTip": 0, "invoicing": false, "deferred": []any{}}
	if acc.Raw != nil {
		if t, ok := acc.Raw["appTip"]; ok {
			pm["appTip"] = t
		}
		if m, ok := acc.Raw["defaultPaymentMethod"].(string); ok && m != "" {
			pm["method"] = m
		}
	}
	return pm, nil
}

func (s *Service) cartStore(cart *client.Cart) string {
	if cart.StoreID != "" {
		return cart.StoreID
	}
	return s.Cfg.StoreID
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func asFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}
