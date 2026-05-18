package store

import "log"

func IncrementDownloads(name, version string) {
	if db == nil {
		return
	}
	_, err := db.Exec(`
		INSERT INTO downloads (package_name, version, count) VALUES ($1, $2, 1)
		ON CONFLICT (package_name, version) DO UPDATE SET count = downloads.count + 1
	`, name, version)
	if err != nil {
		log.Printf("could not increment downloads for %s@%s: %v", name, version, err)
	}
}

// GetDownloads returns the total download count across all versions of a package.
func GetDownloads(name string) int64 {
	if db == nil {
		return 0
	}
	var count int64
	db.QueryRow(`SELECT COALESCE(SUM(count), 0) FROM downloads WHERE package_name = $1`, name).Scan(&count)
	return count
}

// GetVersionDownloads returns the download count for a specific version.
func GetVersionDownloads(name, version string) int64 {
	if db == nil {
		return 0
	}
	var count int64
	db.QueryRow(`SELECT count FROM downloads WHERE package_name = $1 AND version = $2`, name, version).Scan(&count)
	return count
}
