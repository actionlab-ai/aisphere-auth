package casdoor

import "context"

type UserInfo struct {
	ID          string
	Owner       string
	Name        string
	DisplayName string
	Email       string
	Roles       []string
	Groups      []string
}

type Client interface {
	GetLoginURL(state string, redirectURI string, scopes []string) (string, error)
	ExchangeCode(ctx context.Context, code string) (*TokenSet, error)
	GetUserInfo(ctx context.Context, bearer string) (*UserInfo, error)
	GetLogoutURL(postLogoutRedirectURI string) (string, error)
}
