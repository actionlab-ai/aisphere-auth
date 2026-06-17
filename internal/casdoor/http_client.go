package casdoor

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

	"github.com/actionlab-ai/aisphere-auth/internal/config"
)

type HTTPClient struct {
	cfg        config.CasdoorConfig
	httpClient *http.Client
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

	var out struct {
		Status string `json:"status"`
		Msg    string `json:"msg"`
		Data   any    `json:"data"`
	}
	if err := c.doJSON(httpReq, &out); err != nil {
		return nil, err
	}
	if out.Status != "ok" {
		return nil, fmt.Errorf("casdoor enforce status=%s msg=%s", out.Status, out.Msg)
	}
	allow, ok := out.Data.(bool)
	if !ok {
		return nil, fmt.Errorf("casdoor enforce returned non-bool data")
	}
	return &EnforceResponse{Allow: allow}, nil
}

func (c *HTTPClient) doJSON(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("casdoor http=%d body=%s", resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode casdoor response: %w body=%s", err, string(body))
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
