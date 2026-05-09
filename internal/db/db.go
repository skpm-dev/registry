package db

import (
	"database/sql"
	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if err := applySchema(database); err != nil {
		return nil, err
	}

	return database, nil
}

func applySchema(database *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS packages (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT UNIQUE NOT NULL,
		description TEXT NOT NULL,
		author      TEXT NOT NULL,
		latest      TEXT NOT NULL,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS versions (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		package_id   INTEGER NOT NULL REFERENCES packages(id),
		version      TEXT NOT NULL,
		skript_req   TEXT,
		minecraft_req TEXT,
		created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(package_id, version)
	);

	CREATE TABLE IF NOT EXISTS version_files (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id INTEGER NOT NULL REFERENCES versions(id),
		name       TEXT NOT NULL,
		content    TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS version_addons (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id  INTEGER NOT NULL REFERENCES versions(id),
		name        TEXT NOT NULL,
		version_req TEXT NOT NULL
	);`

	_, err := database.Exec(schema)
	return err
}
