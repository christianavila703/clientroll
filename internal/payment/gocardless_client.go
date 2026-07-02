package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	gocardlessAPIVersion = "2015-07-06"
	gocardlessSandboxURL = "https://api-sandbox.gocardless.com"
	gocardlessLiveURL    = "https://api.gocardless.com"
)

// gcClient is the internal HTTP client for GoCardless API.
type gcClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func newGCClient(apiKey, environment string) *gcClient {
	baseURL := gocardlessSandboxURL
	if strings.ToLower(environment) == "live" {
		baseURL = gocardlessLiveURL
	}
	return &gcClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GCError represents a GoCardless API error response.
type GCError struct {
	StatusCode int
	Type       string
	Code       string
	Message    string
	RequestID  string
	Raw        string
}

func (e *GCError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("GoCardless API error (HTTP %d): %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("GoCardless API error: HTTP %d %s", e.StatusCode, e.Raw)
}

// do sends an authenticated request and decodes the JSON response into out.
// If the response is an error (4xx/5xx), returns GCError.
// idempotencyKey is optional; when set it is sent as Idempotency-Key header (for POST).
func (c *gcClient) do(ctx context.Context, method, path, idempotencyKey string, body interface{}, out interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("GoCardless-Version", gocardlessAPIVersion)
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 429 {
		// Rate limited: retry once after short backoff
		time.Sleep(2 * time.Second)
		var retryBody io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			retryBody = bytes.NewReader(b)
		}
		req2, _ := http.NewRequestWithContext(ctx, method, c.baseURL+path, retryBody)
		req2.Header.Set("Authorization", "Bearer "+c.apiKey)
		req2.Header.Set("GoCardless-Version", gocardlessAPIVersion)
		req2.Header.Set("Content-Type", "application/json")
		if idempotencyKey != "" {
			req2.Header.Set("Idempotency-Key", idempotencyKey)
		}
		resp2, err := c.httpClient.Do(req2)
		if err != nil {
			return fmt.Errorf("request retry: %w", err)
		}
		defer resp2.Body.Close()
		raw, _ = io.ReadAll(resp2.Body)
		resp = resp2
	}
	if resp.StatusCode >= 400 {
		var errBody struct {
			Error struct {
				Type      string `json:"type"`
				Code      string `json:"code"`
				Message   string `json:"message"`
				RequestID string `json:"request_id"`
			} `json:"error"`
		}
		_ = json.Unmarshal(raw, &errBody)
		return &GCError{
			StatusCode: resp.StatusCode,
			Type:      errBody.Error.Type,
			Code:      errBody.Error.Code,
			Message:   errBody.Error.Message,
			RequestID: errBody.Error.RequestID,
			Raw:       string(raw),
		}
	}

	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// post is a shorthand for POST with a body and optional idempotency key.
func (c *gcClient) post(ctx context.Context, path, idempotencyKey string, body interface{}, out interface{}) error {
	return c.do(ctx, http.MethodPost, path, idempotencyKey, body, out)
}

// get is a shorthand for GET.
func (c *gcClient) get(ctx context.Context, path string, out interface{}) error {
	return c.do(ctx, http.MethodGet, path, "", nil, out)
}

// getWithParams runs GET with query parameters (e.g. for list endpoints).
func (c *gcClient) getWithParams(ctx context.Context, path string, params url.Values, out interface{}) error {
	if len(params) > 0 {
		path = path + "?" + params.Encode()
	}
	return c.do(ctx, http.MethodGet, path, "", nil, out)
}
