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
		if !required {
			if !allowRate(limiter, "ip:"+c.ClientIP()) {
				respondRateLimited(c)
				return
			}
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

		// Required-token mode uses a credential-derived limiter key. Valid and invalid
		// tokens are rate-limited independently, and empty credentials still hit a
		// missing-token bucket keyed by client IP instead of bypassing rate limiting.
		if !allowRate(limiter, serviceCredentialRateKey(c.ClientIP(), actual)) {
			respondRateLimited(c)
			return
		}

		if actual == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "missing service credential"})
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

func allowRate(limiter *tokenBucketLimiter, key string) bool {
	return limiter == nil || limiter.Allow(key)
}

func respondRateLimited(c *gin.Context) {
	c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate_limited", "message": "too many requests"})
	c.Abort()
}

func serviceCredentialRateKey(clientIP string, token string) string {
	if strings.TrimSpace(token) == "" {
		return "missing-token:ip:" + clientIP
	}
	return "token:" + hashForRateLimit(token)
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
