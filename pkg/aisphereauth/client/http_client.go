package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/actionlab-ai/aisphere-auth/pkg/aisphereauth"
)

const defaultMaxErrorBodyBytes int64 = 4096

// APIError represents a non-2xx response returned by aisphere-auth.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("aisphere-auth http=%d body=%s", e.StatusCode, e.Body)
}

// HookEvent is emitted around SDK HTTP calls so business services can collect
// latency/error metrics without wrapping the client manually.
type HookEvent struct {
	Operation  string
	Method     string
	Path       string
	StatusCode int
	Duration   time.Duration
	Err        error
}

type HookFunc func(ctx context.Context, event HookEvent)

type HTTPClient struct {
	baseURL            string
	serviceToken       string
	serviceTokenHeader string
	maxErrorBodyBytes  int64
	httpClient         *http.Client
	hook               HookFunc
}

type HTTPClientOption func(*HTTPClient)

func WithHTTPClient(httpClient *http.Client) HTTPClientOption {
	return func(c *HTTPClient) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

func WithTimeout(timeout time.Duration) HTTPClientOption {
	return func(c *HTTPClient) {
		if timeout > 0 {
			c.httpClient.Timeout = timeout
		}
	}
}

func WithServiceToken(token string) HTTPClientOption {
	return func(c *HTTPClient) { c.serviceToken = strings.TrimSpace(token) }
}

func WithServiceTokenHeader(header string) HTTPClientOption {
	return func(c *HTTPClient) {
		if strings.TrimSpace(header) != "" {
			c.serviceTokenHeader = strings.TrimSpace(header)
		}
	}
}

func WithMaxErrorBodyBytes(limit int64) HTTPClientOption {
	return func(c *HTTPClient) {
		if limit > 0 {
			c.maxErrorBodyBytes = limit
		}
	}
}

func WithHook(hook HookFunc) HTTPClientOption {
	return func(c *HTTPClient) { c.hook = hook }
}

func NewHTTPClient(baseURL string, opts ...HTTPClientOption) *HTTPClient {
	c := &HTTPClient{
		baseURL:            strings.TrimRight(baseURL, "/"),
		serviceTokenHeader: "X-Aisphere-Service-Token",
		maxErrorBodyBytes:  defaultMaxErrorBodyBytes,
		httpClient:         &http.Client{Timeout: 10 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *HTTPClient) LoginURL(app string, redirect string) string {
	values := url.Values{}
	if strings.TrimSpace(app) != "" {
		values.Set("app", strings.TrimSpace(app))
	}
	if strings.TrimSpace(redirect) != "" {
		values.Set("redirect", strings.TrimSpace(redirect))
	}
	if encoded := values.Encode(); encoded != "" {
		return c.baseURL + "/auth/login?" + encoded
	}
	return c.baseURL + "/auth/login"
}

func (c *HTTPClient) LogoutURL(global bool) string {
	if global {
		return c.baseURL + "/auth/logout?global=true"
	}
	return c.baseURL + "/auth/logout"
}

func (c *HTTPClient) Introspect(ctx context.Context, sessionID string, app string) (*aisphereauth.Principal, error) {
	payload, _ := json.Marshal(map[string]string{"sessionId": sessionID, "app": app})
	var out struct {
		Active         bool                    `json:"active"`
		InactiveReason string                  `json:"inactive_reason,omitempty"`
		Principal      *aisphereauth.Principal `json:"principal"`
		TraceID        string                  `json:"traceId,omitempty"`
	}
	if err := c.post(ctx, "introspect", "/auth/sessions/introspect", payload, &out); err != nil {
		return nil, err
	}
	if !out.Active || out.Principal == nil {
		if out.InactiveReason != "" {
			return nil, fmt.Errorf("%w: %s", aisphereauth.ErrInactiveSession, out.InactiveReason)
		}
		return nil, aisphereauth.ErrInactiveSession
	}
	return out.Principal, nil
}

func (c *HTTPClient) Check(ctx context.Context, req CheckRequest) (*Decision, error) {
	payload, _ := json.Marshal(req)
	var out Decision
	if err := c.post(ctx, "check", "/authz/check", payload, &out); err != nil {
		return nil, err
	}
	if !out.Allow {
		return &out, aisphereauth.ErrPermissionDenied
	}
	return &out, nil
}

func (c *HTTPClient) BatchCheck(ctx context.Context, reqs []CheckRequest) ([]Decision, error) {
	payload, _ := json.Marshal(map[string]any{"checks": reqs})
	var out struct {
		Decisions []Decision `json:"decisions"`
	}
	if err := c.post(ctx, "batch_check", "/authz/batch-check", payload, &out); err != nil {
		return nil, err
	}
	return out.Decisions, nil
}

func (c *HTTPClient) post(ctx context.Context, operation string, path string, payload []byte, out any) error {
	start := time.Now()
	statusCode := 0
	var callErr error
	defer func() {
		if c.hook != nil {
			c.hook(ctx, HookEvent{Operation: operation, Method: http.MethodPost, Path: path, StatusCode: statusCode, Duration: time.Since(start), Err: callErr})
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		callErr = err
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.serviceToken != "" {
		req.Header.Set(c.serviceTokenHeader, c.serviceToken)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		callErr = err
		return err
	}
	defer resp.Body.Close()
	statusCode = resp.StatusCode

	body, err := readLimited(resp.Body, c.maxErrorBodyBytes)
	if err != nil {
		callErr = err
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		callErr = &APIError{StatusCode: resp.StatusCode, Body: body}
		return callErr
	}
	if err := json.Unmarshal([]byte(body), out); err != nil {
		callErr = fmt.Errorf("decode aisphere-auth response: %w", err)
		return callErr
	}
	return nil
}

func readLimited(reader io.Reader, limit int64) (string, error) {
	if limit <= 0 {
		limit = defaultMaxErrorBodyBytes
	}
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return "", err
	}
	if int64(len(body)) > limit {
		return string(body[:limit]) + "...(truncated)", nil
	}
	return string(body), nil
}

func IsInactiveSession(err error) bool { return errors.Is(err, aisphereauth.ErrInactiveSession) }
