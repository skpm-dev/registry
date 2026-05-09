package handlers

import (
	"database/sql"
	"net/http"

	"github.com/skpm-dev/registry/internal/db"
)

func GetPackage(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")

		pkg, err := db.GetPackage(database, name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not fetch package")
			return
		}

		if pkg == nil {
			writeError(w, http.StatusNotFound, "package not found")
			return
		}

		writeJSON(w, http.StatusOK, pkg)
	}
}

func ListPackages(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		summaries, err := db.ListPackages(database)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list packages")
			return
		}

		writeJSON(w, http.StatusOK, summaries)
	}
}

func SearchPackages(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if query == "" {
			writeError(w, http.StatusBadRequest, "missing query parameter q")
			return
		}

		summaries, err := db.SearchPackages(database, query)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not search packages")
			return
		}

		writeJSON(w, http.StatusOK, summaries)
	}
}
