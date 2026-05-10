package handlers

import (
	"net/http"
	"strings"

	"github.com/skpm-dev/registry/internal/models"
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

	if pkg.Removed {
		msg := "this package has been removed from the registry"
		if pkg.RemoveReason != "" {
			msg += ": " + pkg.RemoveReason
		}
		writeError(w, http.StatusGone, msg)
		return
	}

	pkg.Downloads = store.GetDownloads(name)
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
	query := strings.ToLower(r.URL.Query().Get("q"))
	if query == "" {
		writeError(w, http.StatusBadRequest, "missing query parameter q")
		return
	}

	all, err := store.ListPackages()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not search packages")
		return
	}

	var results []models.PackageSummary
	for _, p := range all {
		if strings.Contains(strings.ToLower(p.Name), query) || strings.Contains(strings.ToLower(p.Description), query) {
			results = append(results, p)
		}
	}

	writeJSON(w, http.StatusOK, results)
}
