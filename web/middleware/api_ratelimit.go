package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type apiHitWindow struct {
	windowStart time.Time
	count       int
}

var apiRLMu sync.Mutex
var apiRLState = map[string]*apiHitWindow{}

const apiMaxPerMinute = 120

// APIRateLimit limits anonymous /api requests per IP per rolling minute.
func APIRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()
		apiRLMu.Lock()
		w, ok := apiRLState[ip]
		if !ok || now.Sub(w.windowStart) >= time.Minute {
			w = &apiHitWindow{windowStart: now, count: 0}
			apiRLState[ip] = w
		}
		w.count++
		cnt := w.count
		apiRLMu.Unlock()
		if cnt > apiMaxPerMinute {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
