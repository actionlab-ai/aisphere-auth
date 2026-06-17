package authgin

import (
	"net/http"
	"sync"
	"time"

	"github.com/actionlab-ai/aisphere-auth/pkg/aisphereauth"
	"github.com/actionlab-ai/aisphere-auth/pkg/aisphereauth/client"
	"github.com/gin-gonic/gin"
)

const defaultPrincipalCacheTTL = 5 * time.Second

type ErrorHandler func(c *gin.Context, err error)

type MiddlewareOptions struct {
	App        string
	CookieName string

	// LoginURL and RedirectURL are used when the request has no session cookie.
	// If LoginURL is empty, the middleware returns 401 JSON by default.
	LoginURL    string
	RedirectURL string

	// CacheTTL controls the local session introspection cache. Zero means 5s.
	// Set DisableCache=true to disable local caching completely.
	CacheTTL     time.Duration
	DisableCache bool

	OnUnauthorized ErrorHandler
	OnForbidden    ErrorHandler
	OnError        ErrorHandler
}

func RequireLogin(authClient client.Client, opts MiddlewareOptions) gin.HandlerFunc {
	cache := newPrincipalCache(cacheTTL(opts))
	return func(c *gin.Context) {
		sessionID, _ := c.Cookie(defaultString(opts.CookieName, "aisphere_session"))
		if sessionID == "" {
			handleUnauthorized(c, opts, aisphereauth.ErrUnauthorized)
			return
		}

		cacheKey := sessionID + "|" + opts.App
		if !opts.DisableCache {
			if p, ok := cache.Get(cacheKey); ok {
				SetPrincipal(c, p)
				c.Next()
				return
			}
		}

		p, err := authClient.Introspect(c.Request.Context(), sessionID, opts.App)
		if err != nil {
			handleUnauthorized(c, opts, err)
			return
		}
		if !opts.DisableCache {
			cache.Set(cacheKey, p)
		}
		SetPrincipal(c, p)
		c.Next()
	}
}

func RequirePermission(authClient client.Client, object string, action string, opts ...MiddlewareOptions) gin.HandlerFunc {
	middlewareOptions := MiddlewareOptions{}
	if len(opts) > 0 {
		middlewareOptions = opts[0]
	}
	return func(c *gin.Context) {
		p, ok := CurrentPrincipal(c)
		if !ok || p == nil {
			handleUnauthorized(c, middlewareOptions, aisphereauth.ErrNoPrincipal)
			return
		}

		decision, err := authClient.Check(c.Request.Context(), client.CheckRequest{
			Subject: p.EffectiveSubject(),
			Object:  object,
			Action:  action,
			App:     p.App,
		})
		if err != nil {
			handleForbidden(c, middlewareOptions, err)
			return
		}
		if decision == nil || !decision.Allow {
			handleForbidden(c, middlewareOptions, aisphereauth.ErrPermissionDenied)
			return
		}
		c.Next()
	}
}

func RequirePermissionFunc(authClient client.Client, fn func(c *gin.Context) (object string, action string, err error), opts ...MiddlewareOptions) gin.HandlerFunc {
	middlewareOptions := MiddlewareOptions{}
	if len(opts) > 0 {
		middlewareOptions = opts[0]
	}
	return func(c *gin.Context) {
		object, action, err := fn(c)
		if err != nil {
			handleError(c, middlewareOptions, http.StatusBadRequest, "permission_mapping_error", err)
			return
		}
		RequirePermission(authClient, object, action, middlewareOptions)(c)
	}
}

func handleUnauthorized(c *gin.Context, opts MiddlewareOptions, err error) {
	if opts.OnUnauthorized != nil {
		opts.OnUnauthorized(c, err)
		c.Abort()
		return
	}
	if opts.LoginURL != "" {
		redirectURL := opts.RedirectURL
		if redirectURL == "" {
			redirectURL = c.Request.URL.RequestURI()
		}
		loginURL := opts.LoginURL
		if redirectURL != "" {
			separator := "?"
			if containsQuery(loginURL) {
				separator = "&"
			}
			loginURL = loginURL + separator + "redirect=" + urlQueryEscape(redirectURL)
		}
		c.Redirect(http.StatusFound, loginURL)
		c.Abort()
		return
	}
	c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	c.Abort()
}

func handleForbidden(c *gin.Context, opts MiddlewareOptions, err error) {
	if opts.OnForbidden != nil {
		opts.OnForbidden(c, err)
		c.Abort()
		return
	}
	c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	c.Abort()
}

func handleError(c *gin.Context, opts MiddlewareOptions, status int, code string, err error) {
	if opts.OnError != nil {
		opts.OnError(c, err)
		c.Abort()
		return
	}
	c.JSON(status, gin.H{"error": code})
	c.Abort()
}

func cacheTTL(opts MiddlewareOptions) time.Duration {
	if opts.DisableCache {
		return 0
	}
	if opts.CacheTTL > 0 {
		return opts.CacheTTL
	}
	return defaultPrincipalCacheTTL
}

type principalCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]principalCacheEntry
}

type principalCacheEntry struct {
	principal *aisphereauth.Principal
	expiresAt time.Time
}

func newPrincipalCache(ttl time.Duration) *principalCache {
	return &principalCache{ttl: ttl, entries: map[string]principalCacheEntry{}}
}

func (c *principalCache) Get(key string) (*aisphereauth.Principal, bool) {
	if c == nil || c.ttl <= 0 {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	return clonePrincipal(entry.principal), true
}

func (c *principalCache) Set(key string, p *aisphereauth.Principal) {
	if c == nil || c.ttl <= 0 || p == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = principalCacheEntry{principal: clonePrincipal(p), expiresAt: time.Now().Add(c.ttl)}
}

func clonePrincipal(p *aisphereauth.Principal) *aisphereauth.Principal {
	if p == nil {
		return nil
	}
	out := *p
	out.Roles = append([]string(nil), p.Roles...)
	out.Groups = append([]string(nil), p.Groups...)
	return &out
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func containsQuery(value string) bool {
	for _, ch := range value {
		if ch == '?' {
			return true
		}
	}
	return false
}

func urlQueryEscape(value string) string {
	// Keep this dependency-free for middleware users.
	replacer := sync.OnceValue(func() *strings.Replacer {
		return strings.NewReplacer("%", "%25", " ", "%20", "?", "%3F", "&", "%26", "=", "%3D", "#", "%23")
	})
	return replacer().Replace(value)
}
