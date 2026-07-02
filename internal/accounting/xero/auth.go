package xero

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type AuthClient struct {
	HTTPClient *http.Client
}

func NewAuthClient(httpClient *http.Client) *AuthClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &AuthClient{HTTPClient: httpClient}
}

func BuildAuthorizeURL(clientID, redirectURI, state string, scopes []string) string {
	scope := strings.TrimSpace(strings.Join(scopes, " "))
	q := url.Values{
		"response_type": {"code"},
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"scope":         {scope},
		"state":         {state},
	}
	return AuthBaseURL + "?" + q.Encode()
}

func (c *AuthClient) ExchangeCode(ctx context.Context, clientID, clientSecret, code, redirectURI string) (*TokenResponse, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
	}
	return c.tokenRequest(ctx, clientID, clientSecret, form)
}

func (c *AuthClient) RefreshToken(ctx context.Context, clientID, clientSecret, refreshToken string) (*TokenResponse, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	return c.tokenRequest(ctx, clientID, clientSecret, form)
}

func (c *AuthClient) tokenRequest(ctx context.Context, clientID, clientSecret string, form url.Values) (*TokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Basic "+basicAuth(clientID, clientSecret))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("read xero token response: %w", readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("xero token request failed status=%d body=%s", resp.StatusCode, truncate(raw, 512))
	}

	var out TokenResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parse xero token response: %w", err)
	}
	if out.AccessToken == "" {
		return nil, fmt.Errorf("xero token response missing access_token")
	}
	return &out, nil
}

func basicAuth(clientID, clientSecret string) string {
	return base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
}

func truncate(b []byte, max int) string {
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "...(truncated)"
}
