package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/skpm-dev/registry/internal/github"
	"github.com/skpm-dev/registry/internal/store"
)

type removeRequest struct {
	Reason string `json:"reason"`
}

func requireAdmin(r *http.Request) error {
	token := os.Getenv("REGISTRY_ADMIN_TOKEN")
	if token == "" {
		return fmt.Errorf("admin endpoint not configured")
	}
	auth := r.Header.Get("Authorization")
	if subtle.ConstantTimeCompare([]byte(auth), []byte("Bearer "+token)) != 1 {
		return fmt.Errorf("unauthorized")
	}
	return nil
}

// YankVersion marks a single version as yanked and recalculates latest.
// DELETE /packages/{name}/{version}
func YankVersion(w http.ResponseWriter, r *http.Request) {
	if err := requireAdmin(r); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	name := r.PathValue("name")
	version := r.PathValue("version")

	var req removeRequest
	json.NewDecoder(r.Body).Decode(&req)

	pkg, err := store.GetPackage(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch package")
		return
	}
	if pkg == nil {
		writeError(w, http.StatusNotFound, "package not found")
		return
	}

	v, ok := pkg.Versions[version]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("version %s not found", version))
		return
	}

	v.Yanked = true
	v.YankReason = req.Reason
	pkg.Versions[version] = v
	pkg.Latest = store.LatestNonYanked(pkg.Versions)

	pkgJSON, err := marshalIndent(pkg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not encode package")
		return
	}

	msg := fmt.Sprintf("yank %s@%s", name, version)
	if req.Reason != "" {
		msg += ": " + req.Reason
	}

	client := adminRepoClient()
	if err := client.CommitFile("main", fmt.Sprintf("packages/%s.json", name), pkgJSON, msg); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("could not commit yank: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": fmt.Sprintf("yanked %s@%s", name, version),
	})
}

// RemovePackage hard-removes a package: deletes all .sk files from the repo,
// writes a tombstone so callers get 410 Gone instead of 404, and removes it
// from the index so it no longer appears in search or listings.
// DELETE /packages/{name}
func RemovePackage(w http.ResponseWriter, r *http.Request) {
	if err := requireAdmin(r); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	name := r.PathValue("name")

	var req removeRequest
	json.NewDecoder(r.Body).Decode(&req)

	pkg, err := store.GetPackage(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch package")
		return
	}
	if pkg == nil {
		writeError(w, http.StatusNotFound, "package not found")
		return
	}

	client := adminRepoClient()

	// Delete every .sk file for every version
	for version := range pkg.Versions {
		dir := fmt.Sprintf("files/%s/%s", name, version)
		files, err := client.ListDirectory(dir)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("could not list files for %s@%s: %v", name, version, err))
			return
		}
		for _, f := range files {
			if err := client.DeleteFile(f, fmt.Sprintf("remove %s", f)); err != nil {
				writeError(w, http.StatusInternalServerError, fmt.Sprintf("could not delete %s: %v", f, err))
				return
			}
		}
	}

	// Write tombstone so GET /packages/{name} returns 410 with a reason
	pkg.Removed = true
	pkg.RemoveReason = req.Reason
	tombstoneJSON, err := marshalIndent(pkg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not encode tombstone")
		return
	}
	tombMsg := fmt.Sprintf("remove %s", name)
	if req.Reason != "" {
		tombMsg += ": " + req.Reason
	}
	if err := client.CommitFile("main", fmt.Sprintf("packages/%s.json", name), tombstoneJSON, tombMsg); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("could not write tombstone: %v", err))
		return
	}

	// Remove from index so it disappears from listings and search
	index, err := store.GetIndex()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch index")
		return
	}
	indexJSON, err := marshalIndent(store.RemoveFromIndex(index, name))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not encode index")
		return
	}
	if err := client.CommitFile("main", "index.json", indexJSON, fmt.Sprintf("remove %s from index", name)); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("could not update index: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": fmt.Sprintf("removed %s", name),
	})
}

func adminRepoClient() *github.RepoClient {
	return github.NewRepoClient(os.Getenv("REGISTRY_GITHUB_TOKEN"), "skpm-dev", "registry")
}
