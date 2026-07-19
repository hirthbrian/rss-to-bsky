// Package store persists the origin identifiers of feed items that have
// already been posted, so a restart doesn't repost the whole feed.
package store

import (
	"database/sql"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

// migrate creates the current schema on a fresh database, and upgrades a
// pre-existing database from the legacy `guid`-primary-key schema to the
// current `id` (UUID) / `origin` schema, preserving existing rows.
func migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS posted (
		id TEXT PRIMARY KEY,
		origin TEXT NOT NULL UNIQUE,
		posted_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return err
	}

	hasLegacyGUID, err := columnExists(db, "posted", "guid")
	if err != nil {
		return err
	}
	if !hasLegacyGUID {
		return nil
	}

	rows, err := db.Query(`SELECT guid, posted_at FROM posted`)
	if err != nil {
		return err
	}
	type legacyRow struct {
		origin   string
		postedAt string
	}
	var legacy []legacyRow
	for rows.Next() {
		var r legacyRow
		if err := rows.Scan(&r.origin, &r.postedAt); err != nil {
			rows.Close()
			return err
		}
		legacy = append(legacy, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`ALTER TABLE posted RENAME TO posted_legacy`); err != nil {
		return err
	}
	if _, err := tx.Exec(`CREATE TABLE posted (
		id TEXT PRIMARY KEY,
		origin TEXT NOT NULL UNIQUE,
		posted_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return err
	}
	for _, r := range legacy {
		if _, err := tx.Exec(
			`INSERT INTO posted (id, origin, posted_at) VALUES (?, ?, ?)`,
			uuid.New().String(), r.origin, r.postedAt,
		); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DROP TABLE posted_legacy`); err != nil {
		return err
	}

	return tx.Commit()
}

func columnExists(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			ctype      string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &defaultVal, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) AlreadyPosted(origin string) (bool, error) {
	var exists int
	err := s.db.QueryRow(`SELECT 1 FROM posted WHERE origin = ?`, origin).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) MarkPosted(origin string) error {
	_, err := s.db.Exec(`INSERT INTO posted (id, origin) VALUES (?, ?)`, uuid.New().String(), origin)
	return err
}
