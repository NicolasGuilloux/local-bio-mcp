package cli

import (
	"context"
	"errors"
	"fmt"
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

// client builds an API client from the current config. When LOCALBIO_EMAIL and
// LOCALBIO_PASSWORD are set, it logs in automatically (missing or expired
// session) and persists the new token.
func (a *app) client() *client.Client {
	return client.New(
		client.WithBaseURL(a.cfg.BaseURL),
		client.WithApp(a.cfg.App),
		client.WithToken(a.cfg.Token),
		client.WithCredentials(os.Getenv("LOCALBIO_EMAIL"), os.Getenv("LOCALBIO_PASSWORD")),
		client.WithTokenRefresh(func(token string) {
			a.cfg.Token = token
			a.cfg.Email = os.Getenv("LOCALBIO_EMAIL")
			if err := a.cfg.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not persist new session token: %v\n", err)
			}
		}),
	)
}

// requireAuth ensures there is a session, logging in from the environment
// credentials when possible.
func (a *app) requireAuth(ctx context.Context) error {
	if err := a.client().EnsureAuth(ctx); err != nil {
		if errors.Is(err, client.ErrNotLoggedIn) {
			return fmt.Errorf("not logged in (run `localbio login` or set LOCALBIO_EMAIL/LOCALBIO_PASSWORD)")
		}
		return err
	}
	return nil
}

func (a *app) jsonOutput() bool { return a.format == "json" }
