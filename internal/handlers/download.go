package handlers

import (
	"fmt"
	"net/http"

	"github.com/skpm-dev/registry/internal/store"
)

const rawBase = "https://raw.githubusercontent.com/skpm-dev/registry/main"

// DownloadFile redirects to the raw GitHub URL for the file.
// Returns 410 Gone if the requested version has been yanked.
func DownloadFile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	version := r.PathValue("version")
	filename := r.PathValue("filename")

	pkg, err := store.GetPackage(name)
	if err == nil && pkg != nil {
		if v, ok := pkg.Versions[version]; ok && v.Yanked {
			msg := fmt.Sprintf("%s@%s has been yanked", name, version)
			if v.YankReason != "" {
				msg += ": " + v.YankReason
			}
			writeError(w, http.StatusGone, msg)
			return
		}
	}

	store.IncrementDownloads(name)

	url := fmt.Sprintf("%s/files/%s/%s/%s", rawBase, name, version, filename)
	http.Redirect(w, r, url, http.StatusFound)
}
