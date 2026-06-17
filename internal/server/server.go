package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/actionlab-ai/aisphere-auth/internal/authn"
	"github.com/actionlab-ai/aisphere-auth/internal/authz"
	"github.com/actionlab-ai/aisphere-auth/internal/casdoor"
	"github.com/actionlab-ai/aisphere-auth/internal/config"
	"github.com/actionlab-ai/aisphere-auth/internal/session"
	"github.com/gin-gonic/gin"
)

type closeable interface {
	Close() error
}

type Server struct {
	cfg     config.Config
	router  *gin.Engine
	closers []closeable
}

func New(cfg config.Config) *Server {
	gin.SetMode(cfg.Server.Mode)

	r := gin.New()
	if err := r.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		panic(fmt.Errorf("configure trusted proxies: %w", err))
	}
	r.Use(gin.Recovery())
	r.Use(requestID())

	s := &Server{cfg: cfg, router: r}
	s.registerRoutes()
	return s
}

func (s *Server) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	defer s.closeDependencies()

	httpServer := &http.Server{
		Addr:              s.cfg.Server.Addr,
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("aisphere-auth listening", "addr", s.cfg.Server.Addr)
		err := httpServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		slog.Info("aisphere-auth shutting down")
		return httpServer.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func (s *Server) registerRoutes() {
	casdoorClient := casdoor.NewHTTPClient(s.cfg.Casdoor)
	sessionStore := mustBuildSessionStore(s.cfg)
	stateStore := mustBuildStateStore(s.cfg)
	s.closers = append(s.closers, sessionStore)
	if c, ok := stateStore.(closeable); ok {
		s.closers = append(s.closers, c)
	}

	authnSvc := authn.NewDefaultService(authn.ServiceOptions{Config: s.cfg, Casdoor: casdoorClient, SessionStore: sessionStore, StateStore: stateStore})
	authzSvc := authz.NewDefaultService(s.cfg, casdoorClient)
	authnHandler := authn.NewHandler(s.cfg, authnSvc)
	authzHandler := authz.NewHandler(s.cfg, authnSvc, authzSvc)
	internalAuth := requireServiceToken(s.cfg)

	s.router.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	s.router.GET("/readyz", func(c *gin.Context) {
		status, checks := s.readiness(c.Request.Context(), sessionStore, casdoorClient)
		c.JSON(status, gin.H{"status": statusText(status), "checks": checks})
	})

	auth := s.router.Group("/auth")
	{
		auth.GET("/login", authnHandler.Login)
		auth.GET("/callback/casdoor", authnHandler.Callback)
		auth.GET("/me", authnHandler.Me)
		auth.POST("/logout", authnHandler.Logout)
		auth.POST("/sessions/introspect", internalAuth, authnHandler.Introspect)
	}

	authzGroup := s.router.Group("/authz", internalAuth)
	{
		authzGroup.POST("/check", authzHandler.Check)
		authzGroup.POST("/batch-check", authzHandler.BatchCheck)
	}
}

func (s *Server) readiness(ctx context.Context, sessionStore session.Store, casdoorClient *casdoor.HTTPClient) (int, gin.H) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	checks := gin.H{
		"config":        "ok",
		"session":       s.cfg.Session.Provider,
		"state":         s.cfg.Session.Provider,
		"casdoor":       "unknown",
		"internal_auth": s.internalAuthStatus(),
	}
	ok := true
	if err := sessionStore.Ping(ctx); err != nil {
		checks["session"] = "unavailable"
		checks["session_error"] = err.Error()
		ok = false
	}
	if err := casdoorClient.Ping(ctx); err != nil {
		checks["casdoor"] = "unavailable"
		checks["casdoor_error"] = err.Error()
		ok = false
	} else {
		checks["casdoor"] = "ok"
	}
	if !ok {
		return http.StatusServiceUnavailable, checks
	}
	return http.StatusOK, checks
}

func statusText(status int) string {
	if status >= 200 && status < 300 {
		return "ok"
	}
	return "unavailable"
}

func (s *Server) closeDependencies() {
	for _, c := range s.closers {
		if err := c.Close(); err != nil {
			slog.Warn("close dependency failed", "error", err)
		}
	}
}

func (s *Server) internalAuthStatus() string {
	if s.cfg.Internal.ServiceTokenRequired || strings.TrimSpace(s.cfg.Internal.ServiceToken) != "" {
		return "service-token"
	}
	return "disabled"
}

func mustBuildSessionStore(cfg config.Config) session.Store {
	switch strings.ToLower(strings.TrimSpace(cfg.Session.Provider)) {
	case "", "memory":
		return session.NewMemoryStore()
	case "redis":
		store, err := session.NewRedisStore(cfg.Session.Redis)
		if err != nil {
			panic(fmt.Errorf("initialize redis session store: %w", err))
		}
		return store
	default:
		panic(fmt.Errorf("unsupported session provider: %s", cfg.Session.Provider))
	}
}

func mustBuildStateStore(cfg config.Config) authn.StateStore {
	switch strings.ToLower(strings.TrimSpace(cfg.Session.Provider)) {
	case "", "memory":
		return authn.NewMemoryStateStore()
	case "redis":
		store, err := authn.NewRedisStateStore(cfg.Session.Redis)
		if err != nil {
			panic(fmt.Errorf("initialize redis state store: %w", err))
		}
		return store
	default:
		panic(fmt.Errorf("unsupported state provider: %s", cfg.Session.Provider))
	}
}
