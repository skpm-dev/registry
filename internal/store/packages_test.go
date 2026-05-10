package store_test

import (
	"strings"
	"testing"

	"github.com/skpm-dev/registry/internal/models"
	"github.com/skpm-dev/registry/internal/store"
)

func baseParams() store.PublishParams {
	return store.PublishParams{
		Name:        "economy",
		Description: "A test economy",
		Author:      "testuser",
		Version:     "1.0.0",
		Skript:      ">=2.8.0",
		Minecraft:   ">=1.20",
		Addons:      map[string]string{},
		Filenames:   []string{"economy.sk"},
		Checksums:   map[string]string{"economy.sk": "sha256:abc123"},
	}
}

func TestBuildPackageEntry_newPackage(t *testing.T) {
	p := store.BuildPackageEntry(nil, baseParams())

	if p.Name != "economy" {
		t.Errorf("name: got %q", p.Name)
	}
	if p.Latest != "1.0.0" {
		t.Errorf("latest: got %q", p.Latest)
	}
	if p.Author != "testuser" {
		t.Errorf("author: got %q", p.Author)
	}
	if len(p.Versions) != 1 {
		t.Fatalf("versions: got %d, want 1", len(p.Versions))
	}

	entry := p.Versions["1.0.0"]
	if len(entry.Files) != 1 {
		t.Fatalf("files: got %d, want 1", len(entry.Files))
	}
	if entry.Files[0].Name != "economy.sk" {
		t.Errorf("file name: got %q", entry.Files[0].Name)
	}
	if entry.Files[0].SHA256 != "sha256:abc123" {
		t.Errorf("sha256: got %q", entry.Files[0].SHA256)
	}
}

func TestBuildPackageEntry_fileURLContainsNameAndVersion(t *testing.T) {
	p := store.BuildPackageEntry(nil, baseParams())
	url := p.Versions["1.0.0"].Files[0].URL
	if !strings.Contains(url, "economy") {
		t.Errorf("URL missing package name: %s", url)
	}
	if !strings.Contains(url, "1.0.0") {
		t.Errorf("URL missing version: %s", url)
	}
	if !strings.Contains(url, "economy.sk") {
		t.Errorf("URL missing filename: %s", url)
	}
}

func TestBuildPackageEntry_mergesNewVersion(t *testing.T) {
	existing := store.BuildPackageEntry(nil, baseParams())

	params := baseParams()
	params.Version = "1.0.1"
	params.Checksums = map[string]string{"economy.sk": "sha256:newchecksum"}

	updated := store.BuildPackageEntry(existing, params)

	if updated.Latest != "1.0.1" {
		t.Errorf("latest: got %q, want 1.0.1", updated.Latest)
	}
	if len(updated.Versions) != 2 {
		t.Errorf("versions: got %d, want 2", len(updated.Versions))
	}
	if _, ok := updated.Versions["1.0.0"]; !ok {
		t.Error("original version 1.0.0 was removed")
	}
}

func TestBuildUpdatedIndex_addsNewPackage(t *testing.T) {
	index := &store.Index{Version: "1", Packages: []models.PackageSummary{}}
	pkg := store.BuildPackageEntry(nil, baseParams())

	updated := store.BuildUpdatedIndex(index, pkg)

	if len(updated.Packages) != 1 {
		t.Fatalf("packages: got %d, want 1", len(updated.Packages))
	}
	if updated.Packages[0].Name != "economy" {
		t.Errorf("name: got %q", updated.Packages[0].Name)
	}
}

func TestBuildUpdatedIndex_updatesExisting(t *testing.T) {
	index := &store.Index{
		Version: "1",
		Packages: []models.PackageSummary{
			{Name: "economy", Latest: "1.0.0", Author: "testuser"},
		},
	}

	params := baseParams()
	params.Version = "1.0.1"
	pkg := store.BuildPackageEntry(nil, params)

	updated := store.BuildUpdatedIndex(index, pkg)

	if len(updated.Packages) != 1 {
		t.Fatalf("packages: got %d, want 1", len(updated.Packages))
	}
	if updated.Packages[0].Latest != "1.0.1" {
		t.Errorf("latest: got %q, want 1.0.1", updated.Packages[0].Latest)
	}
}

func TestBuildUpdatedIndex_preservesOtherPackages(t *testing.T) {
	index := &store.Index{
		Version: "1",
		Packages: []models.PackageSummary{
			{Name: "other-package", Latest: "2.0.0"},
		},
	}
	pkg := store.BuildPackageEntry(nil, baseParams())

	updated := store.BuildUpdatedIndex(index, pkg)

	if len(updated.Packages) != 2 {
		t.Fatalf("packages: got %d, want 2", len(updated.Packages))
	}
}
