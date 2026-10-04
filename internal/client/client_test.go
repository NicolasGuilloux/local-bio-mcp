package client

import (
	"context"
	"errors"
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

func TestReloginOn401(t *testing.T) {
	var logins, meCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/customers/login":
			logins++
			_, _ = w.Write([]byte(`{"id":"fresh","ttl":1209600,"userId":"u1"}`))
		case "/customers/me":
			meCalls++
			if r.Header.Get("Authorization") != "fresh" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"message":"expired"}`))
				return
			}
			_, _ = w.Write([]byte(`{"email":"a@b.c"}`))
		}
	}))
	defer srv.Close()

	var refreshed string
	c := New(WithBaseURL(srv.URL), WithToken("stale"),
		WithCredentials("a@b.c", "pw"),
		WithTokenRefresh(func(tok string) { refreshed = tok }))
	acc, err := c.Me(context.Background())
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	if acc.Email != "a@b.c" || logins != 1 || meCalls != 2 {
		t.Fatalf("email=%q logins=%d meCalls=%d", acc.Email, logins, meCalls)
	}
	if c.Token != "fresh" || refreshed != "fresh" {
		t.Fatalf("token=%q refreshed=%q", c.Token, refreshed)
	}
}

func TestNoReloginWithoutCredentials(t *testing.T) {
	var logins int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/customers/login" {
			logins++
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	c := New(WithBaseURL(srv.URL), WithToken("stale"))
	if _, err := c.Me(context.Background()); !IsUnauthorized(err) {
		t.Fatalf("expected 401, got %v", err)
	}
	if logins != 0 {
		t.Fatalf("unexpected login attempts: %d", logins)
	}
}

func TestBadCredentialsDoNotLoop(t *testing.T) {
	var logins int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/customers/login" {
			logins++
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	c := New(WithBaseURL(srv.URL), WithToken("stale"), WithCredentials("a@b.c", "wrong"))
	if _, err := c.Me(context.Background()); err == nil {
		t.Fatal("expected an error")
	}
	if logins != 1 {
		t.Fatalf("logins = %d, want 1", logins)
	}
}

func TestEnsureAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"tok","ttl":1,"userId":"u1"}`))
	}))
	defer srv.Close()
	if err := New(WithBaseURL(srv.URL)).EnsureAuth(context.Background()); !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("want ErrNotLoggedIn, got %v", err)
	}
	c := New(WithBaseURL(srv.URL), WithCredentials("a@b.c", "pw"))
	if err := c.EnsureAuth(context.Background()); err != nil || c.Token != "tok" {
		t.Fatalf("err=%v token=%q", err, c.Token)
	}
}
