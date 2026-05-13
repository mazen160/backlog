package migrate

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Run(db *sql.DB) error {
	if err := ensureMeta(db); err != nil {
		return fmt.Errorf("migrate: ensure meta: %w", err)
	}

	version, err := getVersion(db)
	if err != nil {
		return fmt.Errorf("migrate: get version: %w", err)
	}

	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("migrate: read dir: %w", err)
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for i, name := range names {
		migVer := i + 1
		if migVer <= version {
			continue
		}
		content, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("migrate: read %s: %w", name, err)
		}
		if err := execMigration(db, string(content), migVer); err != nil {
			return fmt.Errorf("migrate: run %s: %w", name, err)
		}
	}
	return nil
}

func ensureMeta(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`)
	return err
}

func getVersion(db *sql.DB) (int, error) {
	var v int
	err := db.QueryRow(`SELECT COALESCE((SELECT CAST(value AS INTEGER) FROM schema_meta WHERE key='schema_version'), 0)`).Scan(&v)
	return v, err
}

func execMigration(db *sql.DB, sql string, version int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.Exec(sql); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT OR REPLACE INTO schema_meta(key,value) VALUES('schema_version',?)`, fmt.Sprintf("%d", version)); err != nil {
		return err
	}
	return tx.Commit()
}
