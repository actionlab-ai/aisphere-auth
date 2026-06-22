package casdoor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/actionlab-ai/aisphere-auth/internal/config"
)

type HTTPClient struct {
	cfg          config.CasdoorConfig
	httpClient   *http.Client
	tokenMu      sync.Mutex
	serviceToken string
	tokenExpiry  time.Time
}

func NewHTTPClient(cfg config.CasdoorConfig) *HTTPClient {
	return &HTTPClient{cfg: cfg, httpClient: &http.Client{Timeout: 15 * time.Second}}
}

func (c *HTTPClient) GetLoginURL(state string, redirectURI string, scopes []string) (string, error) {
	u, err := url.Parse(strings.TrimRight(c.cfg.Endpoint, "/") + "/login/oauth/authorize")
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client_id", c.cfg.ClientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	if len(scopes) > 0 {
		q.Set("scope", strings.Join(scopes, " "))
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (c *HTTPClient) ExchangeCode(ctx context.Context, code string) (*TokenSet, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", c.cfg.RedirectURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.cfg.Endpoint, "/")+"/api/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var out struct {
		AccessToken  string `json:"access_token"`
		IDToken      string `json:"id_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		Error        string `json:"error"`
		Description  string `json:"error_description"`
	}
	if err := c.doJSON(req, &out); err != nil {
		return nil, err
	}
	if out.Error != "" {
		return nil, fmt.Errorf("casdoor token error: %s %s", out.Error, out.Description)
	}
	return &TokenSet{
		AccessToken:  out.AccessToken,
		IDToken:      out.IDToken,
		RefreshToken: out.RefreshToken,
		TokenType:    out.TokenType,
		ExpiresAt:    time.Now().Add(time.Duration(out.ExpiresIn) * time.Second),
	}, nil
}

func (c *HTTPClient) GetUserInfo(ctx context.Context, bearer string) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.cfg.Endpoint, "/")+"/api/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	var out struct {
		Sub         string   `json:"sub"`
		Name        string   `json:"preferred_username"`
		DisplayName string   `json:"name"`
		Email       string   `json:"email"`
		Roles       []string `json:"roles"`
		Groups      []string `json:"groups"`
		Owner       string   `json:"owner"`
		ID          string   `json:"id"`
	}
	if err := c.doJSON(req, &out); err != nil {
		return nil, err
	}
	owner := out.Owner
	name := out.Name
	if owner == "" || name == "" {
		parts := strings.Split(out.Sub, "/")
		if len(parts) == 2 {
			if owner == "" {
				owner = parts[0]
			}
			if name == "" {
				name = parts[1]
			}
		}
	}
	return &UserInfo{
		ID:          firstNonEmpty(out.ID, out.Sub),
		Owner:       owner,
		Name:        name,
		DisplayName: out.DisplayName,
		Email:       out.Email,
		Roles:       out.Roles,
		Groups:      out.Groups,
	}, nil
}

func (c *HTTPClient) GetLogoutURL(postLogoutRedirectURI string) (string, error) {
	u, err := url.Parse(strings.TrimRight(c.cfg.Endpoint, "/") + "/logout")
	if err != nil {
		return "", err
	}
	q := u.Query()
	if postLogoutRedirectURI != "" {
		q.Set("post_logout_redirect_uri", postLogoutRedirectURI)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (c *HTTPClient) Enforce(ctx context.Context, req EnforceRequest) (*EnforceResponse, error) {
	permissionID := req.PermissionID
	if permissionID == "" {
		permissionID = c.cfg.PermissionID
	}
	u, err := url.Parse(strings.TrimRight(c.cfg.Endpoint, "/") + "/api/enforce")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("permissionId", permissionID)
	u.RawQuery = q.Encode()
	payload, _ := json.Marshal([]string{req.Sub, req.Obj, req.Act})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if token, err := c.clientCredentialsToken(ctx); err != nil {
		return nil, err
	} else if token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	var out struct {
		Status string          `json:"status"`
		Msg    string          `json:"msg"`
		Data   json.RawMessage `json:"data"`
	}
	if err := c.doJSON(httpReq, &out); err != nil {
		return nil, err
	}
	if out.Status != "ok" {
		return nil, fmt.Errorf("casdoor enforce status=%s msg=%s", out.Status, out.Msg)
	}
	allow, err := parseBoolRaw(out.Data)
	if err != nil {
		return nil, fmt.Errorf("casdoor enforce returned non-bool data: %w", err)
	}
	return &EnforceResponse{Allow: allow}, nil
}

func (c *HTTPClient) clientCredentialsToken(ctx context.Context) (string, error) {
	if strings.TrimSpace(c.cfg.ClientID) == "" || strings.TrimSpace(c.cfg.ClientSecret) == "" {
		return "", nil
	}
	c.tokenMu.Lock()
	if c.serviceToken != "" && time.Now().Before(c.tokenExpiry) {
		token := c.serviceToken
		c.tokenMu.Unlock()
		return token, nil
	}
	c.tokenMu.Unlock()

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.cfg.Endpoint, "/")+"/api/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var out struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if err := c.doJSON(req, &out); err != nil {
		return "", err
	}
	if out.Error != "" {
		return "", fmt.Errorf("casdoor client credentials error: %s %s", out.Error, out.Description)
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("casdoor client credentials response has no access_token")
	}
	ttl := time.Duration(out.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	expiry := time.Now().Add(ttl - time.Minute)
	if ttl <= time.Minute {
		expiry = time.Now().Add(ttl)
	}
	c.tokenMu.Lock()
	c.serviceToken = out.AccessToken
	c.tokenExpiry = expiry
	c.tokenMu.Unlock()
	return out.AccessToken, nil
}

func (c *HTTPClient) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.cfg.Endpoint, "/")+"/api/health", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("casdoor health http=%d body=%s", resp.StatusCode, string(body))
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil
	}
	var out struct {
		Status string `json:"status"`
		Msg    string `json:"msg"`
	}
	if err := json.Unmarshal(body, &out); err == nil && out.Status != "" && !strings.EqualFold(out.Status, "ok") {
		return fmt.Errorf("casdoor health status=%s msg=%s", out.Status, out.Msg)
	}
	return nil
}

func (c *HTTPClient) doJSON(req *http.Request, out any) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-req.Context().Done():
				return req.Context().Err()
			case <-time.After(time.Duration(attempt) * 150 * time.Millisecond):
			}
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("casdoor http=%d body=%s", resp.StatusCode, string(body))
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("casdoor http=%d body=%s", resp.StatusCode, string(body))
		}
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("decode casdoor response: %w body=%s", err, string(body))
		}
		return nil
	}
	return lastErr
}

func parseBoolRaw(data json.RawMessage) (bool, error) {
	var value bool
	if err := json.Unmarshal(data, &value); err == nil {
		return value, nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		return strconv.ParseBool(text)
	}
	var values []bool
	if err := json.Unmarshal(data, &values); err == nil && len(values) > 0 {
		return values[0], nil
	}
	return false, fmt.Errorf("unsupported data=%s", string(data))
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
