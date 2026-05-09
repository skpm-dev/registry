package db

import (
	"database/sql"
	"fmt"

	"github.com/skpm-dev/registry/internal/models"
)

func GetPackage(database *sql.DB, name string) (*models.Package, error) {
	row := database.QueryRow(`
		SELECT id, name, description, author, latest
		FROM packages WHERE name = ?`, name)

	var pkg models.Package
	var pkgID int
	err := row.Scan(&pkgID, &pkg.Name, &pkg.Description, &pkg.Author, &pkg.Latest)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not fetch package: %w", err)
	}

	versions, err := getVersions(database, pkgID, pkg.Name)
	if err != nil {
		return nil, err
	}
	pkg.Versions = versions

	return &pkg, nil
}

func ListPackages(database *sql.DB) ([]models.PackageSummary, error) {
	rows, err := database.Query(`
		SELECT name, description, author, latest
		FROM packages ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("could not list packages: %w", err)
	}
	defer rows.Close()

	var summaries []models.PackageSummary
	for rows.Next() {
		var s models.PackageSummary
		if err := rows.Scan(&s.Name, &s.Description, &s.Author, &s.Latest); err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}

	return summaries, nil
}

func SearchPackages(database *sql.DB, query string) ([]models.PackageSummary, error) {
	like := "%" + query + "%"
	rows, err := database.Query(`
		SELECT name, description, author, latest
		FROM packages
		WHERE name LIKE ? OR description LIKE ?
		ORDER BY name`, like, like)
	if err != nil {
		return nil, fmt.Errorf("could not search packages: %w", err)
	}
	defer rows.Close()

	var summaries []models.PackageSummary
	for rows.Next() {
		var s models.PackageSummary
		if err := rows.Scan(&s.Name, &s.Description, &s.Author, &s.Latest); err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}

	return summaries, nil
}

func GetFile(database *sql.DB, packageName, version, filename string) (string, error) {
	row := database.QueryRow(`
		SELECT vf.content
		FROM version_files vf
		JOIN versions v ON vf.version_id = v.id
		JOIN packages p ON v.package_id = p.id
		WHERE p.name = ? AND v.version = ? AND vf.name = ?`,
		packageName, version, filename)

	var content string
	err := row.Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("could not fetch file: %w", err)
	}

	return content, nil
}

func UpsertPackage(database *sql.DB, pkg *models.Package, files map[string]string) error {
	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("could not begin transaction: %w", err)
	}
	defer tx.Rollback()

	var pkgID int64
	row := tx.QueryRow(`SELECT id FROM packages WHERE name = ?`, pkg.Name)
	err = row.Scan(&pkgID)

	if err == sql.ErrNoRows {
		result, err := tx.Exec(`
			INSERT INTO packages (name, description, author, latest)
			VALUES (?, ?, ?, ?)`,
			pkg.Name, pkg.Description, pkg.Author, pkg.Latest)
		if err != nil {
			return fmt.Errorf("could not insert package: %w", err)
		}
		pkgID, _ = result.LastInsertId()
	} else if err != nil {
		return fmt.Errorf("could not look up package: %w", err)
	} else {
		_, err = tx.Exec(`
			UPDATE packages SET description = ?, latest = ?
			WHERE id = ?`,
			pkg.Description, pkg.Latest, pkgID)
		if err != nil {
			return fmt.Errorf("could not update package: %w", err)
		}
	}

	versionEntry := pkg.Versions[pkg.Latest]
	result, err := tx.Exec(`
		INSERT INTO versions (package_id, version, skript_req, minecraft_req)
		VALUES (?, ?, ?, ?)`,
		pkgID, pkg.Latest, versionEntry.Skript, versionEntry.Minecraft)
	if err != nil {
		return fmt.Errorf("could not insert version: %w", err)
	}
	versionID, _ := result.LastInsertId()

	for name, content := range files {
		_, err := tx.Exec(`
			INSERT INTO version_files (version_id, name, content)
			VALUES (?, ?, ?)`,
			versionID, name, content)
		if err != nil {
			return fmt.Errorf("could not insert file %s: %w", name, err)
		}
	}

	for addonName, addonVersion := range versionEntry.Addons {
		_, err := tx.Exec(`
			INSERT INTO version_addons (version_id, name, version_req)
			VALUES (?, ?, ?)`,
			versionID, addonName, addonVersion)
		if err != nil {
			return fmt.Errorf("could not insert addon %s: %w", addonName, err)
		}
	}

	return tx.Commit()
}

func GetPackageAuthor(database *sql.DB, name string) (string, error) {
	row := database.QueryRow(`SELECT author FROM packages WHERE name = ?`, name)
	var author string
	err := row.Scan(&author)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("could not fetch author: %w", err)
	}
	return author, nil
}

func getVersions(database *sql.DB, pkgID int, pkgName string) (map[string]models.VersionEntry, error) {
	rows, err := database.Query(`
		SELECT id, version, skript_req, minecraft_req
		FROM versions WHERE package_id = ?`, pkgID)
	if err != nil {
		return nil, fmt.Errorf("could not fetch versions: %w", err)
	}
	defer rows.Close()

	versions := make(map[string]models.VersionEntry)
	for rows.Next() {
		var versionID int
		var versionStr, skript, minecraft string
		if err := rows.Scan(&versionID, &versionStr, &skript, &minecraft); err != nil {
			return nil, err
		}

		files, err := getVersionFiles(database, versionID, pkgName, versionStr)
		if err != nil {
			return nil, err
		}

		addons, err := getVersionAddons(database, versionID)
		if err != nil {
			return nil, err
		}

		versions[versionStr] = models.VersionEntry{
			Skript:    skript,
			Minecraft: minecraft,
			Files:     files,
			Addons:    addons,
		}
	}

	return versions, nil
}

func getVersionFiles(database *sql.DB, versionID int, pkgName, version string) ([]models.FileEntry, error) {
	rows, err := database.Query(`
		SELECT name FROM version_files WHERE version_id = ?`, versionID)
	if err != nil {
		return nil, fmt.Errorf("could not fetch files: %w", err)
	}
	defer rows.Close()

	var files []models.FileEntry
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		files = append(files, models.FileEntry{
			Name: name,
			URL:  fmt.Sprintf("/packages/%s/versions/%s/files/%s", pkgName, version, name),
		})
	}

	return files, nil
}

func getVersionAddons(database *sql.DB, versionID int) (map[string]string, error) {
	rows, err := database.Query(`
		SELECT name, version_req FROM version_addons WHERE version_id = ?`, versionID)
	if err != nil {
		return nil, fmt.Errorf("could not fetch addons: %w", err)
	}
	defer rows.Close()

	addons := make(map[string]string)
	for rows.Next() {
		var name, versionReq string
		if err := rows.Scan(&name, &versionReq); err != nil {
			return nil, err
		}
		addons[name] = versionReq
	}

	return addons, nil
}
