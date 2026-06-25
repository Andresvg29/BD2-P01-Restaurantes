package middleware

import (
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogger is middleware that logs each request with unique ID, method, path, status and duration.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := fmt.Sprintf("%x", time.Now().UnixNano())
		c.Set("request_id", requestID)

		start := time.Now()
		c.Next()
		duration := time.Since(start)

		log.Printf("[%s] %s %s → %d (%s)",
			requestID,
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			duration,
		)
	}
}
