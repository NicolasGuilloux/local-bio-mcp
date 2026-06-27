package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// LoadProducer returns the raw producer payload (`producers[0]`) including its
// `products` catalogue.
func (c *Client) LoadProducer(ctx context.Context, producerID string) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("producerId", producerID)

	var wrapped struct {
		Producers []json.RawMessage `json:"producers"`
	}
	if err := c.do(ctx, http.MethodGet, "/producers/loadProducer", q, nil, &wrapped); err != nil {
		return nil, err
	}
	if len(wrapped.Producers) == 0 {
		return nil, fmt.Errorf("producer %q not found", producerID)
	}
	return wrapped.Producers[0], nil
}

// StoreCatalogue returns every product sold at a store. A store relays one or
// more producers; `loadStore` returns them all with the store's real per-product
// stock (unlike `loadProducer`, which is a single producer's full catalogue).
func (c *Client) StoreCatalogue(ctx context.Context, storeRef string) ([]Product, error) {
	q := url.Values{}
	q.Set("storeId", storeRef)
	var resp struct {
		Producers []struct {
			ID       string    `json:"id"`
			Products []Product `json:"products"`
		} `json:"producers"`
	}
	if err := c.do(ctx, http.MethodGet, "/stores/loadStore", q, nil, &resp); err != nil {
		return nil, err
	}
	var out []Product
	for _, pr := range resp.Producers {
		for _, p := range pr.Products {
			if p.ProducerID == "" {
				p.ProducerID = pr.ID
			}
			out = append(out, p)
		}
	}
	return out, nil
}

// SearchProducts lists the products of the selected store, optionally filtered
// by a free-text query (matched against name, description and category). When
// availableOnly is true, only active products are returned.
func (c *Client) SearchProducts(ctx context.Context, query, storeRef string, availableOnly bool) ([]Product, error) {
	if storeRef == "" {
		return nil, fmt.Errorf("no store selected: run `store set <ref>` first")
	}
	all, err := c.StoreCatalogue(ctx, storeRef)
	if err != nil {
		return nil, err
	}
	q := fold(query)
	out := make([]Product, 0, len(all))
	for _, p := range all {
		if availableOnly && (!p.Active || !p.InStock()) {
			continue
		}
		if q != "" && !productMatches(p, q) {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

func productMatches(p Product, q string) bool {
	return strings.Contains(fold(p.Name), q) ||
		strings.Contains(fold(p.Description), q) ||
		strings.Contains(fold(p.CategoryID), q)
}

// fold lower-cases and strips common French diacritics for lenient matching.
func fold(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	r := strings.NewReplacer(
		"à", "a", "â", "a", "ä", "a",
		"é", "e", "è", "e", "ê", "e", "ë", "e",
		"î", "i", "ï", "i",
		"ô", "o", "ö", "o",
		"ù", "u", "û", "u", "ü", "u",
		"ç", "c",
	)
	return r.Replace(s)
}

// DeliveryInfo describes the next orderable delivery for a store's producer and
// the payment mode that applies, needed to build a valid cart line.
type DeliveryInfo struct {
	Date          string
	Markup        float64
	MinSales      float64
	OnlinePayment bool
	PaymentChoice bool
	Deferred      bool
}

// ImmediatePayment reports whether adding to cart triggers an immediate online
// payment (onsessionPayment) rather than pay-on-delivery (deferred).
func (d DeliveryInfo) ImmediatePayment() bool {
	return d.OnlinePayment && !d.PaymentChoice && !d.Deferred
}

// ProducerDeliveryInfo returns the next orderable delivery + payment mode for a
// producer (each producer relayed by a store has its own delivery schedule).
func (c *Client) ProducerDeliveryInfo(ctx context.Context, producerID string) (*DeliveryInfo, error) {
	raw, err := c.LoadProducer(ctx, producerID)
	if err != nil {
		return nil, err
	}
	var p struct {
		Deliveries []struct {
			NextDates       []string `json:"nextDates"`
			Markup          float64  `json:"markup"`
			MinSales        float64  `json:"minSales"`
			OnlinePayment   bool     `json:"onlinePayment"`
			PaymentChoice   bool     `json:"paymentChoice"`
			DeferredPayment any      `json:"deferredPayment"`
		} `json:"deliveries"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	for _, d := range p.Deliveries {
		if len(d.NextDates) == 0 {
			continue
		}
		return &DeliveryInfo{
			Date:          d.NextDates[0],
			Markup:        d.Markup,
			MinSales:      d.MinSales,
			OnlinePayment: d.OnlinePayment,
			PaymentChoice: d.PaymentChoice,
			Deferred:      d.DeferredPayment != nil,
		}, nil
	}
	return nil, fmt.Errorf("producer %q has no upcoming delivery to order against", producerID)
}

// CatalogueProduct returns a single product of a store's catalogue by id.
func (c *Client) CatalogueProduct(ctx context.Context, storeRef, productID string) (*Product, error) {
	cat, err := c.StoreCatalogue(ctx, storeRef)
	if err != nil {
		return nil, err
	}
	for i := range cat {
		if cat[i].ID == productID {
			return &cat[i], nil
		}
	}
	return nil, fmt.Errorf("product %q not found in store %q catalogue (use `search` to find its id)", productID, storeRef)
}
