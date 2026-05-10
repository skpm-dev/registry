package store

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
)

var db *sql.DB

func InitDB() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("could not open postgres db: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS downloads (
		package_name TEXT    NOT NULL,
		version      TEXT    NOT NULL,
		count        BIGINT  NOT NULL DEFAULT 0,
		PRIMARY KEY (package_name, version)
	)`)
	if err != nil {
		log.Fatalf("could not init downloads table: %v", err)
	}
}
