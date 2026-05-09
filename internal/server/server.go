package server

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/skpm-dev/registry/internal/handlers"
)

func New(database *sql.DB, port string) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /publish", handlers.Publish(database))
	mux.HandleFunc("GET /packages", handlers.ListPackages(database))
	mux.HandleFunc("GET /packages/{name}", handlers.GetPackage(database))
	mux.HandleFunc("GET /packages/{name}/versions/{version}/files/{filename}", handlers.DownloadFile(database))
	mux.HandleFunc("GET /search", handlers.SearchPackages(database))

	return &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: mux,
	}
}
