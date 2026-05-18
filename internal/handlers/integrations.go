package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/skpm-dev/registry/internal/github"
	"github.com/skpm-dev/registry/internal/store"
)

// PublishWithKey handles POST /v1/packages.
// Requires an API key in Authorization and a GitHub token in X-GitHub-Token.
// The GitHub token is used only to verify the caller's identity (read:user).
func PublishWithKey(w http.ResponseWriter, r *http.Request) {
	ghToken := r.Header.Get("X-GitHub-Token")
	if ghToken == "" {
		writeError(w, http.StatusUnauthorized, "missing X-GitHub-Token header")
		return
	}

	user, err := github.GetAuthenticatedUser(ghToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
	var req publishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := validatePublishRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	existing, err := store.GetPackage(req.Manifest.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not check existing package")
		return
	}

	if existing != nil && existing.Author != user.Login {
		writeError(w, http.StatusForbidden, fmt.Sprintf(
			"package %q is owned by %s", req.Manifest.Name, existing.Author,
		))
		return
	}

	if existing != nil {
		if _, exists := existing.Versions[req.Manifest.Version]; exists {
			writeError(w, http.StatusConflict, fmt.Sprintf(
				"version %s of %s already exists — bump your version and try again",
				req.Manifest.Version, req.Manifest.Name,
			))
			return
		}
	}

	filenames, checksums := checksumFiles(req.Files)

	pkg := store.BuildPackageEntry(existing, store.PublishParams{
		Name:         req.Manifest.Name,
		Description:  req.Manifest.Description,
		Author:       user.Login,
		Version:      req.Manifest.Version,
		Skript:       req.Manifest.Skript,
		Minecraft:    req.Manifest.Minecraft,
		Addons:       req.Manifest.Addons,
		Dependencies: req.Manifest.Dependencies,
		Filenames:    filenames,
		Checksums:    checksums,
	})

	prURL, err := openPublishPR(req, pkg, user.Login)
	if err != nil {
		if errors.Is(err, errVersionConflict) {
			writeError(w, http.StatusConflict, fmt.Sprintf("version %s already has an open pull request — bump your version and try again", req.Manifest.Version))
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("could not open PR: %v", err))
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"message": fmt.Sprintf("pull request opened for %s@%s", req.Manifest.Name, req.Manifest.Version),
		"pr":      prURL,
	})
}

// GetFileContent handles GET /v1/packages/{name}/versions/{version}/files/{filename}.
// Returns the raw file content inline rather than redirecting to GitHub.
func GetFileContent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	version := r.PathValue("version")
	filename := r.PathValue("filename")

	if !rePackageName.MatchString(name) || !reSafeFilename.MatchString(filename) {
		writeError(w, http.StatusBadRequest, "invalid name, version, or filename")
		return
	}
	if strings.Contains(version, "..") || strings.Contains(version, "/") {
		writeError(w, http.StatusBadRequest, "invalid name, version, or filename")
		return
	}

	if yankMessage, yanked := yankMessageFor(name, version); yanked {
		writeError(w, http.StatusGone, yankMessage)
		return
	}

	rawURL := fmt.Sprintf("%s/files/%s/%s/%s", rawBase, name, version, filename)
	resp, err := http.Get(rawURL) //nolint:noctx
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not fetch file from registry")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, "could not fetch file from registry")
		return
	}

	store.IncrementDownloads(name, version)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, resp.Body) //nolint:errcheck
}
