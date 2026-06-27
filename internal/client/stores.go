package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// SearchStores returns the pickup points around a coordinate.
func (c *Client) SearchStores(ctx context.Context, lat, lng float64) (*StoreSearchResult, error) {
	q := url.Values{}
	q.Set("lat", strconv.FormatFloat(lat, 'f', -1, 64))
	q.Set("lng", strconv.FormatFloat(lng, 'f', -1, 64))

	var res StoreSearchResult
	if err := c.do(ctx, http.MethodGet, "/stores/search", q, nil, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// LoadStore returns the full detail of a single store by its url/id.
func (c *Client) LoadStore(ctx context.Context, storeID string) (map[string]any, error) {
	q := url.Values{}
	q.Set("storeId", storeID)

	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, "/stores/loadStore", q, nil, &raw); err != nil {
		return nil, err
	}
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out, nil
}
