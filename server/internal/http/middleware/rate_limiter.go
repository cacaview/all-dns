package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter implements a per-key sliding-window rate limiter.
type RateLimiter struct {
	requests map[string]*keyState
	mu       sync.RWMutex
	rate     int           // max requests per window
	window   time.Duration // window duration
}

type keyState struct {
	tokens    int
	lastCheck time.Time
}

func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string]*keyState),
		rate:     rate,
		window:   window,
	}
	go rl.cleanup()
	return rl
}

// Allow checks whether a request under the given key should be allowed.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	state, ok := rl.requests[key]
	if !ok {
		rl.requests[key] = &keyState{tokens: rl.rate - 1, lastCheck: now}
		return true
	}
	elapsed := now.Sub(state.lastCheck)
	// Refill tokens based on elapsed time
	tokensToAdd := int(elapsed.Seconds() * float64(rl.rate) / rl.window.Seconds())
	if tokensToAdd > 0 {
		state.tokens += tokensToAdd
		if state.tokens > rl.rate {
			state.tokens = rl.rate
		}
		state.lastCheck = now
	}
	if state.tokens <= 0 {
		return false
	}
	state.tokens--
	return true
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-2 * rl.window)
		for key, state := range rl.requests {
			if state.lastCheck.Before(cutoff) {
				delete(rl.requests, key)
			}
		}
		rl.mu.Unlock()
	}
}

// Limiter returns a Gin middleware that enforces rate limits.
// writeOnly makes the limiter apply to POST/PUT/DELETE only.
func (rl *RateLimiter) Limiter(writeOnly bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if writeOnly && (c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS") {
			c.Next()
			return
		}
		key := c.ClientIP()
		if user, ok := CurrentUser(c); ok {
			key = fmt.Sprintf("user:%d", user.ID)
		}
		if !rl.Allow(key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
