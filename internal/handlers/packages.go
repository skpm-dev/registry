package handlers

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/skpm-dev/registry/internal/models"
	"github.com/skpm-dev/registry/internal/store"
)

func registryBase() string {
	if base := os.Getenv("REGISTRY_BASE_URL"); base != "" {
		return base
	}
	return "https://registry.skpm.org"
}

// rewriteFileURLs replaces raw GitHub file URLs with registry download endpoints
// so every file download is counted.
func rewriteFileURLs(pkg *models.Package) {
	base := registryBase()
	for version, entry := range pkg.Versions {
		for i, f := range entry.Files {
			if f.Name != "" {
				entry.Files[i].URL = fmt.Sprintf("%s/packages/%s/versions/%s/files/%s",
					base, pkg.Name, version, f.Name)
			}
		}
		pkg.Versions[version] = entry
	}
}

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

	rewriteFileURLs(pkg)
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
