package server

import (
	"net/http"

	"github.com/actionlab-ai/aisphere-auth/internal/config"
	"github.com/gin-gonic/gin"
)

type Server struct {
	cfg    config.Config
	router *gin.Engine
}

func New(cfg config.Config) *Server {
	gin.SetMode(cfg.Server.Mode)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestID())

	s := &Server{cfg: cfg, router: r}
	s.registerRoutes()
	return s
}

func (s *Server) Run() error {
	return s.router.Run(s.cfg.Server.Addr)
}

func (s *Server) registerRoutes() {
	s.router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	s.router.GET("/readyz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"checks": gin.H{
				"config":  "ok",
				"redis":   "not_configured",
				"casdoor": "not_checked",
			},
		})
	})

	auth := s.router.Group("/auth")
	{
		auth.GET("/login", notImplemented("auth.login"))
		auth.GET("/callback/casdoor", notImplemented("auth.callback.casdoor"))
		auth.GET("/me", notImplemented("auth.me"))
		auth.POST("/logout", notImplemented("auth.logout"))
		auth.POST("/sessions/introspect", notImplemented("auth.sessions.introspect"))
	}

	authz := s.router.Group("/authz")
	{
		authz.POST("/check", notImplemented("authz.check"))
		authz.POST("/batch-check", notImplemented("authz.batch_check"))
	}
}

func notImplemented(name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{
			"error":   "not_implemented",
			"handler": name,
		})
	}
}
