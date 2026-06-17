package authgin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/actionlab-ai/aisphere-auth/pkg/aisphereauth"
	"github.com/actionlab-ai/aisphere-auth/pkg/aisphereauth/client"
	"github.com/gin-gonic/gin"
)

type fakeClient struct {
	introspectCalls atomic.Int64
}

func (f *fakeClient) LoginURL(app string, redirect string) string { return "/auth/login" }
func (f *fakeClient) LogoutURL(global bool) string                { return "/auth/logout" }

func (f *fakeClient) Introspect(ctx context.Context, sessionID string, app string) (*aisphereauth.Principal, error) {
	f.introspectCalls.Add(1)
	return &aisphereauth.Principal{SubjectID: "human:test", CasdoorSubject: "aisphere/test", Username: "test", Organization: "aisphere", App: app, AuthProvider: "casdoor"}, nil
}

func (f *fakeClient) Check(ctx context.Context, req client.CheckRequest) (*client.Decision, error) {
	return &client.Decision{Allow: true, Subject: req.Subject, Object: req.Object, Action: req.Action}, nil
}

func (f *fakeClient) BatchCheck(ctx context.Context, reqs []client.CheckRequest) ([]client.Decision, error) {
	return nil, nil
}

func (f *fakeClient) WriteAudit(ctx context.Context, event aisphereauth.AuditEvent) (*aisphereauth.AuditEvent, error) {
	return &event, nil
}

func (f *fakeClient) ListAudit(ctx context.Context, req aisphereauth.AuditListRequest) (*aisphereauth.AuditListResponse, error) {
	return &aisphereauth.AuditListResponse{}, nil
}

func TestRequireLoginCachesIntrospection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authClient := &fakeClient{}
	r := gin.New()
	r.Use(RequireLogin(authClient, MiddlewareOptions{App: "skillhub", CacheTTL: time.Minute}))
	r.GET("/ok", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/ok", nil)
		req.AddCookie(&http.Cookie{Name: "aisphere_session", Value: "sess_test"})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected %d, got %d", http.StatusNoContent, w.Code)
		}
	}
	if got := authClient.introspectCalls.Load(); got != 1 {
		t.Fatalf("expected one introspect call due cache, got %d", got)
	}
}

func TestRequireLoginCustomUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequireLogin(&fakeClient{}, MiddlewareOptions{
		OnUnauthorized: func(c *gin.Context, err error) {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "custom"})
		},
	}))
	r.GET("/ok", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, w.Code)
	}
}
