package authn

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/actionlab-ai/aisphere-auth/internal/config"
	"github.com/actionlab-ai/aisphere-auth/internal/principal"
	"github.com/gin-gonic/gin"
)

type handlerTestService struct{}

func (handlerTestService) LoginURL(context.Context, LoginURLRequest) (*LoginURLResponse, error) {
	return &LoginURLResponse{URL: "http://casdoor.example/authorize"}, nil
}
func (handlerTestService) HandleCallback(context.Context, CallbackRequest) (*CallbackResponse, error) {
	return nil, nil
}
func (handlerTestService) Current(context.Context, string) (*principal.Principal, error) {
	return nil, nil
}
func (handlerTestService) Logout(context.Context, LogoutRequest) error { return nil }
func (handlerTestService) Refresh(context.Context, string) (*principal.Principal, error) {
	return nil, nil
}

func TestLoginDisablesRedirectCaching(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/auth/login", NewHandler(config.Config{}, handlerTestService{}).Login)

	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/auth/login?app=agentkit", nil))

	if got, want := res.Code, http.StatusFound; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := res.Header().Get("Cache-Control"), "no-store"; got != want {
		t.Fatalf("Cache-Control = %q, want %q", got, want)
	}
}

func TestLogoutGlobalReturnsCasdoorLogoutURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/auth/logout", NewHandler(config.Config{Casdoor: config.CasdoorConfig{Endpoint: "http://casdoor.example"}}, handlerTestService{}).Logout)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/logout?global=true&redirect=http://localhost:4200/", nil)
	router.ServeHTTP(res, req)

	if got, want := res.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	var body struct {
		LogoutURL string `json:"logout_url"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got, want := body.LogoutURL, "http://casdoor.example/logout?post_logout_redirect_uri=http%3A%2F%2Flocalhost%3A4200%2F"; got != want {
		t.Fatalf("logout_url = %q, want %q", got, want)
	}
}
