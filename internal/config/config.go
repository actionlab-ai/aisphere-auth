package config

import (
	"os"
	"strconv"
	"strings"
)

// Config is the root runtime configuration for aisphere-auth.
type Config struct {
	Server  ServerConfig
	Gateway GatewayConfig
	Casdoor CasdoorConfig
	Session SessionConfig
	Authz   AuthzConfig
	Token   TokenConfig
}

type ServerConfig struct {
	Addr          string
	Mode          string
	PublicBaseURL string
}

type GatewayConfig struct {
	CookieDomain   string
	CookieSecure   bool
	CookieSameSite string
}

type CasdoorConfig struct {
	Endpoint      string
	Owner         string
	Application   string
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	Scopes        []string
	PermissionID  string
	SubjectFormat string
}

type SessionConfig struct {
	Provider   string
	CookieName string
	TTLSeconds int
	Sliding    bool
	Redis      RedisConfig
}

type RedisConfig struct {
	Addrs    []string
	Username string
	Password string
	DB       int
	Prefix   string
}

type AuthzConfig struct {
	Provider        string
	CacheEnabled    bool
	CacheTTLSeconds int
	FailClosed      bool
}

type TokenConfig struct {
	Enabled               bool
	Issuer                string
	Audience              []string
	Algorithm             string
	SigningSecret         string
	AccessTokenTTLSeconds int
}

// Load builds config from environment variables. YAML loading will be added in the next milestone.
func Load() Config {
	return Config{
		Server: ServerConfig{
			Addr:          env("AISPHERE_AUTH_ADDR", ":18080"),
			Mode:          env("AISPHERE_AUTH_MODE", "debug"),
			PublicBaseURL: env("AISPHERE_AUTH_PUBLIC_BASE_URL", "http://127.0.0.1:18080"),
		},
		Gateway: GatewayConfig{
			CookieDomain:   env("AISPHERE_COOKIE_DOMAIN", ""),
			CookieSecure:   envBool("AISPHERE_COOKIE_SECURE", false),
			CookieSameSite: env("AISPHERE_COOKIE_SAMESITE", "Lax"),
		},
		Casdoor: CasdoorConfig{
			Endpoint:      env("AISPHERE_CASDOOR_ENDPOINT", "http://127.0.0.1:8000"),
			Owner:         env("AISPHERE_CASDOOR_OWNER", "skillhub"),
			Application:   env("AISPHERE_CASDOOR_APPLICATION", "aisphere"),
			ClientID:      env("AISPHERE_CASDOOR_CLIENT_ID", ""),
			ClientSecret:  env("AISPHERE_CASDOOR_CLIENT_SECRET", ""),
			RedirectURL:   env("AISPHERE_CASDOOR_REDIRECT_URL", "http://127.0.0.1:18080/auth/callback/casdoor"),
			Scopes:        envList("AISPHERE_CASDOOR_SCOPES", []string{"openid", "profile", "email"}),
			PermissionID:  env("AISPHERE_CASDOOR_PERMISSION_ID", "skillhub/platform_permission"),
			SubjectFormat: env("AISPHERE_CASDOOR_SUBJECT_FORMAT", "owner-name"),
		},
		Session: SessionConfig{
			Provider:   env("AISPHERE_SESSION_PROVIDER", "memory"),
			CookieName: env("AISPHERE_SESSION_COOKIE_NAME", "aisphere_session"),
			TTLSeconds: envInt("AISPHERE_SESSION_TTL_SECONDS", 28800),
			Sliding:    envBool("AISPHERE_SESSION_SLIDING", true),
			Redis: RedisConfig{
				Addrs:    envList("AISPHERE_REDIS_ADDRS", []string{"127.0.0.1:6379"}),
				Username: env("AISPHERE_REDIS_USERNAME", ""),
				Password: env("AISPHERE_REDIS_PASSWORD", ""),
				DB:       envInt("AISPHERE_REDIS_DB", 0),
				Prefix:   env("AISPHERE_REDIS_PREFIX", "aisphere"),
			},
		},
		Authz: AuthzConfig{
			Provider:        env("AISPHERE_AUTHZ_PROVIDER", "casdoor"),
			CacheEnabled:    envBool("AISPHERE_AUTHZ_CACHE_ENABLED", true),
			CacheTTLSeconds: envInt("AISPHERE_AUTHZ_CACHE_TTL_SECONDS", 30),
			FailClosed:      envBool("AISPHERE_AUTHZ_FAIL_CLOSED", true),
		},
		Token: TokenConfig{
			Enabled:               envBool("AISPHERE_TOKEN_ENABLED", true),
			Issuer:                env("AISPHERE_TOKEN_ISSUER", "aisphere-auth"),
			Audience:              envList("AISPHERE_TOKEN_AUDIENCE", []string{"aisphere"}),
			Algorithm:             env("AISPHERE_TOKEN_ALG", "HS256"),
			SigningSecret:         env("AISPHERE_JWT_SECRET", ""),
			AccessTokenTTLSeconds: envInt("AISPHERE_ACCESS_TOKEN_TTL_SECONDS", 3600),
		},
	}
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envList(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}
