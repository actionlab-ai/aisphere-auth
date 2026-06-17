package session

import (
	"time"

	"github.com/actionlab-ai/aisphere-auth/internal/casdoor"
	"github.com/actionlab-ai/aisphere-auth/internal/principal"
)

type Session struct {
	ID              string
	Principal       *principal.Principal
	CasdoorTokenSet *casdoor.TokenSet

	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiresAt time.Time

	UserAgent string
	ClientIP  string
}
