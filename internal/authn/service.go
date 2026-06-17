package authn

import (
	"context"
	"net/http"

	"github.com/actionlab-ai/aisphere-auth/internal/principal"
)

type LoginURLRequest struct {
	App                string
	RedirectAfterLogin string
	Request            *http.Request
}

type LoginURLResponse struct {
	URL   string
	State string
}

type CallbackRequest struct {
	Code    string
	State   string
	Request *http.Request
}

type CallbackResponse struct {
	Principal     *principal.Principal
	SessionID     string
	RedirectURL   string
	AccessToken   string
	ExpiresAtUnix int64
}

type LogoutRequest struct {
	SessionID string
	Global    bool
}

type Service interface {
	LoginURL(ctx context.Context, req LoginURLRequest) (*LoginURLResponse, error)
	HandleCallback(ctx context.Context, req CallbackRequest) (*CallbackResponse, error)
	Current(ctx context.Context, sessionID string) (*principal.Principal, error)
	Logout(ctx context.Context, req LogoutRequest) error
	Refresh(ctx context.Context, sessionID string) (*principal.Principal, error)
}
