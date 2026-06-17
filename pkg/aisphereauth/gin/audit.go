package authgin

import (
	"strings"

	"github.com/actionlab-ai/aisphere-auth/pkg/aisphereauth"
	"github.com/gin-gonic/gin"
)

// NewAuditEvent builds a standard audit event from the Gin request and Principal.
func NewAuditEvent(c *gin.Context, resourceType string, resourceID string, action string, result string) aisphereauth.AuditEvent {
	p, _ := CurrentPrincipal(c)
	event := aisphereauth.NewAuditEventFromPrincipal(p, resourceType, resourceID, action, result)
	if c == nil || c.Request == nil {
		return event
	}
	if event.TraceID == "" {
		event.TraceID = traceID(c)
	}
	event.IP = c.ClientIP()
	event.UserAgent = c.Request.UserAgent()
	event.RequestPath = c.Request.URL.Path
	event.RequestMethod = c.Request.Method
	return event
}

func traceID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if value, ok := c.Get("request_id"); ok {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return strings.TrimSpace(c.GetHeader("X-Request-Id"))
}
