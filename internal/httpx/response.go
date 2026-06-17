package httpx

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const HeaderRequestID = "X-Request-Id"

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	TraceID string `json:"traceId,omitempty"`
}

func RespondError(c *gin.Context, status int, code string, message string) {
	if strings.TrimSpace(code) == "" {
		code = http.StatusText(status)
	}
	if strings.TrimSpace(message) == "" {
		message = http.StatusText(status)
	}
	c.JSON(status, ErrorResponse{Error: ErrorBody{Code: code, Message: message, TraceID: RequestID(c)}})
}

func RequestID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if value, ok := c.Get("request_id"); ok {
		if text, ok := value.(string); ok && text != "" {
			return text
		}
	}
	return c.GetHeader(HeaderRequestID)
}
