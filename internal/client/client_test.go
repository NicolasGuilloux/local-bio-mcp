package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFindToken(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]any
		want string
	}{
		{"loopback access token", map[string]any{"id": "TOK", "ttl": float64(1209600), "userId": "u1"}, "TOK"},
		{"nested auth.id", map[string]any{"auth": map[string]any{"id": "TOK"}}, "TOK"},
		{"token string", map[string]any{"token": "TOK"}, "TOK"},
		{"accessToken string", map[string]any{"accessToken": "TOK"}, "TOK"},
		{"bare id without ttl is not a token", map[string]any{"id": "u1", "email": "x"}, ""},
		{"empty", map[string]any{}, ""},
	}
	for _, c := range cases {
		if got := findToken(c.in); got != c.want {
			t.Errorf("%s: findToken = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestLoginExtractsLoopbackToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/customers/login" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"id":"abc123","ttl":1209600,"created":"2026-01-01T00:00:00Z","userId":"u1"}`))
	}))
	defer srv.Close()
	c := New(WithBaseURL(srv.URL))
	lr, err := c.Login(context.Background(), "a@b.c", "pw")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if lr.TokenValue() != "abc123" {
		t.Fatalf("token = %q, want abc123", lr.TokenValue())
	}
}

func TestAPIErrorMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"missing token","statusCode":401}`))
	}))
	defer srv.Close()
	c := New(WithBaseURL(srv.URL))
	_, err := c.Me(context.Background())
	if !IsUnauthorized(err) {
		t.Fatalf("expected unauthorized, got %v", err)
	}
	if got := err.Error(); got != "local.bio API 401: missing token" {
		t.Fatalf("unexpected error string: %q", got)
	}
}

func TestDeriveLegacyBase(t *testing.T) {
	c := New(WithBaseURL("https://www.local.bio/api-v2"))
	if c.LegacyBaseURL != "https://www.local.bio/api" {
		t.Fatalf("legacy base = %q, want https://www.local.bio/api", c.LegacyBaseURL)
	}
}
