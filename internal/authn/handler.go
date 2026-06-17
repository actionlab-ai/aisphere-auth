package authn

import (
	"net/http"
	"strings"
	"time"

	"github.com/actionlab-ai/aisphere-auth/internal/config"
	"github.com/actionlab-ai/aisphere-auth/internal/httpx"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	cfg config.Config
	svc Service
}

func NewHandler(cfg config.Config, svc Service) *Handler { return &Handler{cfg: cfg, svc: svc} }

func (h *Handler) Login(c *gin.Context) {
	resp, err := h.svc.LoginURL(c.Request.Context(), LoginURLRequest{App: c.Query("app"), RedirectAfterLogin: c.Query("redirect"), Request: c.Request})
	if err != nil {
		httpx.RespondError(c, http.StatusInternalServerError, "auth_login_failed", err.Error())
		return
	}
	c.Redirect(http.StatusFound, resp.URL)
}

func (h *Handler) Callback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		httpx.RespondError(c, http.StatusBadRequest, "missing_code_or_state", "缺少 code 或 state")
		return
	}
	resp, err := h.svc.HandleCallback(c.Request.Context(), CallbackRequest{Code: code, State: state, Request: c.Request})
	if err != nil {
		httpx.RespondError(c, http.StatusUnauthorized, "auth_callback_failed", err.Error())
		return
	}
	h.setSessionCookie(c, resp.SessionID, resp.ExpiresAtUnix)
	redirect := resp.RedirectURL
	if redirect == "" {
		redirect = "/"
	}
	c.Redirect(http.StatusFound, redirect)
}

func (h *Handler) Me(c *gin.Context) {
	sessionID, err := c.Cookie(h.cfg.Session.CookieName)
	if err != nil || sessionID == "" {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized", "未登录或会话不存在")
		return
	}
	p, err := h.svc.Current(c.Request.Context(), sessionID)
	if err != nil {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized", err.Error())
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *Handler) Logout(c *gin.Context) {
	sessionID, _ := c.Cookie(h.cfg.Session.CookieName)
	if err := h.svc.Logout(c.Request.Context(), LogoutRequest{SessionID: sessionID, Global: c.Query("global") == "true"}); err != nil {
		httpx.RespondError(c, http.StatusInternalServerError, "auth_logout_failed", err.Error())
		return
	}
	h.clearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) Introspect(c *gin.Context) {
	var req struct {
		SessionID string `json:"sessionId"`
		App       string `json:"app"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.SessionID == "" {
		req.SessionID, _ = c.Cookie(h.cfg.Session.CookieName)
	}
	p, err := h.svc.Current(c.Request.Context(), req.SessionID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"active": false, "inactive_reason": err.Error(), "traceId": httpx.RequestID(c)})
		return
	}
	if req.App != "" {
		p.App = req.App
	}
	c.JSON(http.StatusOK, gin.H{"active": true, "principal": p})
}

func (h *Handler) setSessionCookie(c *gin.Context, sessionID string, expiresAtUnix int64) {
	maxAge := int(time.Until(time.Unix(expiresAtUnix, 0)).Seconds())
	if maxAge <= 0 {
		maxAge = h.cfg.Session.TTLSeconds
	}
	expires := time.Now().Add(time.Duration(maxAge) * time.Second)
	h.writeSessionCookie(c, sessionID, maxAge, expires)
}

func (h *Handler) clearSessionCookie(c *gin.Context) {
	h.writeSessionCookie(c, "", -1, time.Unix(0, 0))
}

func (h *Handler) writeSessionCookie(c *gin.Context, value string, maxAge int, expires time.Time) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.cfg.Session.CookieName,
		Value:    value,
		Path:     "/",
		Domain:   h.cfg.Gateway.CookieDomain,
		MaxAge:   maxAge,
		Expires:  expires,
		Secure:   h.cfg.Gateway.CookieSecure,
		HttpOnly: true,
		SameSite: sameSiteMode(h.cfg.Gateway.CookieSameSite),
	})
}

func sameSiteMode(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "default":
		return http.SameSiteDefaultMode
	case "", "lax":
		return http.SameSiteLaxMode
	default:
		return http.SameSiteLaxMode
	}
}
