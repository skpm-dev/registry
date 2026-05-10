package store

import "log"

func IncrementDownloads(name string) {
	_, err := db.Exec(`
		INSERT INTO downloads (package_name, count) VALUES (?, 1)
		ON CONFLICT(package_name) DO UPDATE SET count = count + 1
	`, name)
	if err != nil {
		log.Printf("could not increment downloads for %s: %v", name, err)
	}
}

func GetDownloads(name string) int64 {
	var count int64
	err := db.QueryRow(`SELECT count FROM downloads WHERE package_name = ?`, name).Scan(&count)
	if err != nil {
		return 0
	}
	return count
}
