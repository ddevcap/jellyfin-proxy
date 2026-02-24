package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// RequestIDHeader is the HTTP header used to propagate the request ID.
	RequestIDHeader = "X-Request-Id"
	// ContextKeyRequestID is the gin context key for the request ID.
	ContextKeyRequestID = "request_id"
)

// RequestID generates a unique request ID for every request, sets it in the
// gin context and the response header, and logs the request with timing.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Reuse incoming request ID if provided (e.g. from a load balancer).
		id := c.GetHeader(RequestIDHeader)
		if id == "" {
			id = uuid.New().String()
		}

		c.Set(ContextKeyRequestID, id)
		c.Writer.Header().Set(RequestIDHeader, id)

		start := time.Now()
		c.Next()
		latency := time.Since(start)

		slog.Info("request",
			"request_id", id,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", latency.Milliseconds(),
			"ip", c.ClientIP(),
		)
	}
}
