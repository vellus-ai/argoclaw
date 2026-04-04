package http

import "net/http"

// GeneralRateLimiter is a per-IP rate limiter for non-auth HTTP endpoints.
// It prevents abuse of public-facing routes (health, API, etc.) by enforcing
// a global request-per-minute limit per source IP.
type GeneralRateLimiter struct {
	limiter *ipLimiter
}

// NewGeneralRateLimiter creates a rate limiter with the given requests-per-minute
// and burst size. Recommended: 60 rpm, burst 10 for general endpoints.
func NewGeneralRateLimiter(rpm, burst int) *GeneralRateLimiter {
	return &GeneralRateLimiter{limiter: newIPLimiter(rpm, burst)}
}

// Wrap returns middleware that enforces the rate limit on all requests.
func (gl *GeneralRateLimiter) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !gl.limiter.allow(ip) {
			w.Header().Set("Retry-After", "60")
			writeJSONError(w, http.StatusTooManyRequests, "too many requests — try again later")
			return
		}
		next.ServeHTTP(w, r)
	})
}
