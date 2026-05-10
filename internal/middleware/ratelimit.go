package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type bucket struct {
	count  int
	resets time.Time
}

// RateLimit returns middleware that limits each client IP to limit requests per window.
// Stale buckets are purged once per window to bound memory growth.
func RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	var mu sync.Mutex
	buckets := make(map[string]*bucket)

	go func() {
		for range time.Tick(window) {
			mu.Lock()
			now := time.Now()
			for ip, b := range buckets {
				if now.After(b.resets) {
					delete(buckets, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			now := time.Now()

			mu.Lock()
			b, ok := buckets[ip]
			if !ok || now.After(b.resets) {
				b = &bucket{count: 0, resets: now.Add(window)}
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

func clientIP(r *http.Request) string {
	addr := r.RemoteAddr
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}
