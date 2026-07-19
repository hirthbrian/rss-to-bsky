// Package store persists the GUIDs of feed items that have already been
// posted, so a restart doesn't repost the whole feed.
package store

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS posted (
		guid TEXT PRIMARY KEY,
		posted_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) AlreadyPosted(guid string) (bool, error) {
	var exists int
	err := s.db.QueryRow(`SELECT 1 FROM posted WHERE guid = ?`, guid).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) MarkPosted(guid string) error {
	_, err := s.db.Exec(`INSERT INTO posted (guid) VALUES (?)`, guid)
	return err
}
