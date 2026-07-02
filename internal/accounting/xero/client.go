package xero

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("xero request failed status=%d body=%s", e.StatusCode, e.Body)
}

type Client struct {
	HTTPClient    *http.Client
	AccessToken   string
	XeroTenantID  string
	MaxRetryOn429 int
}

func NewClient(httpClient *http.Client, accessToken, xeroTenantID string) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		HTTPClient:    httpClient,
		AccessToken:   accessToken,
		XeroTenantID:  xeroTenantID,
		MaxRetryOn429: 2,
	}
}

func (c *Client) doJSON(ctx context.Context, method, url string, body any, out any) (int, []byte, error) {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
	}

	attempt := 0
	for {
		req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(payload))
		if err != nil {
			return 0, nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.AccessToken)
		req.Header.Set("Accept", "application/json")
		if c.XeroTenantID != "" {
			req.Header.Set("Xero-tenant-id", c.XeroTenantID)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return 0, nil, err
		}
		raw, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return 0, nil, fmt.Errorf("read xero response: %w", readErr)
		}

		if resp.StatusCode == http.StatusTooManyRequests && attempt < c.MaxRetryOn429 {
			attempt++
			sleep := retryAfter(resp.Header.Get("Retry-After"))
			time.Sleep(sleep)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return resp.StatusCode, raw, &APIError{StatusCode: resp.StatusCode, Body: truncate(raw, 512)}
		}
		if out != nil {
			if err := json.Unmarshal(raw, out); err != nil {
				return resp.StatusCode, raw, fmt.Errorf("parse xero json response: %w", err)
			}
		}
		return resp.StatusCode, raw, nil
	}
}

func retryAfter(value string) time.Duration {
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return 1500 * time.Millisecond
	}
	return time.Duration(n) * time.Second
}
