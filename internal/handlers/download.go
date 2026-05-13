package handlers

import (
	"fmt"
	"net/http"

	"github.com/skpm-dev/registry/internal/store"
)

const rawBase = "https://raw.githubusercontent.com/skpm-dev/registry/main"

// DownloadFile increments the download counter then redirects to the raw GitHub
// URL for the file. Returns 410 Gone if the requested version has been yanked.
func DownloadFile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	version := r.PathValue("version")
	filename := r.PathValue("filename")

	if yankMessage, yanked := yankMessageFor(name, version); yanked {
		writeError(w, http.StatusGone, yankMessage)
		return
	}

	store.IncrementDownloads(name, version)

	rawURL := fmt.Sprintf("%s/files/%s/%s/%s", rawBase, name, version, filename)
	http.Redirect(w, r, rawURL, http.StatusFound)
}

// yankMessageFor returns a yank message and true if the version is yanked.
// Errors fetching the package are treated as "not yanked" so transient GitHub
// failures do not block downloads.
func yankMessageFor(name, version string) (string, bool) {
	pkg, err := store.GetPackage(name)
	if err != nil || pkg == nil {
		return "", false
	}

	entry, ok := pkg.Versions[version]
	if !ok || !entry.Yanked {
		return "", false
	}

	msg := fmt.Sprintf("%s@%s has been yanked", name, version)
	if entry.YankReason != "" {
		msg += ": " + entry.YankReason
	}
	return msg, true
}
