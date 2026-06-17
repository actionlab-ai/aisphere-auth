package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/spf13/viper"
)

// Config is the root runtime configuration for aisphere-auth.
type Config struct {
	Server   ServerConfig
	Gateway  GatewayConfig
	Casdoor  CasdoorConfig
	Session  SessionConfig
	Authz    AuthzConfig
	Token    TokenConfig
	Internal InternalConfig
}

type ServerConfig struct {
	Addr           string
	Mode           string
	PublicBaseURL  string
	TrustedProxies []string
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

// InternalConfig protects service-to-service endpoints used by SkillHub,
// AgentRuntime, SQLHub and other trusted AI Sphere components.
type InternalConfig struct {
	ServiceTokenRequired bool
	ServiceTokenHeader   string
	ServiceToken         string
	RateLimitEnabled     bool
	RateLimitQPS         int
	RateLimitBurst       int
}

// NewViper creates a configured Viper instance with defaults and environment bindings.
func NewViper(configFile string) *viper.Viper {
	v := viper.New()
	v.SetConfigType("yaml")
	if strings.TrimSpace(configFile) != "" {
		v.SetConfigFile(configFile)
	} else {
		v.SetConfigName("config")
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")
	}

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)
	bindLegacyEnvs(v)
	return v
}

// ReadConfig reads the config file if it exists. Missing default config is not fatal.
func ReadConfig(v *viper.Viper) (string, error) {
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			return "", nil
		}
		return "", fmt.Errorf("读取配置文件失败: %w", err)
	}
	return v.ConfigFileUsed(), nil
}

// Load builds Config from a Viper instance. Precedence is: flags > env > config file > defaults.
func Load(v *viper.Viper) (Config, error) {
	cfg := Config{
		Server: ServerConfig{
			Addr:           v.GetString("server.addr"),
			Mode:           v.GetString("server.mode"),
			PublicBaseURL:  v.GetString("server.publicBaseURL"),
			TrustedProxies: getStringSlice(v, "server.trustedProxies"),
		},
		Gateway: GatewayConfig{
			CookieDomain:   v.GetString("gateway.cookieDomain"),
			CookieSecure:   v.GetBool("gateway.cookieSecure"),
			CookieSameSite: v.GetString("gateway.cookieSameSite"),
		},
		Casdoor: CasdoorConfig{
			Endpoint:      v.GetString("casdoor.endpoint"),
			Owner:         v.GetString("casdoor.owner"),
			Application:   v.GetString("casdoor.application"),
			ClientID:      v.GetString("casdoor.clientId"),
			ClientSecret:  v.GetString("casdoor.clientSecret"),
			RedirectURL:   v.GetString("casdoor.redirectURL"),
			Scopes:        getStringSlice(v, "casdoor.scopes"),
			PermissionID:  v.GetString("casdoor.permissionId"),
			SubjectFormat: v.GetString("casdoor.subjectFormat"),
		},
		Session: SessionConfig{
			Provider:   v.GetString("session.provider"),
			CookieName: v.GetString("session.cookieName"),
			TTLSeconds: v.GetInt("session.ttlSeconds"),
			Sliding:    v.GetBool("session.sliding"),
			Redis: RedisConfig{
				Addrs:    getStringSlice(v, "session.redis.addrs"),
				Username: v.GetString("session.redis.username"),
				Password: v.GetString("session.redis.password"),
				DB:       v.GetInt("session.redis.db"),
				Prefix:   v.GetString("session.redis.prefix"),
			},
		},
		Authz: AuthzConfig{
			Provider:        v.GetString("authz.provider"),
			CacheEnabled:    v.GetBool("authz.cacheEnabled"),
			CacheTTLSeconds: v.GetInt("authz.cacheTTLSeconds"),
			FailClosed:      v.GetBool("authz.failClosed"),
		},
		Token: TokenConfig{
			Enabled:               v.GetBool("token.enabled"),
			Issuer:                v.GetString("token.issuer"),
			Audience:              getStringSlice(v, "token.audience"),
			Algorithm:             v.GetString("token.alg"),
			SigningSecret:         v.GetString("token.signingSecret"),
			AccessTokenTTLSeconds: v.GetInt("token.accessTokenTTLSeconds"),
		},
		Internal: InternalConfig{
			ServiceTokenRequired: v.GetBool("internal.serviceTokenRequired"),
			ServiceTokenHeader:   v.GetString("internal.serviceTokenHeader"),
			ServiceToken:         v.GetString("internal.serviceToken"),
			RateLimitEnabled:     v.GetBool("internal.rateLimitEnabled"),
			RateLimitQPS:         v.GetInt("internal.rateLimitQPS"),
			RateLimitBurst:       v.GetInt("internal.rateLimitBurst"),
		},
	}

	normalize(&cfg)
	return cfg, validate(cfg)
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.addr", ":18080")
	v.SetDefault("server.mode", "debug")
	v.SetDefault("server.publicBaseURL", "http://127.0.0.1:18080")
	v.SetDefault("server.trustedProxies", []string{})

	v.SetDefault("gateway.cookieDomain", "")
	v.SetDefault("gateway.cookieSecure", false)
	v.SetDefault("gateway.cookieSameSite", "Lax")

	v.SetDefault("casdoor.endpoint", "http://127.0.0.1:8000")
	v.SetDefault("casdoor.owner", "skillhub")
	v.SetDefault("casdoor.application", "aisphere")
	v.SetDefault("casdoor.clientId", "")
	v.SetDefault("casdoor.clientSecret", "")
	v.SetDefault("casdoor.redirectURL", "http://127.0.0.1:18080/auth/callback/casdoor")
	v.SetDefault("casdoor.scopes", []string{"openid", "profile", "email"})
	v.SetDefault("casdoor.permissionId", "skillhub/platform_permission")
	v.SetDefault("casdoor.subjectFormat", "owner-name")

	v.SetDefault("session.provider", "memory")
	v.SetDefault("session.cookieName", "aisphere_session")
	v.SetDefault("session.ttlSeconds", 28800)
	v.SetDefault("session.sliding", true)
	v.SetDefault("session.redis.addrs", []string{"127.0.0.1:6379"})
	v.SetDefault("session.redis.username", "")
	v.SetDefault("session.redis.password", "")
	v.SetDefault("session.redis.db", 0)
	v.SetDefault("session.redis.prefix", "aisphere")

	v.SetDefault("authz.provider", "casdoor")
	v.SetDefault("authz.cacheEnabled", true)
	v.SetDefault("authz.cacheTTLSeconds", 30)
	v.SetDefault("authz.failClosed", true)

	// 本地 Token/JWT 能力当前仍是预留能力，默认关闭，避免空密钥误用。
	v.SetDefault("token.enabled", false)
	v.SetDefault("token.issuer", "aisphere-auth")
	v.SetDefault("token.audience", []string{"aisphere"})
	v.SetDefault("token.alg", "HS256")
	v.SetDefault("token.signingSecret", "")
	v.SetDefault("token.accessTokenTTLSeconds", 3600)

	v.SetDefault("internal.serviceTokenRequired", false)
	v.SetDefault("internal.serviceTokenHeader", "X-Aisphere-Service-Token")
	v.SetDefault("internal.serviceToken", "")
	v.SetDefault("internal.rateLimitEnabled", true)
	v.SetDefault("internal.rateLimitQPS", 100)
	v.SetDefault("internal.rateLimitBurst", 200)
}

func bindLegacyEnvs(v *viper.Viper) {
	_ = v.BindEnv("server.addr", "AISPHERE_AUTH_ADDR")
	_ = v.BindEnv("server.mode", "AISPHERE_AUTH_MODE")
	_ = v.BindEnv("server.publicBaseURL", "AISPHERE_AUTH_PUBLIC_BASE_URL")
	_ = v.BindEnv("server.trustedProxies", "AISPHERE_TRUSTED_PROXIES")
	_ = v.BindEnv("gateway.cookieDomain", "AISPHERE_COOKIE_DOMAIN")
	_ = v.BindEnv("gateway.cookieSecure", "AISPHERE_COOKIE_SECURE")
	_ = v.BindEnv("gateway.cookieSameSite", "AISPHERE_COOKIE_SAMESITE")
	_ = v.BindEnv("casdoor.endpoint", "AISPHERE_CASDOOR_ENDPOINT")
	_ = v.BindEnv("casdoor.owner", "AISPHERE_CASDOOR_OWNER")
	_ = v.BindEnv("casdoor.application", "AISPHERE_CASDOOR_APPLICATION")
	_ = v.BindEnv("casdoor.clientId", "AISPHERE_CASDOOR_CLIENT_ID")
	_ = v.BindEnv("casdoor.clientSecret", "AISPHERE_CASDOOR_CLIENT_SECRET")
	_ = v.BindEnv("casdoor.redirectURL", "AISPHERE_CASDOOR_REDIRECT_URL")
	_ = v.BindEnv("casdoor.scopes", "AISPHERE_CASDOOR_SCOPES")
	_ = v.BindEnv("casdoor.permissionId", "AISPHERE_CASDOOR_PERMISSION_ID")
	_ = v.BindEnv("casdoor.subjectFormat", "AISPHERE_CASDOOR_SUBJECT_FORMAT")
	_ = v.BindEnv("session.provider", "AISPHERE_SESSION_PROVIDER")
	_ = v.BindEnv("session.cookieName", "AISPHERE_SESSION_COOKIE_NAME")
	_ = v.BindEnv("session.ttlSeconds", "AISPHERE_SESSION_TTL_SECONDS")
	_ = v.BindEnv("session.sliding", "AISPHERE_SESSION_SLIDING")
	_ = v.BindEnv("session.redis.addrs", "AISPHERE_REDIS_ADDRS")
	_ = v.BindEnv("session.redis.username", "AISPHERE_REDIS_USERNAME")
	_ = v.BindEnv("session.redis.password", "AISPHERE_REDIS_PASSWORD")
	_ = v.BindEnv("session.redis.db", "AISPHERE_REDIS_DB")
	_ = v.BindEnv("session.redis.prefix", "AISPHERE_REDIS_PREFIX")
	_ = v.BindEnv("authz.provider", "AISPHERE_AUTHZ_PROVIDER")
	_ = v.BindEnv("authz.cacheEnabled", "AISPHERE_AUTHZ_CACHE_ENABLED")
	_ = v.BindEnv("authz.cacheTTLSeconds", "AISPHERE_AUTHZ_CACHE_TTL_SECONDS")
	_ = v.BindEnv("authz.failClosed", "AISPHERE_AUTHZ_FAIL_CLOSED")
	_ = v.BindEnv("token.enabled", "AISPHERE_TOKEN_ENABLED")
	_ = v.BindEnv("token.issuer", "AISPHERE_TOKEN_ISSUER")
	_ = v.BindEnv("token.audience", "AISPHERE_TOKEN_AUDIENCE")
	_ = v.BindEnv("token.alg", "AISPHERE_TOKEN_ALG")
	_ = v.BindEnv("token.signingSecret", "AISPHERE_JWT_SECRET")
	_ = v.BindEnv("token.accessTokenTTLSeconds", "AISPHERE_ACCESS_TOKEN_TTL_SECONDS")
	_ = v.BindEnv("internal.serviceTokenRequired", "AISPHERE_SERVICE_TOKEN_REQUIRED")
	_ = v.BindEnv("internal.serviceTokenHeader", "AISPHERE_SERVICE_TOKEN_HEADER")
	_ = v.BindEnv("internal.serviceToken", "AISPHERE_SERVICE_TOKEN")
	_ = v.BindEnv("internal.rateLimitEnabled", "AISPHERE_INTERNAL_RATE_LIMIT_ENABLED")
	_ = v.BindEnv("internal.rateLimitQPS", "AISPHERE_INTERNAL_RATE_LIMIT_QPS")
	_ = v.BindEnv("internal.rateLimitBurst", "AISPHERE_INTERNAL_RATE_LIMIT_BURST")
}

func getStringSlice(v *viper.Viper, key string) []string {
	values := v.GetStringSlice(key)
	if len(values) == 1 && strings.Contains(values[0], ",") {
		values = strings.Split(values[0], ",")
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func normalize(cfg *Config) {
	cfg.Server.Mode = strings.TrimSpace(cfg.Server.Mode)
	cfg.Server.PublicBaseURL = strings.TrimSpace(cfg.Server.PublicBaseURL)
	cfg.Gateway.CookieSameSite = normalizeCookieSameSite(cfg.Gateway.CookieSameSite)
	cfg.Session.Provider = strings.ToLower(strings.TrimSpace(cfg.Session.Provider))
	cfg.Session.CookieName = strings.TrimSpace(cfg.Session.CookieName)
	cfg.Authz.Provider = strings.ToLower(strings.TrimSpace(cfg.Authz.Provider))
	cfg.Token.Algorithm = strings.ToUpper(strings.TrimSpace(cfg.Token.Algorithm))
	cfg.Internal.ServiceTokenHeader = strings.TrimSpace(cfg.Internal.ServiceTokenHeader)
	if cfg.Internal.ServiceTokenHeader == "" {
		cfg.Internal.ServiceTokenHeader = "X-Aisphere-Service-Token"
	}
}

func normalizeCookieSameSite(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return "Strict"
	case "none":
		return "None"
	case "default":
		return "Default"
	case "", "lax":
		return "Lax"
	default:
		return "Lax"
	}
}

func validate(cfg Config) error {
	if strings.TrimSpace(cfg.Server.Addr) == "" {
		return fmt.Errorf("server.addr 不能为空")
	}
	if cfg.Server.PublicBaseURL != "" {
		u, err := url.Parse(cfg.Server.PublicBaseURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("server.publicBaseURL 必须是带 scheme/host 的合法 URL")
		}
	}
	for _, proxy := range cfg.Server.TrustedProxies {
		if err := validateProxyCIDR(proxy); err != nil {
			return fmt.Errorf("server.trustedProxies 包含非法 CIDR/IP %q: %w", proxy, err)
		}
	}
	if strings.TrimSpace(cfg.Casdoor.Endpoint) == "" {
		return fmt.Errorf("casdoor.endpoint 不能为空")
	}
	if u, err := url.Parse(cfg.Casdoor.Endpoint); err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("casdoor.endpoint 必须是带 scheme/host 的合法 URL")
	}
	if strings.TrimSpace(cfg.Casdoor.ClientID) == "" {
		return fmt.Errorf("casdoor.clientId 不能为空")
	}
	if strings.TrimSpace(cfg.Casdoor.ClientSecret) == "" {
		return fmt.Errorf("casdoor.clientSecret 不能为空")
	}
	if strings.TrimSpace(cfg.Session.CookieName) == "" {
		return fmt.Errorf("session.cookieName 不能为空")
	}
	if cfg.Session.TTLSeconds <= 0 {
		return fmt.Errorf("session.ttlSeconds 必须大于 0")
	}
	if cfg.Authz.CacheEnabled && cfg.Authz.CacheTTLSeconds <= 0 {
		return fmt.Errorf("authz.cacheEnabled=true 时 authz.cacheTTLSeconds 必须大于 0")
	}
	if cfg.Authz.CacheTTLSeconds < 0 {
		return fmt.Errorf("authz.cacheTTLSeconds 不能小于 0")
	}
	if cfg.Authz.CacheTTLSeconds > 60 {
		return fmt.Errorf("authz.cacheTTLSeconds 不建议超过 60 秒，当前值: %d", cfg.Authz.CacheTTLSeconds)
	}
	if strings.EqualFold(cfg.Gateway.CookieSameSite, "None") && !cfg.Gateway.CookieSecure {
		return fmt.Errorf("gateway.cookieSameSite=None 时必须同时设置 gateway.cookieSecure=true")
	}
	if cfg.Token.Enabled && strings.TrimSpace(cfg.Token.SigningSecret) == "" {
		return fmt.Errorf("token.enabled=true 时 token.signingSecret/AISPHERE_JWT_SECRET 不能为空")
	}
	if cfg.Internal.ServiceTokenRequired && len(strings.TrimSpace(cfg.Internal.ServiceToken)) < 32 {
		return fmt.Errorf("internal.serviceTokenRequired=true 时 internal.serviceToken 长度至少 32 位")
	}
	if cfg.Internal.RateLimitQPS <= 0 {
		return fmt.Errorf("internal.rateLimitQPS 必须大于 0")
	}
	if cfg.Internal.RateLimitBurst <= 0 {
		return fmt.Errorf("internal.rateLimitBurst 必须大于 0")
	}
	return nil
}

func validateProxyCIDR(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if strings.Contains(value, "/") {
		_, _, err := net.ParseCIDR(value)
		return err
	}
	if net.ParseIP(value) == nil {
		return fmt.Errorf("not an IP or CIDR")
	}
	return nil
}
