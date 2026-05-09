package store

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/skpm-dev/registry/internal/models"
)

const rawBase = "https://raw.githubusercontent.com/skpm-dev/registry/main"

type Index struct {
	Version  string                   `json:"version"`
	Packages []models.PackageSummary  `json:"packages"`
}

// GetPackage fetches a package entry from the registry repo on GitHub.
// Returns nil if the package does not exist yet.
func GetPackage(name string) (*models.Package, error) {
	url := fmt.Sprintf("%s/packages/%s.json", rawBase, name)

	resp, err := http.Get(url)
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

	resp, err := http.Get(url)
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

// ListPackages fetches all package summaries from the index.
func ListPackages() ([]models.PackageSummary, error) {
	index, err := GetIndex()
	if err != nil {
		return nil, err
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

// BuildPackageEntry creates a new package entry or merges a new version into an existing one.
func BuildPackageEntry(existing *models.Package, name, description, author, version, skript, minecraft string, addons map[string]string, filenames []string) *models.Package {
	versionEntry := models.VersionEntry{
		Skript:    skript,
		Minecraft: minecraft,
		Addons:    addons,
		Files:     buildFileEntries(name, version, filenames),
	}

	if existing == nil {
		return &models.Package{
			Name:        name,
			Description: description,
			Author:      author,
			Latest:      version,
			Versions:    map[string]models.VersionEntry{version: versionEntry},
		}
	}

	existing.Latest = version
	existing.Versions[version] = versionEntry
	return existing
}

func buildFileEntries(packageName, version string, filenames []string) []models.FileEntry {
	entries := make([]models.FileEntry, len(filenames))
	for i, name := range filenames {
		entries[i] = models.FileEntry{
			Name: name,
			URL:  fmt.Sprintf("%s/files/%s/%s/%s", rawBase, packageName, version, name),
		}
	}
	return entries
}
