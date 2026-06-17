package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/actionlab-ai/aisphere-auth/pkg/aisphereauth"
)

type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{baseURL: strings.TrimRight(baseURL, "/"), httpClient: &http.Client{Timeout: 10 * time.Second}}
}

func (c *HTTPClient) Introspect(ctx context.Context, sessionID string, app string) (*aisphereauth.Principal, error) {
	payload, _ := json.Marshal(map[string]string{"sessionId": sessionID, "app": app})
	var out struct { Active bool `json:"active"`; Principal *aisphereauth.Principal `json:"principal"` }
	if err := c.post(ctx, "/auth/sessions/introspect", payload, &out); err != nil { return nil, err }
	if !out.Active || out.Principal == nil { return nil, fmt.Errorf("inactive session") }
	return out.Principal, nil
}

func (c *HTTPClient) Check(ctx context.Context, req CheckRequest) (*Decision, error) {
	payload, _ := json.Marshal(req)
	var out Decision
	if err := c.post(ctx, "/authz/check", payload, &out); err != nil { return nil, err }
	return &out, nil
}

func (c *HTTPClient) BatchCheck(ctx context.Context, reqs []CheckRequest) ([]Decision, error) {
	payload, _ := json.Marshal(map[string]any{"checks": reqs})
	var out struct { Decisions []Decision `json:"decisions"` }
	if err := c.post(ctx, "/authz/batch-check", payload, &out); err != nil { return nil, err }
	return out.Decisions, nil
}

func (c *HTTPClient) post(ctx context.Context, path string, payload []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil { return err }
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil { return err }
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return fmt.Errorf("aisphere-auth http=%d body=%s", resp.StatusCode, string(body)) }
	if err := json.Unmarshal(body, out); err != nil { return fmt.Errorf("decode aisphere-auth response: %w body=%s", err, string(body)) }
	return nil
}
