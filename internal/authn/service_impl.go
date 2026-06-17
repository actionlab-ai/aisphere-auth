package authn

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/actionlab-ai/aisphere-auth/internal/casdoor"
	"github.com/actionlab-ai/aisphere-auth/internal/config"
	"github.com/actionlab-ai/aisphere-auth/internal/principal"
	"github.com/actionlab-ai/aisphere-auth/internal/session"
	"github.com/google/uuid"
)

var (
	ErrInvalidState = errors.New("invalid login state")
	ErrNoSession    = errors.New("session not found")
)

type StateStore interface {
	Create(ctx context.Context, st LoginState, ttl time.Duration) error
	Consume(ctx context.Context, state string) (LoginState, error)
}

type LoginState struct {
	State       string
	App         string
	RedirectURL string
	CreatedAt   time.Time
}

type ServiceOptions struct {
	Config       config.Config
	Casdoor      casdoor.Client
	SessionStore session.Store
	StateStore   StateStore
}

type DefaultService struct {
	cfg      config.Config
	casdoor  casdoor.Client
	sessions session.Store
	states   StateStore
}

func NewDefaultService(opts ServiceOptions) *DefaultService {
	return &DefaultService{cfg: opts.Config, casdoor: opts.Casdoor, sessions: opts.SessionStore, states: opts.StateStore}
}

func (s *DefaultService) LoginURL(ctx context.Context, req LoginURLRequest) (*LoginURLResponse, error) {
	state, err := randomState()
	if err != nil {
		return nil, err
	}
	app := strings.TrimSpace(req.App)
	if app == "" {
		app = "portal"
	}
	redirect := normalizeRedirect(req.RedirectAfterLogin, "/")
	if err := s.states.Create(ctx, LoginState{State: state, App: app, RedirectURL: redirect, CreatedAt: time.Now()}, 10*time.Minute); err != nil {
		return nil, err
	}
	url, err := s.casdoor.GetLoginURL(state, s.cfg.Casdoor.RedirectURL, s.cfg.Casdoor.Scopes)
	if err != nil {
		return nil, err
	}
	return &LoginURLResponse{URL: url, State: state}, nil
}

func (s *DefaultService) HandleCallback(ctx context.Context, req CallbackRequest) (*CallbackResponse, error) {
	if len(req.State) == 0 || len(req.State) > 128 {
		return nil, ErrInvalidState
	}
	st, err := s.states.Consume(ctx, req.State)
	if err != nil {
		return nil, ErrInvalidState
	}
	tokens, err := s.casdoor.ExchangeCode(ctx, req.Code)
	if err != nil {
		return nil, err
	}
	userinfo, err := s.casdoor.GetUserInfo(ctx, tokens.AccessToken)
	if err != nil {
		return nil, err
	}
	p := principal.FromCasdoorUser(st.App, s.cfg.Casdoor.Owner, userinfo)
	sessionID := "sess_" + uuid.NewString()
	p.SessionID = sessionID
	p.AuthProvider = "casdoor"
	expiresAt := time.Now().Add(time.Duration(s.cfg.Session.TTLSeconds) * time.Second)
	p.ExpiresAtUnix = expiresAt.Unix()
	sess := &session.Session{
		ID:              sessionID,
		Principal:       p,
		CasdoorTokenSet: tokens,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		ExpiresAt:       expiresAt,
		UserAgent:       userAgent(req.Request),
		ClientIP:        clientIP(req.Request, s.cfg.Server.TrustedProxies),
	}
	if err := s.sessions.Create(ctx, sess, time.Until(expiresAt)); err != nil {
		return nil, err
	}
	return &CallbackResponse{Principal: p, SessionID: sessionID, RedirectURL: st.RedirectURL, ExpiresAtUnix: expiresAt.Unix()}, nil
}

func (s *DefaultService) Current(ctx context.Context, sessionID string) (*principal.Principal, error) {
	sess, err := s.sessions.Get(ctx, sessionID)
	if err != nil {
		return nil, ErrNoSession
	}
	if time.Now().After(sess.ExpiresAt) {
		if err := s.sessions.Delete(ctx, sessionID); err != nil {
			slog.Warn("delete expired session failed", "session_id", sessionID, "error", err)
		}
		return nil, ErrNoSession
	}
	if s.cfg.Session.Sliding {
		ttl := time.Duration(s.cfg.Session.TTLSeconds) * time.Second
		if err := s.sessions.Touch(ctx, sessionID, ttl); err != nil {
			slog.Warn("touch session failed", "session_id", sessionID, "error", err)
		}
	}
	return sess.Principal, nil
}

func (s *DefaultService) Logout(ctx context.Context, req LogoutRequest) error {
	if strings.TrimSpace(req.SessionID) == "" {
		return nil
	}
	return s.sessions.Delete(ctx, req.SessionID)
}

func (s *DefaultService) Refresh(ctx context.Context, sessionID string) (*principal.Principal, error) {
	return s.Current(ctx, sessionID)
}

func randomState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func normalizeRedirect(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "//") || strings.HasPrefix(value, "/\\") || strings.Contains(value, "\\") || containsControl(value) {
		return fallback
	}
	u, err := url.Parse(value)
	if err != nil || u.IsAbs() || u.Host != "" || !strings.HasPrefix(u.Path, "/") {
		return fallback
	}
	return value
}

func containsControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func userAgent(r *http.Request) string {
	if r == nil {
		return ""
	}
	return r.UserAgent()
}

func clientIP(r *http.Request, trustedProxies []string) string {
	if r == nil {
		return ""
	}
	remote := remoteHost(r.RemoteAddr)
	if isTrustedProxy(remote, trustedProxies) {
		if value := firstHeaderIP(r.Header.Get("X-Forwarded-For")); value != "" {
			return value
		}
		if value := strings.TrimSpace(r.Header.Get("X-Real-IP")); value != "" {
			return value
		}
	}
	return remote
}

func remoteHost(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}

func isTrustedProxy(remote string, trustedProxies []string) bool {
	ip := net.ParseIP(strings.TrimSpace(remote))
	if ip == nil {
		return false
	}
	for _, item := range trustedProxies {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.Contains(item, "/") {
			_, cidr, err := net.ParseCIDR(item)
			if err == nil && cidr.Contains(ip) {
				return true
			}
			continue
		}
		if trustedIP := net.ParseIP(item); trustedIP != nil && trustedIP.Equal(ip) {
			return true
		}
	}
	return false
}

func firstHeaderIP(value string) string {
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if net.ParseIP(part) != nil {
			return part
		}
	}
	return ""
}
