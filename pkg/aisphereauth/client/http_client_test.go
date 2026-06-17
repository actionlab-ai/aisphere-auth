package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/actionlab-ai/aisphere-auth/pkg/aisphereauth"
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

func TestHTTPClientWriteAndListAudit(t *testing.T) {
	var gotWriteToken string
	var gotListToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/audit/events":
			gotWriteToken = r.Header.Get("X-Aisphere-Service-Token")
			_ = json.NewEncoder(w).Encode(aisphereauth.AuditEvent{ID: "evt_1", ActorSubject: "aisphere/admin", ResourceType: "skill", Action: "skill.create", Result: aisphereauth.AuditResultSuccess})
		case r.Method == http.MethodGet && r.URL.Path == "/audit/events":
			gotListToken = r.Header.Get("X-Aisphere-Service-Token")
			if r.URL.Query().Get("app") != "skillhub" {
				t.Fatalf("expected app query skillhub, got %q", r.URL.Query().Get("app"))
			}
			_ = json.NewEncoder(w).Encode(aisphereauth.AuditListResponse{Items: []aisphereauth.AuditEvent{{ID: "evt_1"}}, Total: 1, Limit: 10})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, WithServiceToken("secret"))
	event, err := c.WriteAudit(context.Background(), aisphereauth.AuditEvent{ActorSubject: "aisphere/admin", ResourceType: "skill", Action: "skill.create", Result: aisphereauth.AuditResultSuccess})
	if err != nil {
		t.Fatalf("write audit failed: %v", err)
	}
	if event.ID != "evt_1" {
		t.Fatalf("expected audit event id evt_1, got %q", event.ID)
	}
	resp, err := c.ListAudit(context.Background(), aisphereauth.AuditListRequest{App: "skillhub", Limit: 10})
	if err != nil {
		t.Fatalf("list audit failed: %v", err)
	}
	if resp.Total != 1 {
		t.Fatalf("expected total 1, got %d", resp.Total)
	}
	if gotWriteToken != "secret" || gotListToken != "secret" {
		t.Fatalf("expected service token on audit calls, got write=%q list=%q", gotWriteToken, gotListToken)
	}
}
