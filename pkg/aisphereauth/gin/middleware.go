package authgin

import (
	"net/http"

	"github.com/actionlab-ai/aisphere-auth/pkg/aisphereauth/client"
	"github.com/gin-gonic/gin"
)

type MiddlewareOptions struct {
	App         string
	CookieName  string
	LoginURL    string
	RedirectURL string
}

func RequireLogin(authClient client.Client, opts MiddlewareOptions) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, _ := c.Cookie(defaultString(opts.CookieName, "aisphere_session"))
		if sessionID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		p, err := authClient.Introspect(c.Request.Context(), sessionID, opts.App)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": err.Error()})
			c.Abort()
			return
		}
		SetPrincipal(c, p)
		c.Next()
	}
}

func RequirePermission(authClient client.Client, object string, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := CurrentPrincipal(c)
		if !ok || p == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		decision, err := authClient.Check(c.Request.Context(), client.CheckRequest{
			Subject: p.CasdoorSubject,
			Object:  object,
			Action:  action,
			App:     p.App,
		})
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": err.Error()})
			c.Abort()
			return
		}
		if decision == nil || !decision.Allow {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func RequirePermissionFunc(authClient client.Client, fn func(c *gin.Context) (object string, action string, err error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		object, action, err := fn(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "permission_mapping_error", "message": err.Error()})
			c.Abort()
			return
		}
		RequirePermission(authClient, object, action)(c)
	}
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
