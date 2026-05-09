package handlers

import (
	"net/http"

	"github.com/skpm-dev/registry/internal/store"
)

func GetPackage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	pkg, err := store.GetPackage(name)
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

func ListPackages(w http.ResponseWriter, r *http.Request) {
	summaries, err := store.ListPackages()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list packages")
		return
	}

	writeJSON(w, http.StatusOK, summaries)
}

func SearchPackages(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "missing query parameter q")
		return
	}

	all, err := store.ListPackages()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not search packages")
		return
	}

	var results []interface{}
	for _, p := range all {
		if contains(p.Name, query) || contains(p.Description, query) {
			results = append(results, p)
		}
	}

	writeJSON(w, http.StatusOK, results)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
