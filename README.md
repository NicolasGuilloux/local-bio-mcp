# local-bio-mcp

A CLI **and** [MCP](https://modelcontextprotocol.io) server to drive a
[local.bio](https://www.local.bio) account from the terminal or from an LLM:
log in, pick a pickup point (*point de retrait*), search products, manage your
basket and review your orders.

Written in Go. Dev environment managed with [devenv](https://devenv.sh) +
[direnv](https://direnv.net). Ships as a static binary and a Docker image that
runs the MCP server over Streamable HTTP. The MCP server is also available over
stdio.

> ⚠️ Unofficial. This project talks to the public local.bio HTTP API the same
> way the website does (see [`docs/API.md`](docs/API.md)). Use it with your own
> account, responsibly.

## Install / build

```sh
# With devenv (recommended)
direnv allow          # or: devenv shell
build                 # -> ./bin/localbio

# Or plain Go
go build -o bin/localbio ./cmd/localbio
```

## CLI

```text
login                       Log in with your local.bio account
logout                      Log out and clear stored tokens
info                        Show info about your account
store set <ref>             Select your store
store search <query>        Search stores by city or postal code
orders                      List previous orders (most recent first)
orders <#|id>               Show order detail with articles (index or order id)
search [query]              List store products (no query) or filter them
basket get                  Show current basket contents
basket add <id> [qty]       Add a product (by product id) to your basket
basket remove <id> [qty]    Remove a product (default: remove all)
mcp                         Start MCP server (stdio transport)
mcp http [addr]             Start MCP server (Streamable HTTP, default :8080)
```

Every command accepts `--format json` for machine-readable output:

```sh
localbio store search Lyon --format json
localbio login --email me@example.com   # password prompted (or $LOCALBIO_PASSWORD)
localbio store set aaahh-la-ferme-
localbio search                         # all products available for the store
localbio search radis                   # filter (accent-insensitive)
localbio search --all                   # include inactive/out-of-stock items
localbio basket add 677bea76...88f94 2  # add by product id (ID column of search)
localbio basket get
localbio orders                         # newest first
localbio orders 1                       # detail of the most recent order
localbio basket add 3270190007890 2
localbio basket get
localbio orders
```

Notes specific to local.bio: there is **no EAN/barcode** — products are
referenced by their product id (the `ID` column of `search`). Orders have no
human number, so `orders` are addressed by a 1-based index or their id. The
basket is server-side (shared with the website/mobile app) — read from
`customers/me.cart`, written via `POST /customers/cart`.

State (session token, selected store and local basket) is stored in
`$LOCALBIO_CONFIG_DIR` (defaults to your OS config dir, e.g.
`~/.config/local-bio/config.json`, mode `0600`).

### Environment variables

| Variable             | Purpose                                            |
| -------------------- | -------------------------------------------------- |
| `LOCALBIO_API_BASE`  | API base URL (default `https://www.local.bio/api-v2`) |
| `LOCALBIO_APP`       | Tenant header (default `local.bio`)                |
| `LOCALBIO_TOKEN`     | Session token (overrides stored one)               |
| `LOCALBIO_EMAIL` / `LOCALBIO_PASSWORD` | Non-interactive login            |
| `LOCALBIO_CONFIG_DIR`| Where to persist config                            |

## MCP server

Two transports, both backed by the same tools:

```sh
localbio mcp                 # stdio (for Claude Desktop, etc.)
localbio mcp http :8080      # Streamable HTTP
```

Tools exposed: `localbio_login`, `localbio_logout`, `localbio_account_info`,
`localbio_store_search`, `localbio_store_set`, `localbio_product_search`,
`localbio_basket_get`, `localbio_basket_add`, `localbio_basket_remove`,
`localbio_orders_list`, `localbio_order_detail`. Tool results are returned as
JSON text, friendly for LLM consumption.

### Claude Desktop (stdio) example

```json
{
  "mcpServers": {
    "local-bio": {
      "command": "/path/to/bin/localbio",
      "args": ["mcp"]
    }
  }
}
```

## Docker

```sh
docker build -t local-bio-mcp .
docker run --rm -p 8080:8080 -v localbio:/data local-bio-mcp
# -> MCP Streamable HTTP on http://localhost:8080
```

Pre-built images are published to GHCR:
`ghcr.io/<owner>/local-bio-mcp` — `latest`/`edge`/`sha-<short>` on every push to
`main`, and `vX.Y.Z` / `X.Y` on tagged releases.

```sh
docker run --rm -p 8080:8080 -v localbio:/data ghcr.io/<owner>/local-bio-mcp:latest
```

## Development

```sh
devenv shell        # Go toolchain + helper scripts
build               # build the binary -> bin/localbio
run <args>          # go run the CLI
test                # go test ./...
lint                # gofumpt + go vet + golangci-lint
mcp-stdio           # run the MCP server (stdio)
mcp-http [addr]     # run the MCP server (HTTP)
docker-build        # build the Docker image
```

A Chrome instance reachable over CDP (`http://localhost:9222`) is available for
**exploration only** (`cdp` helper script). The CLI itself never uses it — it
only speaks plain HTTP to the public API.

## Project layout

```
cmd/localbio          entrypoint (version, wiring)
internal/client       HTTP API client (auth, stores, products, basket, orders)
internal/config       on-disk session/store state
internal/geocode      city/postal-code -> coordinates (api-adresse.data.gouv.fr)
internal/cli          cobra command tree + output formatting
internal/mcp          MCP server + tools (stdio & Streamable HTTP)
docs/API.md           reverse-engineering notes
```

## License

MIT
