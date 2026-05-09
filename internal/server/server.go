package server

import (
	"fmt"
	"net/http"

	"github.com/skpm-dev/registry/internal/handlers"
)

func New(port string) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /publish", handlers.Publish)
	mux.HandleFunc("GET /packages", handlers.ListPackages)
	mux.HandleFunc("GET /packages/{name}", handlers.GetPackage)
	mux.HandleFunc("GET /packages/{name}/versions/{version}/files/{filename}", handlers.DownloadFile)
	mux.HandleFunc("GET /search", handlers.SearchPackages)

	return &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: mux,
	}
}
