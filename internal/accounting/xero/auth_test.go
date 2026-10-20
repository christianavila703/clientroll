package xero

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestBuildAuthorizeURL(t *testing.T) {
	u := BuildAuthorizeURL("client-id", "https://app.example.com/api/xero/oauth/callback", "state123", []string{
		"openid", "accounting.contacts", "offline_access",
	})
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	q := parsed.Query()
	if q.Get("client_id") != "client-id" {
		t.Fatalf("unexpected client_id: %s", q.Get("client_id"))
	}
	if q.Get("state") != "state123" {
		t.Fatalf("unexpected state: %s", q.Get("state"))
	}
	if !strings.Contains(q.Get("scope"), "offline_access") {
		t.Fatalf("scope missing expected permission: %s", q.Get("scope"))
	}
}

func TestTokenExpiry_DefaultsToThirtyMinutes(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	resp := TokenResponse{ExpiresIn: 0}
	exp := resp.Expiry(now)
	if !exp.Equal(now.Add(30 * time.Minute)) {
		t.Fatalf("expected 30m expiry, got %s", exp.Sub(now))
	}
}
