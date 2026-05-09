package handlers

import (
	"database/sql"
	"net/http"

	"github.com/skpm-dev/registry/internal/db"
)

func DownloadFile(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		version := r.PathValue("version")
		filename := r.PathValue("filename")

		content, err := db.GetFile(database, name, version, filename)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not fetch file")
			return
		}

		if content == "" {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename="+filename)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}
}
