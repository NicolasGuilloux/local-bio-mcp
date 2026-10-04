// Package client is a thin, dependency-free HTTP client for the local.bio
// (`/api-v2`) back-end. It is the single source of truth for every endpoint the
// CLI and the MCP server use.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

// DefaultBaseURL is the NestJS API used by the modern front-end.
const DefaultBaseURL = "https://www.local.bio/api-v2"

// DefaultLegacyBaseURL is the older LoopBack API. A few endpoints (order detail,
// place/cancel order) still live there.
const DefaultLegacyBaseURL = "https://www.local.bio/api"

// DefaultApp is the tenant identifier sent in the `app` header.
const DefaultApp = "local.bio"

// Client talks to the local.bio REST API.
type Client struct {
	BaseURL       string
	LegacyBaseURL string
	App           string
	Token         string
	Debug         bool

	// Email/Password, when set, let the client log in on its own: lazily via
	// EnsureAuth, and transparently when a request comes back 401.
	Email    string
	Password string
	// OnTokenRefresh is called with the new token after an automatic login, so
	// callers can persist it.
	OnTokenRefresh func(token string)

	http *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithToken sets the bearer token used for authenticated calls.
func WithToken(t string) Option { return func(c *Client) { c.Token = t } }

// WithCredentials enables automatic (re-)login with the given account. Empty
// values disable it.
func WithCredentials(email, password string) Option {
	return func(c *Client) { c.Email, c.Password = email, password }
}

// WithTokenRefresh registers a callback invoked after an automatic login.
func WithTokenRefresh(fn func(token string)) Option {
	return func(c *Client) { c.OnTokenRefresh = fn }
}

// WithBaseURL overrides the API base URL. The legacy base is derived from it
// (…/api-v2 → …/api) unless set explicitly via WithLegacyBaseURL.
func WithBaseURL(u string) Option {
	return func(c *Client) {
		if u != "" {
			c.BaseURL = strings.TrimRight(u, "/")
			c.LegacyBaseURL = deriveLegacyBase(c.BaseURL)
		}
	}
}

// WithLegacyBaseURL overrides the legacy (LoopBack) API base URL.
func WithLegacyBaseURL(u string) Option {
	return func(c *Client) {
		if u != "" {
			c.LegacyBaseURL = strings.TrimRight(u, "/")
		}
	}
}

// deriveLegacyBase maps an api-v2 base URL to its legacy counterpart.
func deriveLegacyBase(base string) string {
	if strings.HasSuffix(base, "/api-v2") {
		return strings.TrimSuffix(base, "-v2")
	}
	return DefaultLegacyBaseURL
}

// WithApp overrides the tenant header.
func WithApp(a string) Option {
	return func(c *Client) {
		if a != "" {
			c.App = a
		}
	}
}

// WithHTTPClient injects a custom *http.Client (handy for tests).
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithDebug enables verbose request/response logging to stderr.
func WithDebug(d bool) Option { return func(c *Client) { c.Debug = d } }

// New builds a Client.
func New(opts ...Option) *Client {
	c := &Client{
		BaseURL:       DefaultBaseURL,
		LegacyBaseURL: DefaultLegacyBaseURL,
		App:           DefaultApp,
		http:          &http.Client{Timeout: 30 * time.Second},
		Debug:         os.Getenv("LOCALBIO_DEBUG") != "",
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// APIError is a non-2xx response.
type APIError struct {
	StatusCode int
	Status     string
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("local.bio API %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("local.bio API %d: %s", e.StatusCode, e.Status)
}

// IsUnauthorized reports whether an error is a 401.
func IsUnauthorized(err error) bool {
	ae, ok := err.(*APIError)
	return ok && ae.StatusCode == http.StatusUnauthorized
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	return c.doBase(ctx, c.BaseURL, method, path, query, body, out)
}

// doLegacy issues a request against the legacy LoopBack base URL.
func (c *Client) doLegacy(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	return c.doBase(ctx, c.LegacyBaseURL, method, path, query, body, out)
}

// HasCredentials reports whether the client can log in on its own.
func (c *Client) HasCredentials() bool { return c.Email != "" && c.Password != "" }

// ErrNotLoggedIn is returned by EnsureAuth when there is neither a token nor
// credentials to obtain one.
var ErrNotLoggedIn = errors.New("not logged in")

// EnsureAuth makes sure the client holds a token, logging in with the
// configured credentials when it does not.
func (c *Client) EnsureAuth(ctx context.Context) error {
	if c.Token != "" {
		return nil
	}
	if !c.HasCredentials() {
		return ErrNotLoggedIn
	}
	return c.relogin(ctx)
}

// relogin obtains a fresh token with the configured credentials.
func (c *Client) relogin(ctx context.Context) error {
	lr, err := c.Login(ctx, c.Email, c.Password)
	if err != nil {
		return fmt.Errorf("automatic login: %w", err)
	}
	token := lr.TokenValue()
	if token == "" {
		return fmt.Errorf("automatic login: no token in response (fields: %s)", lr.Fields())
	}
	c.Token = token
	if c.OnTokenRefresh != nil {
		c.OnTokenRefresh(token)
	}
	return nil
}

func (c *Client) doBase(ctx context.Context, base, method, path string, query url.Values, body any, out any) error {
	err := c.doOnce(ctx, base, method, path, query, body, out)
	// Session expired (or never existed): log in again and retry once. The
	// login endpoint itself is excluded so bad credentials don't loop.
	if IsUnauthorized(err) && c.HasCredentials() && path != loginPath {
		if c.Debug {
			fmt.Fprintf(os.Stderr, "[localbio] 401 on %s %s, logging in again\n", method, path)
		}
		if lerr := c.relogin(ctx); lerr != nil {
			return lerr
		}
		return c.doOnce(ctx, base, method, path, query, body, out)
	}
	return err
}

func (c *Client) doOnce(ctx context.Context, base, method, path string, query url.Values, body any, out any) error {
	u := base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		rdr = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("app", c.App)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", c.Token)
	}

	if c.Debug {
		fmt.Fprintf(os.Stderr, "[localbio] -> %s %s\n", method, u)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if c.Debug {
		fmt.Fprintf(os.Stderr, "[localbio] <- %d %s\n%s\n", resp.StatusCode, resp.Header.Get("Content-Type"), redact(data))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Message:    extractMessage(data),
			Body:       string(data),
		}
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// redact masks obvious secret values when debug-logging response bodies.
func redact(data []byte) string {
	s := string(data)
	if len(s) > 4096 {
		s = s[:4096] + "…(truncated)"
	}
	return s
}

// formatCartValidation turns the cart endpoint's `[[line, diff], ...]` 422 body
// into a readable message naming the offending products and fields.
func formatCartValidation(v []any) string {
	var parts []string
	for _, item := range v {
		pair, ok := item.([]any)
		if !ok || len(pair) < 2 {
			return ""
		}
		line, _ := pair[0].(map[string]any)
		diff, ok := pair[1].(map[string]any)
		if !ok {
			return ""
		}
		name, _ := line["name"].(string)
		if name == "" {
			name, _ = line["productId"].(string)
		}
		fields := make([]string, 0, len(diff))
		for k := range diff {
			fields = append(fields, k)
		}
		sort.Strings(fields)
		parts = append(parts, fmt.Sprintf("%q not orderable (mismatch: %s)", name, strings.Join(fields, ", ")))
	}
	return strings.Join(parts, "; ")
}

// extractMessage best-effort pulls a human message out of an error body. It
// tolerates both LoopBack (`{error:{message}}`) and NestJS (`{message, error}`)
// shapes, where `message` may be a string or a nested validation array.
func extractMessage(data []byte) string {
	var m map[string]any
	if json.Unmarshal(data, &m) != nil {
		return ""
	}
	switch v := m["message"].(type) {
	case string:
		if v != "" {
			return v
		}
	case []any:
		if msg := formatCartValidation(v); msg != "" {
			return msg
		}
		parts := make([]string, 0, len(v))
		for _, p := range v {
			parts = append(parts, fmt.Sprint(p))
		}
		if len(parts) > 0 {
			return strings.Join(parts, "; ")
		}
	}
	// LoopBack: {"error":{"message":"..."}}
	if e, ok := m["error"].(map[string]any); ok {
		if msg, _ := e["message"].(string); msg != "" {
			return msg
		}
	}
	return ""
}
