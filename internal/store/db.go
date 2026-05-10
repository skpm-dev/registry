package store

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func InitDB(path string) {
	var err error
	db, err = sql.Open("sqlite", path)
	if err != nil {
		log.Fatalf("could not open sqlite db: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS downloads (
		package_name TEXT PRIMARY KEY,
		count        INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		log.Fatalf("could not init downloads table: %v", err)
	}
}
