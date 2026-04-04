package http

import (
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// ipLimiter is a per-IP token bucket rate limiter.
type ipLimiter struct {
	limiters sync.Map
	r        rate.Limit
	burst    int
}

type ipEntry struct {
	limiter  *rate.Limiter
	lastSeen atomic.Int64 // unix nano — atomic for concurrent access from allow() and cleanupLoop()
}

func newIPLimiter(rpm, burst int) *ipLimiter {
	if burst <= 0 {
		burst = 1
	}
	r := rate.Limit(0)
	if rpm > 0 {
		r = rate.Limit(float64(rpm) / 60.0)
	}
	il := &ipLimiter{r: r, burst: burst}
	go il.cleanupLoop()
	return il
}

func (il *ipLimiter) allow(key string) bool {
	if il.r == 0 {
		return true
	}
	newEntry := &ipEntry{limiter: rate.NewLimiter(il.r, il.burst)}
	newEntry.lastSeen.Store(time.Now().UnixNano())

	v, _ := il.limiters.LoadOrStore(key, newEntry)
	entry := v.(*ipEntry)
	entry.lastSeen.Store(time.Now().UnixNano())
	if !entry.limiter.Allow() {
		slog.Warn("security.auth_rate_limited", "ip", key)
		return false
	}
	return true
}

func (il *ipLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-10 * time.Minute)
		il.limiters.Range(func(key, value any) bool {
			if value.(*ipEntry).lastSeen.Load() < cutoff.UnixNano() {
				il.limiters.Delete(key)
			}
			return true
		})
	}
}

// AuthRateLimiter wraps per-endpoint rate limiters for auth endpoints.
type AuthRateLimiter struct {
	login    *ipLimiter
	register *ipLimiter
	refresh  *ipLimiter
}

// NewAuthRateLimiter creates rate limiters for auth endpoints.
// loginRPM: requests per minute for login (recommended: 10).
// registerRPM: requests per minute for register (recommended: 5).
// refreshRPM: requests per minute for refresh (recommended: 20).
func NewAuthRateLimiter(loginRPM, registerRPM, refreshRPM int) *AuthRateLimiter {
	return &AuthRateLimiter{
		login:    newIPLimiter(loginRPM, 3),
		register: newIPLimiter(registerRPM, 2),
		refresh:  newIPLimiter(refreshRPM, 5),
	}
}

// WrapLogin returns an HTTP handler that enforces rate limiting on the login endpoint.
func (rl *AuthRateLimiter) WrapLogin(next http.HandlerFunc) http.HandlerFunc {
	return rl.wrap(rl.login, next)
}

// WrapRegister returns an HTTP handler that enforces rate limiting on the register endpoint.
func (rl *AuthRateLimiter) WrapRegister(next http.HandlerFunc) http.HandlerFunc {
	return rl.wrap(rl.register, next)
}

// WrapRefresh returns an HTTP handler that enforces rate limiting on the refresh endpoint.
func (rl *AuthRateLimiter) WrapRefresh(next http.HandlerFunc) http.HandlerFunc {
	return rl.wrap(rl.refresh, next)
}

func (rl *AuthRateLimiter) wrap(limiter *ipLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !limiter.allow(ip) {
			w.Header().Set("Retry-After", "60")
			writeJSONError(w, http.StatusTooManyRequests, "too many requests — try again later")
			return
		}
		next(w, r)
	}
}
