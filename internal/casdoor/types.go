package casdoor

import "time"

type TokenSet struct {
	AccessToken  string
	IDToken      string
	RefreshToken string
	TokenType    string
	ExpiresAt    time.Time
}
