package handlers

import (
	"fmt"
	"net/http"
)

const rawBase = "https://raw.githubusercontent.com/skpm-dev/registry/main"

// DownloadFile redirects to the raw GitHub URL for the file.
// Files are only available after the publish PR has been merged.
func DownloadFile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	version := r.PathValue("version")
	filename := r.PathValue("filename")

	url := fmt.Sprintf("%s/files/%s/%s/%s", rawBase, name, version, filename)
	http.Redirect(w, r, url, http.StatusFound)
}
