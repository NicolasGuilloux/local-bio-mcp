package cli

import (
	"os"

	"github.com/nover/local-bio-mcp/internal/client"
	"github.com/nover/local-bio-mcp/internal/config"
	"github.com/nover/local-bio-mcp/internal/geocode"
)

// app carries shared state across cobra commands.
type app struct {
	cfg      *config.Config
	format   string // "text" | "json"
	geocoder *geocode.Geocoder
}

func newApp() (*app, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	// Environment overrides (useful for the Docker/MCP server).
	if v := os.Getenv("LOCALBIO_API_BASE"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("LOCALBIO_APP"); v != "" {
		cfg.App = v
	}
	if v := os.Getenv("LOCALBIO_TOKEN"); v != "" {
		cfg.Token = v
	}
	return &app{cfg: cfg, format: "text", geocoder: geocode.New()}, nil
}

// client builds an API client from the current config.
func (a *app) client() *client.Client {
	return client.New(
		client.WithBaseURL(a.cfg.BaseURL),
		client.WithApp(a.cfg.App),
		client.WithToken(a.cfg.Token),
	)
}

func (a *app) jsonOutput() bool { return a.format == "json" }
