package client

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
)

// Login authenticates and returns the login response. The bearer token is
// extracted from the payload (the SPA reads it at `auth.id`).
func (c *Client) Login(ctx context.Context, email, password string) (*LoginResponse, error) {
	body := map[string]string{"email": email, "password": password}

	var raw json.RawMessage
	if err := c.do(ctx, http.MethodPost, "/customers/login", nil, body, &raw); err != nil {
		return nil, err
	}
	var lr LoginResponse
	_ = json.Unmarshal(raw, &lr)
	_ = json.Unmarshal(raw, &lr.Raw)
	// Robustly locate the token wherever the API puts it.
	if lr.TokenValue() == "" {
		lr.Token = findToken(lr.Raw)
	}
	return &lr, nil
}

// Logout invalidates the current session server-side. Errors are non-fatal for
// callers that simply want to clear local state.
func (c *Client) Logout(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/customers/logout", nil, nil, nil)
}

// Me returns the authenticated account.
func (c *Client) Me(ctx context.Context) (*Account, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, "/customers/me", nil, nil, &raw); err != nil {
		return nil, err
	}
	var acc Account
	_ = json.Unmarshal(raw, &acc)
	_ = json.Unmarshal(raw, &acc.Raw)
	return &acc, nil
}

// findToken walks a decoded login response looking for a session token. It
// checks the known field names used by the local.bio back-ends, preferring
// token-bearing objects (LoopBack access tokens expose the value at `.id`).
func findToken(m map[string]any) string {
	if m == nil {
		return ""
	}
	// Preferred explicit locations, in order.
	for _, key := range []string{"auth", "token", "accessToken", "access_token", "session"} {
		switch v := m[key].(type) {
		case string:
			if v != "" {
				return v
			}
		case map[string]any:
			if id, _ := v["id"].(string); id != "" {
				return id
			}
			if t, _ := v["token"].(string); t != "" {
				return t
			}
		}
	}
	// Some APIs return a bare access-token object: {id, ttl, userId, created}.
	if id, _ := m["id"].(string); id != "" {
		if _, hasTTL := m["ttl"]; hasTTL {
			return id
		}
		if _, hasUser := m["userId"]; hasUser {
			return id
		}
	}
	return ""
}

// topKeys returns the sorted top-level keys of a payload, for diagnostics.
func topKeys(m map[string]any) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
