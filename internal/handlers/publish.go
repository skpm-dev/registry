package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/skpm-dev/registry/internal/github"
	"github.com/skpm-dev/registry/internal/models"
	"github.com/skpm-dev/registry/internal/store"
)

var (
	rePackageName = regexp.MustCompile(`^[a-z][a-z0-9-]{1,38}$`)
	// safeFilename matches names that cannot escape their directory.
	reSafeFilename = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)
)

var errVersionConflict = errors.New("version already has an open pull request")

type publishRequest struct {
	Manifest publishManifest   `json:"manifest"`
	Files    map[string]string `json:"files"`
}

type publishManifest struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Version      string            `json:"version"`
	Skript       string            `json:"skript"`
	Minecraft    string            `json:"minecraft"`
	Addons       map[string]string `json:"addons"`
	Dependencies map[string]string `json:"dependencies"`
}

func Publish(w http.ResponseWriter, r *http.Request) {
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

	index, err := store.GetIndex()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch registry index")
		return
	}

	filenames := make([]string, 0, len(req.Files))
	checksums := make(map[string]string, len(req.Files))
	for name, content := range req.Files {
		filenames = append(filenames, name)
		checksums[name] = computeChecksum(content)
	}

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

	updatedIndex := store.BuildUpdatedIndex(index, pkg)

	prURL, err := openPublishPR(req, pkg, updatedIndex, user.Login)
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

func openPublishPR(req publishRequest, pkg *models.Package, updatedIndex *store.Index, author string) (string, error) {
	registryToken := os.Getenv("REGISTRY_GITHUB_TOKEN")
	if registryToken == "" {
		return "", fmt.Errorf("REGISTRY_GITHUB_TOKEN is not set")
	}

	client := github.NewRepoClient(registryToken, "skpm-dev", "registry")

	mainSHA, err := client.GetBranchSHA("main")
	if err != nil {
		return "", fmt.Errorf("could not get main branch SHA: %w", err)
	}

	branch := fmt.Sprintf("publish/%s-%s", req.Manifest.Name, req.Manifest.Version)

	if open, err := client.HasOpenPR(branch); err != nil {
		return "", fmt.Errorf("could not check open PRs: %w", err)
	} else if open {
		return "", errVersionConflict
	}

	if err := client.CreateBranch(branch, mainSHA); err != nil {
		return "", fmt.Errorf("could not create branch: %w", err)
	}

	for filename, content := range req.Files {
		path := fmt.Sprintf("files/%s/%s/%s", req.Manifest.Name, req.Manifest.Version, filename)
		msg := fmt.Sprintf("add %s for %s@%s", filename, req.Manifest.Name, req.Manifest.Version)
		if err := client.CommitFile(branch, path, content, msg); err != nil {
			return "", fmt.Errorf("could not commit file %s: %w", filename, err)
		}
	}

	pkgJSON, err := marshalIndent(pkg)
	if err != nil {
		return "", err
	}
	pkgPath := fmt.Sprintf("packages/%s.json", req.Manifest.Name)
	if err := client.CommitFile(branch, pkgPath, pkgJSON, fmt.Sprintf("update packages/%s.json", req.Manifest.Name)); err != nil {
		return "", fmt.Errorf("could not commit package entry: %w", err)
	}

	indexJSON, err := marshalIndent(updatedIndex)
	if err != nil {
		return "", err
	}
	if err := client.CommitFile(branch, "index.json", indexJSON, "update index.json"); err != nil {
		return "", fmt.Errorf("could not commit index: %w", err)
	}

	title := fmt.Sprintf("publish %s@%s by %s", req.Manifest.Name, req.Manifest.Version, author)
	body := fmt.Sprintf("Automated publish request for **%s** version **%s** by @%s.", req.Manifest.Name, req.Manifest.Version, author)

	return client.OpenPR(branch, "main", title, body)
}

func validatePublishRequest(req publishRequest) error {
	m := req.Manifest

	if m.Name == "" {
		return fmt.Errorf("manifest missing name")
	}
	if !rePackageName.MatchString(m.Name) {
		return fmt.Errorf("invalid package name %q: must match ^[a-z][a-z0-9-]{1,38}$", m.Name)
	}
	if m.Version == "" {
		return fmt.Errorf("manifest missing version")
	}
	if _, err := semver.NewVersion(m.Version); err != nil {
		return fmt.Errorf("invalid version %q: must be a valid semver string", m.Version)
	}
	if m.Description == "" {
		return fmt.Errorf("manifest missing description")
	}
	if len(req.Files) == 0 {
		return fmt.Errorf("no files provided")
	}
	for filename := range req.Files {
		if !reSafeFilename.MatchString(filename) || strings.Contains(filename, "..") {
			return fmt.Errorf("invalid filename %q: must match ^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$ and contain no path separators", filename)
		}
	}
	for addon, constraint := range m.Addons {
		if _, err := semver.NewConstraint(constraint); err != nil {
			return fmt.Errorf("invalid semver constraint for addon %q: %q", addon, constraint)
		}
	}
	return nil
}
