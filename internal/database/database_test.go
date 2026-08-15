package database_test

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

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

func TestOpen_MembershipRolesAndNamesAreOrganisationScoped(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "roles.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, statement := range []string{
		`INSERT INTO organisations(id,name,created_at) VALUES('org_one','One','` + now + `')`,
		`INSERT INTO organisations(id,name,created_at) VALUES('org_two','Two','` + now + `')`,
		`INSERT INTO users(id,kind,name,created_at) VALUES('usr_one','bot','Hector','` + now + `')`,
		`INSERT INTO users(id,kind,name,created_at) VALUES('usr_two','bot','hector','` + now + `')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO organisation_memberships(organisation_id,user_id,role,name_normalized,created_at) VALUES('org_one','usr_one','member','hector',?)`, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO organisation_memberships(organisation_id,user_id,role,name_normalized,created_at) VALUES('org_one','usr_two','member','hector',?)`, now); err == nil {
		t.Fatal("duplicate normalised name was accepted in one organisation")
	}
	if _, err := db.Exec(`INSERT INTO organisation_memberships(organisation_id,user_id,role,name_normalized,created_at) VALUES('org_two','usr_two','member','hector',?)`, now); err != nil {
		t.Fatalf("same name in another organisation: %v", err)
	}
	var role string
	if err := db.QueryRow(`SELECT role FROM organisation_memberships WHERE organisation_id='org_one' AND user_id='usr_one'`).Scan(&role); err != nil {
		t.Fatal(err)
	}
	if role != "member" {
		t.Fatalf("role = %q", role)
	}
}

func TestNormalizeName_TrimsUnicodeWhitespaceAndLowercasesUnicode(t *testing.T) {
	if got := database.NormalizeName("\u2003Élodie\u00a0"); got != "élodie" {
		t.Fatalf("normalised name = %q", got)
	}
}

func TestOpen_MigratesVersionOneMemberships(t *testing.T) {
	path := filepath.Join(t.TempDir(), "version-one.db")
	old, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	schema := `
		CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,applied_at TEXT NOT NULL);
		INSERT INTO schema_migrations(version,applied_at) VALUES(1,'2026-01-01T00:00:00Z');
		CREATE TABLE organisations(id TEXT PRIMARY KEY,name TEXT NOT NULL,created_at TEXT NOT NULL);
		CREATE TABLE users(id TEXT PRIMARY KEY,kind TEXT NOT NULL,email TEXT,name TEXT NOT NULL,password_hash TEXT,created_at TEXT NOT NULL);
		CREATE TABLE organisation_memberships(organisation_id TEXT NOT NULL REFERENCES organisations(id),user_id TEXT NOT NULL REFERENCES users(id),created_at TEXT NOT NULL,PRIMARY KEY(organisation_id,user_id));
		INSERT INTO organisations(id,name,created_at) VALUES('org','Mainstay','` + now + `');
		INSERT INTO users(id,kind,name,created_at) VALUES('human','human','Michael','` + now + `'),('later_human','human','Member','` + now + `'),('bot','bot','Hector','` + now + `'),('bot_two','bot',' heCTOR ','` + now + `');
		INSERT INTO organisation_memberships(organisation_id,user_id,created_at) VALUES('org','human','` + now + `'),('org','later_human','` + now + `'),('org','bot','` + now + `'),('org','bot_two','` + now + `');`
	if _, err := old.Exec(schema); err != nil {
		t.Fatal(err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := database.Open(path)
	if err != nil {
		t.Fatalf("migrate version one: %v", err)
	}
	defer db.Close()
	roles := map[string]string{}
	rows, err := db.Query(`SELECT user_id,role FROM organisation_memberships ORDER BY user_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, role string
		if err := rows.Scan(&id, &role); err != nil {
			t.Fatal(err)
		}
		roles[id] = role
	}
	if roles["human"] != "admin" || roles["later_human"] != "member" || roles["bot"] != "member" {
		t.Fatalf("roles = %#v", roles)
	}
	var secondBotName string
	if err := db.QueryRow(`SELECT name FROM users WHERE id='bot_two'`).Scan(&secondBotName); err != nil {
		t.Fatal(err)
	}
	if secondBotName != "heCTOR 2" {
		t.Fatalf("migrated duplicate name = %q", secondBotName)
	}
}
