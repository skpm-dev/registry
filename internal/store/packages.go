package store

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

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

type cacheEntry struct {
	etag string
	body []byte
}

// githubCache is a bounded ETag cache. When full, the oldest entries are evicted
// by clearing the entire map (a cache miss just causes a fresh GitHub fetch).
type boundedCache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry
	maxSize int
}

func (c *boundedCache) load(url string) (cacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[url]
	return e, ok
}

func (c *boundedCache) store(url string, e cacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[url]; !exists && len(c.entries) >= c.maxSize {
		// Evict all entries. Cache misses are cheap: one extra GitHub CDN round-trip.
		c.entries = make(map[string]cacheEntry, c.maxSize)
	}
	c.entries[url] = e
}

var githubCache = &boundedCache{
	entries: make(map[string]cacheEntry, 128),
	maxSize: 500,
}

// cachedFetch fetches url with ETag-based conditional GET. On 304 it returns
// the cached body. On 200 it updates the cache if the response carries an ETag.
// Returns the response body bytes and HTTP status code.
func cachedFetch(url string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	if tok := os.Getenv("REGISTRY_GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	if cached, ok := githubCache.load(url); ok {
		req.Header.Set("If-None-Match", cached.etag)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		if cached, ok := githubCache.load(url); ok {
			return cached.body, http.StatusOK, nil
		}
		return nil, http.StatusNotModified, fmt.Errorf("got 304 with no cache entry for %s", url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	if resp.StatusCode == http.StatusOK {
		if etag := resp.Header.Get("ETag"); etag != "" {
			githubCache.store(url, cacheEntry{etag: etag, body: body})
		}
	}

	return body, resp.StatusCode, nil
}

// GetPackage fetches a package entry from the registry repo on GitHub.
// Returns nil if the package does not exist yet.
func GetPackage(name string) (*models.Package, error) {
	url := fmt.Sprintf("%s/packages/%s.json", rawBase, name)

	body, status, err := cachedFetch(url)
	if err != nil {
		return nil, fmt.Errorf("could not reach registry: %w", err)
	}
	if status == http.StatusNotFound {
		return nil, nil
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("registry returned %d", status)
	}

	var pkg models.Package
	if err := json.Unmarshal(body, &pkg); err != nil {
		return nil, fmt.Errorf("could not decode package: %w", err)
	}
	return &pkg, nil
}

// GetIndex fetches the current index.json from the registry repo.
func GetIndex() (*Index, error) {
	url := fmt.Sprintf("%s/index.json", rawBase)

	body, status, err := cachedFetch(url)
	if err != nil {
		return nil, fmt.Errorf("could not reach registry: %w", err)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("registry returned %d fetching index", status)
	}

	var index Index
	if err := json.Unmarshal(body, &index); err != nil {
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
	if isNewerVersion(p.Version, existing.Latest) {
		existing.Latest = p.Version
	}
	return existing
}

// isNewerVersion reports whether candidate is a valid semver that is strictly
// greater than current. Returns true when current is absent or unparseable,
// so any valid version beats an empty or corrupt Latest field.
func isNewerVersion(candidate, current string) bool {
	newSV, err := semver.NewVersion(candidate)
	if err != nil {
		return false
	}
	curSV, err := semver.NewVersion(current)
	if err != nil {
		return true
	}
	return newSV.GreaterThan(curSV)
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
