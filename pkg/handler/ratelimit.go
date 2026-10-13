package handler

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	ips map[string]*ipLimiter
	mu  sync.Mutex
	r   rate.Limit
	b   int
}

func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	rl := &RateLimiter{
		ips: make(map[string]*ipLimiter),
		r:   rate.Every(time.Minute / time.Duration(requestsPerMinute)),
		b:   requestsPerMinute,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.ips[ip]
	if !exists {
		limiter = &ipLimiter{limiter: rate.NewLimiter(rl.r, rl.b), lastSeen: time.Now()}
		rl.ips[ip] = limiter
	}
	limiter.lastSeen = time.Now()
	return limiter.limiter
}

func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return rl.MiddlewareKey(func(c *gin.Context) string {
		return c.ClientIP()
	})
}

func (rl *RateLimiter) MiddlewareKey(keyFn func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := keyFn(c)
		if key == "" {
			key = c.ClientIP()
		}
		if !rl.getLimiter(key).Allow() {
			NewErrorResponse(c, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		c.Next()
	}
}

func (rl *RateLimiter) cleanup() {
	for {
		time.Sleep(time.Minute)
		rl.mu.Lock()
		for ip, limiter := range rl.ips {
			if time.Since(limiter.lastSeen) > 3*time.Minute {
				delete(rl.ips, ip)
			}
		}
		rl.mu.Unlock()
	}
}
