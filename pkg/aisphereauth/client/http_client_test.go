package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPClientSendsServiceToken(t *testing.T) {
	var gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Aisphere-Service-Token")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"active": true,
			"principal": map[string]any{
				"subjectId":      "human:test",
				"casdoorSubject": "skillhub/test",
				"username":       "test",
				"organization":   "skillhub",
				"authProvider":   "casdoor",
			},
		})
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, WithServiceToken("secret"))
	_, err := c.Introspect(context.Background(), "sess_test", "skillhub")
	if err != nil {
		t.Fatalf("introspect failed: %v", err)
	}
	if gotToken != "secret" {
		t.Fatalf("expected service token header, got %q", gotToken)
	}
}

func TestHTTPClientSupportsCustomServiceTokenHeader(t *testing.T) {
	var gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Custom-Service-Token")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Decision{Allow: true, Source: "test"})
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, WithServiceToken("secret"), WithServiceTokenHeader("X-Custom-Service-Token"))
	_, err := c.Check(context.Background(), CheckRequest{Subject: "skillhub/test", Object: "skillhub:skill:*", Action: "read"})
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}
	if gotToken != "secret" {
		t.Fatalf("expected service token header, got %q", gotToken)
	}
}
