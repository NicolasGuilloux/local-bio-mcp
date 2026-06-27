// Package mcp exposes the local.bio features as Model Context Protocol tools,
// over either stdio or Streamable HTTP.
package mcp

import (
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/nover/local-bio-mcp/internal/client"
	"github.com/nover/local-bio-mcp/internal/config"
	"github.com/nover/local-bio-mcp/internal/geocode"
)

// deps gives each tool handler access to config + clients, re-read per call so
// the server always reflects the latest stored login/store.
type deps struct {
	geocoder *geocode.Geocoder
}

func (d *deps) load() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if v := os.Getenv("LOCALBIO_API_BASE"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("LOCALBIO_APP"); v != "" {
		cfg.App = v
	}
	if v := os.Getenv("LOCALBIO_TOKEN"); v != "" {
		cfg.Token = v
	}
	return cfg, nil
}

func (d *deps) client(cfg *config.Config) *client.Client {
	return client.New(
		client.WithBaseURL(cfg.BaseURL),
		client.WithApp(cfg.App),
		client.WithToken(cfg.Token),
	)
}

// NewServer builds the MCP server with every local.bio tool registered.
func NewServer(version string) *server.MCPServer {
	s := server.NewMCPServer(
		"local-bio-mcp",
		version,
		server.WithToolCapabilities(true),
		server.WithInstructions(
			"Tools to drive a local.bio account: log in, pick a pickup point (store), "+
				"search products, manage the basket and review orders. "+
				"Call login first; the session token and selected store are persisted between calls.",
		),
	)
	d := &deps{geocoder: geocode.New()}
	registerTools(s, d)
	return s
}

// ServeStdio runs the server over stdio (default MCP transport).
func ServeStdio(version string) error {
	return server.ServeStdio(NewServer(version))
}

// ServeHTTP runs the Streamable HTTP transport on addr (e.g. ":8080").
func ServeHTTP(version, addr string) error {
	s := NewServer(version)
	httpSrv := server.NewStreamableHTTPServer(s)
	return httpSrv.Start(addr)
}
