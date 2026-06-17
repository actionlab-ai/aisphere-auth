package token

import (
	"context"
	"time"

	"github.com/actionlab-ai/aisphere-auth/internal/principal"
)

type IssueRequest struct {
	Principal *principal.Principal
	TTL       time.Duration
	Audience  []string
	TokenType string
}

type IssueResponse struct {
	AccessToken   string
	TokenID       string
	ExpiresAtUnix int64
}

type Issuer interface {
	Issue(ctx context.Context, req IssueRequest) (*IssueResponse, error)
}

type Verifier interface {
	Verify(ctx context.Context, raw string) (*principal.Principal, error)
}
