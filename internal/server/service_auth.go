package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
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
	expectedHash := sha256.Sum256([]byte(expected))
	limiter := newTokenBucketLimiter(cfg.Internal)

	return func(c *gin.Context) {
		if !limiter.Allow("ip:" + c.ClientIP()) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate_limited", "message": "too many requests"})
			c.Abort()
			return
		}

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

		if actual != "" && !limiter.Allow("token:"+hashForRateLimit(actual)) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate_limited", "message": "too many requests"})
			c.Abort()
			return
		}

		actualHash := sha256.Sum256([]byte(actual))
		if subtle.ConstantTimeCompare(actualHash[:], expectedHash[:]) != 1 {
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

func hashForRateLimit(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}
