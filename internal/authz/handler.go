package authz

import (
	"net/http"

	"github.com/actionlab-ai/aisphere-auth/internal/authn"
	"github.com/actionlab-ai/aisphere-auth/internal/config"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	cfg   config.Config
	authn authn.Service
	authz Service
}

func NewHandler(cfg config.Config, authnSvc authn.Service, authzSvc Service) *Handler {
	return &Handler{cfg: cfg, authn: authnSvc, authz: authzSvc}
}

func (h *Handler) Check(c *gin.Context) {
	var req CheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	if req.Subject == "" {
		if sid, err := c.Cookie(h.cfg.Session.CookieName); err == nil && sid != "" {
			p, _ := h.authn.Current(c.Request.Context(), sid)
			req.Principal = p
		}
	}
	decision, err := h.authz.Check(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"allow": false, "error": "forbidden", "decision": decision, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, decision)
}

func (h *Handler) BatchCheck(c *gin.Context) {
	var body struct {
		Checks []CheckRequest `json:"checks"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	if sid, err := c.Cookie(h.cfg.Session.CookieName); err == nil && sid != "" {
		if p, err := h.authn.Current(c.Request.Context(), sid); err == nil {
			for i := range body.Checks {
				if body.Checks[i].Subject == "" {
					body.Checks[i].Principal = p
				}
			}
		}
	}
	decisions, err := h.authz.BatchCheck(c.Request.Context(), body.Checks)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "decisions": decisions, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"decisions": decisions})
}
