package authn

import (
	"net/http"
	"time"

	"github.com/actionlab-ai/aisphere-auth/internal/config"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	cfg config.Config
	svc Service
}

func NewHandler(cfg config.Config, svc Service) *Handler { return &Handler{cfg: cfg, svc: svc} }

func (h *Handler) Login(c *gin.Context) {
	resp, err := h.svc.LoginURL(c.Request.Context(), LoginURLRequest{App: c.Query("app"), RedirectAfterLogin: c.Query("redirect"), Request: c.Request})
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error":"auth_login_failed","message":err.Error()}); return }
	c.Redirect(http.StatusFound, resp.URL)
}

func (h *Handler) Callback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" { c.JSON(http.StatusBadRequest, gin.H{"error":"missing_code_or_state"}); return }
	resp, err := h.svc.HandleCallback(c.Request.Context(), CallbackRequest{Code: code, State: state, Request: c.Request})
	if err != nil { c.JSON(http.StatusUnauthorized, gin.H{"error":"auth_callback_failed","message":err.Error()}); return }
	h.setSessionCookie(c, resp.SessionID, resp.ExpiresAtUnix)
	redirect := resp.RedirectURL
	if redirect == "" { redirect = "/" }
	c.Redirect(http.StatusFound, redirect)
}

func (h *Handler) Me(c *gin.Context) {
	sessionID, err := c.Cookie(h.cfg.Session.CookieName)
	if err != nil || sessionID == "" { c.JSON(http.StatusUnauthorized, gin.H{"error":"unauthorized"}); return }
	p, err := h.svc.Current(c.Request.Context(), sessionID)
	if err != nil { c.JSON(http.StatusUnauthorized, gin.H{"error":"unauthorized","message":err.Error()}); return }
	c.JSON(http.StatusOK, p)
}

func (h *Handler) Logout(c *gin.Context) {
	sessionID, _ := c.Cookie(h.cfg.Session.CookieName)
	_ = h.svc.Logout(c.Request.Context(), LogoutRequest{SessionID: sessionID, Global: c.Query("global") == "true"})
	h.clearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"status":"ok"})
}

func (h *Handler) Introspect(c *gin.Context) {
	var req struct { SessionID string `json:"sessionId"`; App string `json:"app"` }
	_ = c.ShouldBindJSON(&req)
	if req.SessionID == "" { req.SessionID, _ = c.Cookie(h.cfg.Session.CookieName) }
	p, err := h.svc.Current(c.Request.Context(), req.SessionID)
	if err != nil { c.JSON(http.StatusOK, gin.H{"active":false}); return }
	if req.App != "" { p.App = req.App }
	c.JSON(http.StatusOK, gin.H{"active":true,"principal":p})
}

func (h *Handler) setSessionCookie(c *gin.Context, sessionID string, expiresAtUnix int64) {
	maxAge := int(time.Until(time.Unix(expiresAtUnix, 0)).Seconds())
	if maxAge <= 0 { maxAge = h.cfg.Session.TTLSeconds }
	c.SetCookie(h.cfg.Session.CookieName, sessionID, maxAge, "/", h.cfg.Gateway.CookieDomain, h.cfg.Gateway.CookieSecure, true)
}

func (h *Handler) clearSessionCookie(c *gin.Context) {
	c.SetCookie(h.cfg.Session.CookieName, "", -1, "/", h.cfg.Gateway.CookieDomain, h.cfg.Gateway.CookieSecure, true)
}
