package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/actionlab-ai/aisphere-auth/internal/config"
	"github.com/gin-gonic/gin"
)

func TestRequireServiceTokenDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", requireServiceToken(config.Config{}), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d", http.StatusNoContent, w.Code)
	}
}

func TestRequireServiceTokenRejectsMissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", requireServiceToken(config.Config{Internal: config.InternalConfig{
		ServiceTokenRequired: true,
		ServiceTokenHeader:   "X-Aisphere-Service-Token",
		ServiceToken:         "secret",
	}}), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestRequireServiceTokenAcceptsHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", requireServiceToken(config.Config{Internal: config.InternalConfig{
		ServiceTokenRequired: true,
		ServiceTokenHeader:   "X-Aisphere-Service-Token",
		ServiceToken:         "secret",
	}}), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("X-Aisphere-Service-Token", "secret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d", http.StatusNoContent, w.Code)
	}
}

func TestRequireServiceTokenAcceptsBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", requireServiceToken(config.Config{Internal: config.InternalConfig{
		ServiceTokenRequired: true,
		ServiceTokenHeader:   "X-Aisphere-Service-Token",
		ServiceToken:         "secret",
	}}), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d", http.StatusNoContent, w.Code)
	}
}
