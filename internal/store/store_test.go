package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestOpen_FreshDatabase(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "posted.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer s.Close()

	posted, err := s.AlreadyPosted("abc")
	if err != nil {
		t.Fatalf("AlreadyPosted() error = %v", err)
	}
	if posted {
		t.Error("expected fresh database to report nothing posted")
	}
}

func TestOpen_CreatesParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "posted.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer s.Close()
}

func TestMarkPosted_RoundTrip(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "posted.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer s.Close()

	const origin = "https://example.com/post/1"

	if err := s.MarkPosted(origin); err != nil {
		t.Fatalf("MarkPosted() error = %v", err)
	}

	posted, err := s.AlreadyPosted(origin)
	if err != nil {
		t.Fatalf("AlreadyPosted() error = %v", err)
	}
	if !posted {
		t.Error("expected origin to be marked as posted")
	}
}

func TestMarkPosted_DuplicateOriginRejected(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "posted.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer s.Close()

	const origin = "dup"
	if err := s.MarkPosted(origin); err != nil {
		t.Fatalf("first MarkPosted() error = %v", err)
	}
	if err := s.MarkPosted(origin); err == nil {
		t.Error("expected duplicate origin to violate the UNIQUE constraint")
	}
}

func TestOpen_MigratesLegacySchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "posted.db")

	// Seed a database using the pre-migration schema (guid as primary key,
	// no id/origin columns) to exercise the upgrade path in migrate().
	legacyDB, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("opening legacy db: %v", err)
	}
	if _, err := legacyDB.Exec(`CREATE TABLE posted (
		guid TEXT PRIMARY KEY,
		posted_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("creating legacy table: %v", err)
	}
	if _, err := legacyDB.Exec(`INSERT INTO posted (guid) VALUES (?), (?)`, "legacy-1", "legacy-2"); err != nil {
		t.Fatalf("seeding legacy rows: %v", err)
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatalf("closing legacy db: %v", err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer s.Close()

	for _, origin := range []string{"legacy-1", "legacy-2"} {
		posted, err := s.AlreadyPosted(origin)
		if err != nil {
			t.Fatalf("AlreadyPosted(%q) error = %v", origin, err)
		}
		if !posted {
			t.Errorf("expected migrated origin %q to be marked posted", origin)
		}
	}

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM posted`).Scan(&count); err != nil {
		t.Fatalf("counting rows: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 rows after migration, got %d", count)
	}

	rows, err := s.db.Query(`SELECT id FROM posted`)
	if err != nil {
		t.Fatalf("querying ids: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scanning id: %v", err)
		}
		if _, err := uuid.Parse(id); err != nil {
			t.Errorf("expected generated id %q to be a valid UUID: %v", id, err)
		}
	}
}
