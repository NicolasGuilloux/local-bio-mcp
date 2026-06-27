// Package geocode resolves a free-text query (city or postal code) into
// coordinates using the official French government address API — the same
// provider whitelisted in the local.bio CSP.
package geocode

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const endpoint = "https://api-adresse.data.gouv.fr/search/"

// Place is a geocoding result.
type Place struct {
	Label    string  `json:"label"`
	City     string  `json:"city,omitempty"`
	Postcode string  `json:"postcode,omitempty"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
}

// Geocoder resolves places.
type Geocoder struct {
	http *http.Client
}

// New builds a Geocoder.
func New() *Geocoder {
	return &Geocoder{http: &http.Client{Timeout: 15 * time.Second}}
}

// Lookup returns the best match for query.
func (g *Geocoder) Lookup(ctx context.Context, query string) (*Place, error) {
	q := url.Values{}
	q.Set("q", query)
	q.Set("limit", "1")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := g.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("geocode %q: %w", query, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geocode %q: HTTP %d", query, resp.StatusCode)
	}

	var fc struct {
		Features []struct {
			Geometry struct {
				Coordinates []float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties struct {
				Label    string `json:"label"`
				City     string `json:"city"`
				Postcode string `json:"postcode"`
			} `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("geocode decode: %w", err)
	}
	if len(fc.Features) == 0 {
		return nil, fmt.Errorf("no location found for %q", query)
	}
	f := fc.Features[0]
	if len(f.Geometry.Coordinates) < 2 {
		return nil, fmt.Errorf("geocode %q: missing coordinates", query)
	}
	return &Place{
		Label:    f.Properties.Label,
		City:     f.Properties.City,
		Postcode: f.Properties.Postcode,
		Lng:      f.Geometry.Coordinates[0],
		Lat:      f.Geometry.Coordinates[1],
	}, nil
}
