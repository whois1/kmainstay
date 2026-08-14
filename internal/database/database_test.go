package database_test

import (
	"path/filepath"
	"testing"

	"kmainstay/internal/database"
)

func TestOpen_WhenDatabaseIsNew_MigratesIdempotently(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	for range 2 {
		db, err := database.Open(path)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		db.Close()
	}
	db, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, table := range []string{"organisations", "users", "organisation_memberships", "conversations", "conversation_members", "messages", "human_sessions", "api_keys", "realtime_events", "schema_migrations"} {
		var got string
		if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&got); err != nil {
			t.Errorf("missing table %s: %v", table, err)
		}
	}
}

func TestOpen_ConfiguresSQLiteForConcurrentIntegrity(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var foreignKeys, busyTimeout int
	var journalMode string
	if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 || busyTimeout != 5000 || journalMode != "wal" {
		t.Fatalf("foreign_keys=%d busy_timeout=%d journal_mode=%q", foreignKeys, busyTimeout, journalMode)
	}
}
