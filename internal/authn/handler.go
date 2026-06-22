package authn

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/actionlab-ai/aisphere-auth/internal/config"
	"github.com/actionlab-ai/aisphere-auth/internal/httpx"
	"github.com/gin-gonic/gin"
)

const maxOAuthStateLength = 128

type Handler struct {
	cfg config.Config
	svc Service
}

func NewHandler(cfg config.Config, svc Service) *Handler { return &Handler{cfg: cfg, svc: svc} }

func (h *Handler) Login(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	resp, err := h.svc.LoginURL(c.Request.Context(), LoginURLRequest{App: c.Query("app"), RedirectAfterLogin: c.Query("redirect"), Request: c.Request})
	if err != nil {
		slog.Error("auth login failed", "trace_id", httpx.RequestID(c), "error", err)
		httpx.RespondError(c, http.StatusInternalServerError, "auth_login_failed", "登录初始化失败")
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
	if len(state) > maxOAuthStateLength {
		httpx.RespondError(c, http.StatusBadRequest, "invalid_state", "state 参数非法")
		return
	}
	resp, err := h.svc.HandleCallback(c.Request.Context(), CallbackRequest{Code: code, State: state, Request: c.Request})
	if err != nil {
		slog.Warn("auth callback failed", "trace_id", httpx.RequestID(c), "error", err)
		httpx.RespondError(c, http.StatusUnauthorized, "auth_callback_failed", "登录回调校验失败")
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
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized", "未登录或会话已过期")
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *Handler) Logout(c *gin.Context) {
	logoutURL, err := h.globalLogoutURL(c.Query("redirect"), c.Query("global") == "true")
	if err != nil {
		httpx.RespondError(c, http.StatusInternalServerError, "auth_logout_failed", "退出登录失败")
		return
	}
	sessionID, _ := c.Cookie(h.cfg.Session.CookieName)
	if err := h.svc.Logout(c.Request.Context(), LogoutRequest{SessionID: sessionID, Global: c.Query("global") == "true"}); err != nil {
		slog.Warn("auth logout failed", "trace_id", httpx.RequestID(c), "error", err)
		httpx.RespondError(c, http.StatusInternalServerError, "auth_logout_failed", "退出登录失败")
		return
	}
	h.clearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"status": "ok", "logout_url": logoutURL})
}

func (h *Handler) globalLogoutURL(redirect string, global bool) (string, error) {
	if !global {
		return "", nil
	}
	u, err := url.Parse(strings.TrimRight(h.cfg.Casdoor.Endpoint, "/") + "/logout")
	if err != nil {
		return "", err
	}
	if redirect = normalizeRedirect(redirect, ""); redirect != "" {
		q := u.Query()
		q.Set("post_logout_redirect_uri", redirect)
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
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
		c.JSON(http.StatusOK, gin.H{"active": false, "inactive_reason": "session_inactive", "traceId": httpx.RequestID(c)})
		return
	}
	if req.App != "" {
		cp := *p
		cp.App = req.App
		p = &cp
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
