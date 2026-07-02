package payment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestGCClientRetriesOnceOn429(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limit"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client := &gcClient{
		baseURL:    srv.URL,
		apiKey:     "test_key",
		httpClient: srv.Client(),
	}

	var out map[string]bool
	if err := client.do(context.Background(), http.MethodGet, "", "", nil, &out); err != nil {
		t.Fatalf("expected retry to succeed, got error: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected 2 calls (initial + retry), got %d", calls.Load())
	}
	if !out["ok"] {
		t.Fatalf("unexpected response body: %#v", out)
	}
}

func TestGCClientSetsIdempotencyHeader(t *testing.T) {
	var gotIdempotency string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIdempotency = r.Header.Get("Idempotency-Key")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client := &gcClient{
		baseURL:    srv.URL,
		apiKey:     "test_key",
		httpClient: srv.Client(),
	}
	var out map[string]bool
	if err := client.do(context.Background(), http.MethodPost, "", "idem-key-1", map[string]string{"a": "b"}, &out); err != nil {
		t.Fatalf("expected post to succeed, got error: %v", err)
	}
	if gotIdempotency != "idem-key-1" {
		t.Fatalf("expected idempotency header to be set, got %q", gotIdempotency)
	}
}
