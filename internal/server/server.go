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

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "https://skpm.org")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
