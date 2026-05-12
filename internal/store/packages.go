package store

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/Masterminds/semver/v3"
	"github.com/skpm-dev/registry/internal/models"
)

// RemoveFromIndex removes a package entry from the index by name.
func RemoveFromIndex(index *Index, name string) *Index {
	kept := make([]models.PackageSummary, 0, len(index.Packages))
	for _, p := range index.Packages {
		if p.Name != name {
			kept = append(kept, p)
		}
	}
	index.Packages = kept
	return index
}

// LatestNonYanked returns the highest semver version in the map that is not yanked.
// Returns "" if every version is yanked.
func LatestNonYanked(versions map[string]models.VersionEntry) string {
	var bestSV *semver.Version
	best := ""
	for v, entry := range versions {
		if entry.Yanked {
			continue
		}
		sv, err := semver.NewVersion(v)
		if err != nil {
			continue
		}
		if bestSV == nil || sv.GreaterThan(bestSV) {
			bestSV = sv
			best = v
		}
	}
	return best
}

const rawBase = "https://raw.githubusercontent.com/skpm-dev/registry/main"

type Index struct {
	Version  string                  `json:"version"`
	Packages []models.PackageSummary `json:"packages"`
}

func githubGet(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if tok := os.Getenv("REGISTRY_GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	return http.DefaultClient.Do(req)
}

// GetPackage fetches a package entry from the registry repo on GitHub.
// Returns nil if the package does not exist yet.
func GetPackage(name string) (*models.Package, error) {
	url := fmt.Sprintf("%s/packages/%s.json", rawBase, name)

	resp, err := githubGet(url)
	if err != nil {
		return nil, fmt.Errorf("could not reach registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned %d", resp.StatusCode)
	}

	var pkg models.Package
	if err := json.NewDecoder(resp.Body).Decode(&pkg); err != nil {
		return nil, fmt.Errorf("could not decode package: %w", err)
	}

	return &pkg, nil
}

// GetIndex fetches the current index.json from the registry repo.
func GetIndex() (*Index, error) {
	url := fmt.Sprintf("%s/index.json", rawBase)

	resp, err := githubGet(url)
	if err != nil {
		return nil, fmt.Errorf("could not reach registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned %d fetching index", resp.StatusCode)
	}

	var index Index
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		return nil, fmt.Errorf("could not decode index: %w", err)
	}

	return &index, nil
}

// ListPackages fetches all package summaries from the index, with live download counts.
func ListPackages() ([]models.PackageSummary, error) {
	index, err := GetIndex()
	if err != nil {
		return nil, err
	}
	for i, p := range index.Packages {
		index.Packages[i].Downloads = GetDownloads(p.Name)
	}
	return index.Packages, nil
}

// BuildUpdatedIndex returns the index with the package added or updated.
func BuildUpdatedIndex(index *Index, pkg *models.Package) *Index {
	summary := models.PackageSummary{
		Name:        pkg.Name,
		Description: pkg.Description,
		Author:      pkg.Author,
		Latest:      pkg.Latest,
	}

	for i, p := range index.Packages {
		if p.Name == pkg.Name {
			index.Packages[i] = summary
			return index
		}
	}

	index.Packages = append(index.Packages, summary)
	return index
}

type PublishParams struct {
	Name         string
	Description  string
	Author       string
	Version      string
	Skript       string
	Minecraft    string
	Addons       map[string]string
	Dependencies map[string]string
	Filenames    []string
	Checksums    map[string]string // filename → "sha256:<hex>"
}

// BuildPackageEntry creates a new package entry or merges a new version into an existing one.
func BuildPackageEntry(existing *models.Package, p PublishParams) *models.Package {
	versionEntry := models.VersionEntry{
		Skript:       p.Skript,
		Minecraft:    p.Minecraft,
		Addons:       p.Addons,
		Dependencies: p.Dependencies,
		Files:        buildFileEntries(p.Name, p.Version, p.Filenames, p.Checksums),
	}

	if existing == nil {
		return &models.Package{
			Name:        p.Name,
			Description: p.Description,
			Author:      p.Author,
			Latest:      p.Version,
			Versions:    map[string]models.VersionEntry{p.Version: versionEntry},
		}
	}

	existing.Versions[p.Version] = versionEntry
	// Only advance Latest if the new version is strictly greater, so a
	// backport publish cannot roll users back to an older version.
	newSV, newErr := semver.NewVersion(p.Version)
	curSV, curErr := semver.NewVersion(existing.Latest)
	if newErr == nil && (curErr != nil || newSV.GreaterThan(curSV)) {
		existing.Latest = p.Version
	}
	return existing
}

func buildFileEntries(packageName, version string, filenames []string, checksums map[string]string) []models.FileEntry {
	entries := make([]models.FileEntry, len(filenames))
	for i, name := range filenames {
		entries[i] = models.FileEntry{
			Name:   name,
			URL:    fmt.Sprintf("%s/files/%s/%s/%s", rawBase, packageName, version, name),
			SHA256: checksums[name],
		}
	}
	return entries
}
