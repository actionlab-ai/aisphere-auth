package casdoor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/actionlab-ai/aisphere-auth/internal/config"
)

func TestEnforceUsesClientCredentialsBearerToken(t *testing.T) {
	var sawTokenRequest bool
	var sawBearer bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/login/oauth/access_token":
			sawTokenRequest = true
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}
			if r.Form.Get("grant_type") != "client_credentials" {
				t.Fatalf("expected client_credentials grant, got %q", r.Form.Get("grant_type"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "service-token",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
		case "/api/enforce":
			if r.Header.Get("Authorization") == "Bearer service-token" {
				sawBearer = true
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "ok",
				"data":   []bool{true},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewHTTPClient(config.CasdoorConfig{
		Endpoint:     srv.URL,
		ClientID:     "client",
		ClientSecret: "secret",
		PermissionID: "aisphere/perm_aihub_admin",
	})
	resp, err := client.Enforce(context.Background(), EnforceRequest{Sub: "aisphere/test1", Obj: "aihub:admin:*", Act: "read"})
	if err != nil {
		t.Fatalf("enforce failed: %v", err)
	}
	if resp == nil || !resp.Allow {
		t.Fatalf("expected allow response, got %#v", resp)
	}
	if !sawTokenRequest {
		t.Fatal("expected client credentials token request")
	}
	if !sawBearer {
		t.Fatal("expected enforce request to carry bearer token")
	}
}
