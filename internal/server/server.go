package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/skpm-dev/registry/internal/handlers"
	"github.com/skpm-dev/registry/internal/middleware"
)

func New(port string) *http.Server {
	mux := http.NewServeMux()

	publishLimit := middleware.RateLimit(10, time.Minute)
	mux.Handle("POST /publish", publishLimit(http.HandlerFunc(handlers.Publish)))
	mux.HandleFunc("GET /packages", handlers.ListPackages)
	mux.HandleFunc("GET /packages/{name}", handlers.GetPackage)
	mux.HandleFunc("DELETE /packages/{name}", handlers.RemovePackage)
	mux.HandleFunc("DELETE /packages/{name}/{version}", handlers.YankVersion)
	mux.HandleFunc("GET /packages/{name}/versions/{version}/files/{filename}", handlers.DownloadFile)
	mux.HandleFunc("GET /search", handlers.SearchPackages)

	globalLimit := middleware.RateLimit(120, time.Minute)
	return &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: globalLimit(corsMiddleware(mux)),
	}
}

var allowedOrigins = map[string]bool{
	"https://skpm.org":       true,
	"https://www.skpm.org":   true,
	"http://localhost:8000":  true,
	"http://localhost:3000":  true,
	"http://localhost:5173":  true,
	"http://127.0.0.1:8000":  true,
	"http://127.0.0.1:3000":  true,
	"http://127.0.0.1:5173":  true,
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		// Read-only endpoints are public API — allow any origin.
		// Mutating endpoints (publish, admin delete) restrict to known origins.
		if r.Method == http.MethodGet || r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
