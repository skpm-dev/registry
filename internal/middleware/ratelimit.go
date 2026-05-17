package middleware

import (
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type bucket struct {
	count    int
	resetsAt time.Time
}

// RateLimit returns middleware that limits each client IP to limit requests per window.
// Stale buckets are purged lazily on ~1% of requests to avoid a background goroutine
// that would leak if RateLimit is called multiple times (e.g. in tests).
func RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	var mu sync.Mutex
	buckets := make(map[string]*bucket)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			now := time.Now()

			mu.Lock()
			// Lazy GC: purge expired buckets on ~1% of requests.
			if rand.IntN(100) == 0 {
				for ip, b := range buckets {
					if now.After(b.resetsAt) {
						delete(buckets, ip)
					}
				}
			}
			b, ok := buckets[ip]
			if !ok || now.After(b.resetsAt) {
				b = &bucket{count: 0, resetsAt: now.Add(window)}
				buckets[ip] = b
			}
			b.count++
			over := b.count > limit
			mu.Unlock()

			if over {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP returns the request's real client IP. XFF and X-Real-IP headers are
// only trusted when TRUSTED_PROXY=true, which should only be set when the
// registry runs behind a known reverse proxy that sets these headers.
func clientIP(r *http.Request) string {
	if os.Getenv("TRUSTED_PROXY") == "true" {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if idx := strings.IndexByte(xff, ','); idx != -1 {
				xff = xff[:idx]
			}
			xff = strings.TrimSpace(xff)
			if xff != "" {
				return xff
			}
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
	}
	addr := r.RemoteAddr
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}
