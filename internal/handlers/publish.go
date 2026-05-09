package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/skpm-dev/registry/internal/db"
	"github.com/skpm-dev/registry/internal/github"
	"github.com/skpm-dev/registry/internal/models"
)

type publishRequest struct {
	Manifest publishManifest   `json:"manifest"`
	Files    map[string]string `json:"files"`
}

type publishManifest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Repo        string            `json:"repo"`
	Skript      string            `json:"skript"`
	Minecraft   string            `json:"minecraft"`
	Addons      map[string]string `json:"addons"`
}

func Publish(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := github.ExtractToken(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}

		user, err := github.GetAuthenticatedUser(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}

		var req publishRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if err := validatePublishRequest(req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		existingAuthor, err := db.GetPackageAuthor(database, req.Manifest.Name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not check package ownership")
			return
		}

		if existingAuthor != "" && existingAuthor != user.Login {
			writeError(w, http.StatusForbidden, fmt.Sprintf(
				"package %q is owned by %s", req.Manifest.Name, existingAuthor,
			))
			return
		}

		pkg := buildPackage(req, user.Login)

		if err := db.UpsertPackage(database, pkg, req.Files); err != nil {
			writeError(w, http.StatusInternalServerError, "could not save package")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("published %s@%s", req.Manifest.Name, req.Manifest.Version),
		})
	}
}

func buildPackage(req publishRequest, author string) *models.Package {
	return &models.Package{
		Name:        req.Manifest.Name,
		Description: req.Manifest.Description,
		Author:      author,
		Latest:      req.Manifest.Version,
		Versions: map[string]models.VersionEntry{
			req.Manifest.Version: {
				Skript:    req.Manifest.Skript,
				Minecraft: req.Manifest.Minecraft,
				Addons:    req.Manifest.Addons,
			},
		},
	}
}

func validatePublishRequest(req publishRequest) error {
	m := req.Manifest
	switch {
	case m.Name == "":
		return fmt.Errorf("manifest missing name")
	case m.Version == "":
		return fmt.Errorf("manifest missing version")
	case m.Description == "":
		return fmt.Errorf("manifest missing description")
	case len(req.Files) == 0:
		return fmt.Errorf("no files provided")
	}
	return nil
}
