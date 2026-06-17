package server

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/actionlab-ai/aisphere-auth/internal/config"
	"github.com/gin-gonic/gin"
)

func requireServiceToken(cfg config.Config) gin.HandlerFunc {
	headerName := strings.TrimSpace(cfg.Internal.ServiceTokenHeader)
	if headerName == "" {
		headerName = "X-Aisphere-Service-Token"
	}

	required := cfg.Internal.ServiceTokenRequired || strings.TrimSpace(cfg.Internal.ServiceToken) != ""
	expected := strings.TrimSpace(cfg.Internal.ServiceToken)

	return func(c *gin.Context) {
		if !required {
			c.Next()
			return
		}

		if expected == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "config_error",
				"message": "service token is required but AISPHERE_SERVICE_TOKEN is empty",
			})
			c.Abort()
			return
		}

		actual := strings.TrimSpace(c.GetHeader(headerName))
		if actual == "" {
			actual = bearerToken(c.GetHeader("Authorization"))
		}

		if subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) != 1 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "invalid service credential"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func bearerToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.SplitN(value, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
