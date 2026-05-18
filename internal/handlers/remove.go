package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/skpm-dev/registry/internal/github"
	"github.com/skpm-dev/registry/internal/models"
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

	entry, ok := pkg.Versions[version]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("version %s not found", version))
		return
	}

	entry.Yanked = true
	entry.YankReason = req.Reason
	pkg.Versions[version] = entry
	pkg.Latest = store.LatestNonYanked(pkg.Versions)

	pkgJSON, err := marshalIndent(pkg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not encode package")
		return
	}

	commitMsg := fmt.Sprintf("yank %s@%s", name, version)
	if req.Reason != "" {
		commitMsg += ": " + req.Reason
	}

	client := adminRepoClient()
	if err := client.CommitFile("main", fmt.Sprintf("packages/%s.json", name), pkgJSON, commitMsg); err != nil {
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

	if err := deleteAllPackageFiles(client, name, pkg.Versions); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := writeTombstone(client, pkg, req.Reason); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := removeFromIndex(client, name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": fmt.Sprintf("removed %s", name),
	})
}

func deleteAllPackageFiles(client *github.RepoClient, packageName string, versions map[string]models.VersionEntry) error {
	for version := range versions {
		dir := fmt.Sprintf("files/%s/%s", packageName, version)
		files, err := client.ListDirectory(dir)
		if err != nil {
			return fmt.Errorf("could not list files for %s@%s: %w", packageName, version, err)
		}
		for _, file := range files {
			if err := client.DeleteFile(file, fmt.Sprintf("remove %s", file)); err != nil {
				return fmt.Errorf("could not delete %s: %w", file, err)
			}
		}
	}
	return nil
}

func writeTombstone(client *github.RepoClient, pkg *models.Package, reason string) error {
	pkg.Removed = true
	pkg.RemoveReason = reason

	tombstoneJSON, err := marshalIndent(pkg)
	if err != nil {
		return fmt.Errorf("could not encode tombstone: %w", err)
	}

	commitMsg := fmt.Sprintf("remove %s", pkg.Name)
	if reason != "" {
		commitMsg += ": " + reason
	}

	if err := client.CommitFile("main", fmt.Sprintf("packages/%s.json", pkg.Name), tombstoneJSON, commitMsg); err != nil {
		return fmt.Errorf("could not write tombstone: %w", err)
	}
	return nil
}

func removeFromIndex(client *github.RepoClient, packageName string) error {
	index, err := store.GetIndex()
	if err != nil {
		return fmt.Errorf("could not fetch index: %w", err)
	}

	indexJSON, err := marshalIndent(store.RemoveFromIndex(index, packageName))
	if err != nil {
		return fmt.Errorf("could not encode index: %w", err)
	}

	if err := client.CommitFile("main", "index.json", indexJSON, fmt.Sprintf("remove %s from index", packageName)); err != nil {
		return fmt.Errorf("could not update index: %w", err)
	}
	return nil
}

func adminRepoClient() *github.RepoClient {
	return github.NewRepoClient(os.Getenv("REGISTRY_GITHUB_TOKEN"), githubOwner(), githubRepo())
}
