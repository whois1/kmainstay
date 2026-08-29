package database_test

import (
	"database/sql"
	"path/filepath"
	"strings"
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
	for _, table := range []string{"organisations", "users", "organisation_memberships", "conversations", "conversation_members", "conversation_read_positions", "conversation_archives", "messages", "message_mentions", "message_bot_deliveries", "attachments", "human_sessions", "api_keys", "realtime_events", "realtime_event_sequences", "message_update_events", "schema_migrations"} {
		var got string
		if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&got); err != nil {
			t.Errorf("missing table %s: %v", table, err)
		}
	}
	var version int
	if err := db.QueryRow(`SELECT max(version) FROM schema_migrations`).Scan(&version); err != nil || version != 11 {
		t.Fatalf("schema version = %d, err=%v", version, err)
	}
	var attachmentPositionColumns int
	if err := db.QueryRow(`SELECT count(*) FROM pragma_table_info('attachments') WHERE name='position'`).Scan(&attachmentPositionColumns); err != nil || attachmentPositionColumns != 1 {
		t.Fatalf("attachment position columns = %d, err=%v", attachmentPositionColumns, err)
	}
}

func TestOpen_MessageMentionsSurviveMentionedUserDeletion(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "mention-history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, statement := range []string{
		`INSERT INTO organisations(id,name,created_at) VALUES('org','Mainstay','` + now + `')`,
		`INSERT INTO users(id,kind,name,created_at) VALUES('author','bot','Author','` + now + `'),('mentioned','bot','Hector','` + now + `')`,
		`INSERT INTO conversations(id,organisation_id,name,visibility,created_at) VALUES('conversation','org','General','organisation','` + now + `')`,
		`INSERT INTO messages(id,conversation_id,author_id,body,created_at) VALUES('message','conversation','author','@Hector','` + now + `')`,
		`INSERT INTO message_mentions(message_id,user_id,name) VALUES('message','mentioned','Hector')`,
		`DELETE FROM users WHERE id='mentioned'`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	var name string
	if err := db.QueryRow(`SELECT name FROM message_mentions WHERE message_id='message'`).Scan(&name); err != nil || name != "Hector" {
		t.Fatalf("preserved mention name = %q, err=%v", name, err)
	}
}

func TestOpen_AttachmentsPermitImageOnlyMessagesAndCascade(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "attachments.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, statement := range []string{
		`INSERT INTO organisations(id,name,created_at) VALUES('org','Mainstay','` + now + `')`,
		`INSERT INTO users(id,kind,name,created_at) VALUES('author','bot','Author','` + now + `')`,
		`INSERT INTO conversations(id,organisation_id,name,visibility,created_at) VALUES('conversation','org','General','organisation','` + now + `')`,
		`INSERT INTO messages(id,conversation_id,author_id,body,created_at) VALUES('message','conversation','author','','` + now + `')`,
		`INSERT INTO attachments(id,message_id,storage_key,media_type,byte_size,width,height,original_filename,sha256,created_at) VALUES('attachment','message','0123456789abcdef','image/png',12,1,1,'image.png','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','` + now + `')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`DELETE FROM messages WHERE id='message'`); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM attachments`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("attachments after message deletion = %d, err=%v", count, err)
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

func TestOpen_MigrationFourBackfillsAccessibleConversationsToLatestSequence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "version-three.db")
	db, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	statements := []string{
		`INSERT INTO organisations(id,name,created_at) VALUES('org','Mainstay',?)`,
		`INSERT INTO users(id,kind,name,created_at) VALUES('reader','bot','Reader',?)`,
		`INSERT INTO users(id,kind,name,created_at) VALUES('author','bot','Author',?)`,
		`INSERT INTO organisation_memberships(organisation_id,user_id,role,name_normalized,created_at) VALUES('org','reader','member','reader',?)`,
		`INSERT INTO organisation_memberships(organisation_id,user_id,role,name_normalized,created_at) VALUES('org','author','member','author',?)`,
		`INSERT INTO conversations(id,organisation_id,name,visibility,created_at) VALUES('public','org','Public','organisation',?)`,
		`INSERT INTO conversations(id,organisation_id,name,visibility,created_at) VALUES('private','org','Private','members',?)`,
		`INSERT INTO messages(id,conversation_id,author_id,body,created_at) VALUES('message','public','author','hello',?)`,
		`INSERT INTO realtime_events(sequence,id,organisation_id,conversation_id,message_id,occurred_at) VALUES(1,'event','org','public','message',?)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`DROP TABLE conversation_read_positions; DELETE FROM schema_migrations WHERE version=4`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var sequence int64
	if err := db.QueryRow(`SELECT sequence FROM conversation_read_positions WHERE user_id='reader' AND conversation_id='public'`).Scan(&sequence); err != nil {
		t.Fatal(err)
	}
	if sequence != 1 {
		t.Fatalf("backfilled sequence = %d, want 1", sequence)
	}
	var inaccessible int
	if err := db.QueryRow(`SELECT count(*) FROM conversation_read_positions WHERE user_id='reader' AND conversation_id='private'`).Scan(&inaccessible); err != nil || inaccessible != 0 {
		t.Fatalf("inaccessible rows = %d, err=%v", inaccessible, err)
	}
}

func TestOpen_ConversationReadPositionsEnforceSequenceAndCascade(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "read-constraints.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, statement := range []string{
		`INSERT INTO organisations(id,name,created_at) VALUES('org','Mainstay','` + now + `')`,
		`INSERT INTO users(id,kind,name,created_at) VALUES('reader','bot','Reader','` + now + `')`,
		`INSERT INTO conversations(id,organisation_id,name,visibility,created_at) VALUES('conversation','org','General','organisation','` + now + `')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO conversation_read_positions(user_id,conversation_id,sequence,updated_at) VALUES('reader','conversation',-1,?)`, now); err == nil {
		t.Fatal("negative sequence was accepted")
	}
	if _, err := db.Exec(`INSERT INTO conversation_read_positions(user_id,conversation_id,sequence,updated_at) VALUES('reader','conversation',0,?)`, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM conversations WHERE id='conversation'`); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM conversation_read_positions`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rows after cascade = %d, err=%v", count, err)
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
	if got := database.NormalizeName("\u2003E\u0301LODIE\u00a0"); got != "élodie" {
		t.Fatalf("normalised name = %q", got)
	}
}

func TestOpen_MigratesVersionTwoCanonicalNameCollisions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "version-two.db")
	old, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	schema := `
		CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,applied_at TEXT NOT NULL);
		INSERT INTO schema_migrations(version,applied_at) VALUES(1,'2026-01-01T00:00:00Z'),(2,'2026-01-02T00:00:00Z');
		CREATE TABLE organisations(id TEXT PRIMARY KEY,name TEXT NOT NULL,created_at TEXT NOT NULL);
		CREATE TABLE users(id TEXT PRIMARY KEY,kind TEXT NOT NULL,email TEXT,name TEXT NOT NULL,password_hash TEXT,created_at TEXT NOT NULL);
		CREATE TABLE organisation_memberships(organisation_id TEXT NOT NULL REFERENCES organisations(id),user_id TEXT NOT NULL REFERENCES users(id),role TEXT NOT NULL,name_normalized TEXT NOT NULL,created_at TEXT NOT NULL,PRIMARY KEY(organisation_id,user_id),UNIQUE(organisation_id,name_normalized));
		CREATE TABLE conversations(id TEXT PRIMARY KEY,organisation_id TEXT NOT NULL REFERENCES organisations(id),visibility TEXT NOT NULL);
		CREATE TABLE conversation_members(conversation_id TEXT NOT NULL,user_id TEXT NOT NULL,PRIMARY KEY(conversation_id,user_id));
		CREATE TABLE realtime_events(sequence INTEGER PRIMARY KEY AUTOINCREMENT,conversation_id TEXT NOT NULL);
		INSERT INTO organisations(id,name,created_at) VALUES('org','Mainstay','` + now + `');
		INSERT INTO users(id,kind,name,created_at) VALUES('first','bot','Élodie','` + now + `'),('second','bot','ÉLODIE','` + now + `'),('orphan','bot','ÉCHO','` + now + `');
		INSERT INTO organisation_memberships(organisation_id,user_id,role,name_normalized,created_at) VALUES('org','first','admin','élodie','` + now + `'),('org','second','member','élodie','` + now + `');`
	if _, err := old.Exec(schema); err != nil {
		t.Fatal(err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := database.Open(path)
	if err != nil {
		t.Fatalf("migrate version two: %v", err)
	}
	defer db.Close()
	var version int
	if err := db.QueryRow(`SELECT max(version) FROM schema_migrations`).Scan(&version); err != nil || version != 11 {
		t.Fatalf("schema version = %d, err=%v", version, err)
	}
	var first, second, firstNormalized, secondNormalized, firstRole, secondRole, firstCreatedAt, secondCreatedAt string
	if err := db.QueryRow(`SELECT u.name,m.name_normalized,m.role,m.created_at FROM users u JOIN organisation_memberships m ON m.user_id=u.id WHERE u.id='first'`).Scan(&first, &firstNormalized, &firstRole, &firstCreatedAt); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT u.name,m.name_normalized,m.role,m.created_at FROM users u JOIN organisation_memberships m ON m.user_id=u.id WHERE u.id='second'`).Scan(&second, &secondNormalized, &secondRole, &secondCreatedAt); err != nil {
		t.Fatal(err)
	}
	if first != "Élodie" || second != "ÉLODIE 2" || firstNormalized != "élodie" || secondNormalized != "élodie 2" || firstRole != "admin" || secondRole != "member" || firstCreatedAt != now || secondCreatedAt != now {
		t.Fatalf("migrated memberships = names %q/%q normalized=%q/%q roles=%q/%q created=%q/%q", first, second, firstNormalized, secondNormalized, firstRole, secondRole, firstCreatedAt, secondCreatedAt)
	}
	var orphan string
	if err := db.QueryRow(`SELECT name FROM users WHERE id='orphan'`).Scan(&orphan); err != nil || orphan != "ÉCHO" {
		t.Fatalf("orphan name = %q, err=%v", orphan, err)
	}
	foreignKeys, err := db.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	defer foreignKeys.Close()
	if foreignKeys.Next() {
		t.Fatal("migration left a foreign-key violation")
	}
}

func TestOpen_MessageAttachmentMigrationPreservesExistingMessagingData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "version-five.db")
	db, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`INSERT INTO organisations(id,name,created_at) VALUES('org','Org','2026-01-01T00:00:00Z')`,
		`INSERT INTO users(id,kind,email,name,password_hash,created_at) VALUES('human','human','human@example.com','Human','hash','2026-01-01T00:00:00Z')`,
		`INSERT INTO users(id,kind,name,created_at) VALUES('bot','bot','Bot','2026-01-01T00:00:00Z')`,
		`INSERT INTO organisation_memberships(organisation_id,user_id,role,name_normalized,created_at) VALUES('org','human','admin','human','2026-01-01T00:00:00Z'),('org','bot','member','bot','2026-01-01T00:00:00Z')`,
		`INSERT INTO conversations(id,organisation_id,name,visibility,created_at) VALUES('conversation','org','general','organisation','2026-01-01T00:00:00Z')`,
		`INSERT INTO messages(id,conversation_id,author_id,body,client_id,created_at) VALUES('message','conversation','human','hello','client','2026-01-01T00:00:00Z')`,
		`INSERT INTO realtime_event_sequences(sequence) VALUES(27)`,
		`INSERT INTO realtime_events(sequence,id,organisation_id,conversation_id,message_id,occurred_at) VALUES(27,'event','org','conversation','message','2026-01-01T00:00:00Z')`,
		`INSERT INTO message_mentions(message_id,user_id,name) VALUES('message','bot','Bot')`,
		`INSERT INTO message_bot_deliveries(message_id,user_id) VALUES('message','bot')`,
		`DROP TABLE attachments`,
		`DELETE FROM schema_migrations WHERE version=6`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			t.Fatalf("prepare version five database: %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var body, mentionName, deliveryUser string
	var sequence int64
	if err := db.QueryRow(`SELECT body FROM messages WHERE id='message'`).Scan(&body); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT sequence FROM realtime_events WHERE id='event'`).Scan(&sequence); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT name FROM message_mentions WHERE message_id='message'`).Scan(&mentionName); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT user_id FROM message_bot_deliveries WHERE message_id='message'`).Scan(&deliveryUser); err != nil {
		t.Fatal(err)
	}
	if body != "hello" || sequence != 27 || mentionName != "Bot" || deliveryUser != "bot" {
		t.Fatalf("body=%q sequence=%d mention=%q delivery=%q", body, sequence, mentionName, deliveryUser)
	}
	if _, err := db.Exec(`INSERT INTO messages(id,conversation_id,author_id,body,created_at) VALUES('next-message','conversation','human','next','2026-01-01T00:00:01Z')`); err != nil {
		t.Fatal(err)
	}
	result, err := db.Exec(`INSERT INTO realtime_event_sequences(sequence) VALUES(NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	nextSequence, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO realtime_events(sequence,id,organisation_id,conversation_id,message_id,occurred_at) VALUES(?,'next-event','org','conversation','next-message','2026-01-01T00:00:01Z')`, nextSequence)
	if err != nil || nextSequence != 28 {
		t.Fatalf("next sequence = %d, err=%v", nextSequence, err)
	}
}

func TestOpen_MessageEditEventsRejectOldBinaryCreationWrites(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "message-edit-events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, statement := range []string{
		`INSERT INTO organisations(id,name,created_at) VALUES('org','Org','2026-01-01T00:00:00Z')`,
		`INSERT INTO users(id,kind,name,created_at) VALUES('author','bot','Author','2026-01-01T00:00:00Z')`,
		`INSERT INTO conversations(id,organisation_id,name,visibility,created_at) VALUES('conversation','org','general','organisation','2026-01-01T00:00:00Z')`,
		`INSERT INTO messages(id,conversation_id,author_id,body,created_at) VALUES('message','conversation','author','message','2026-01-01T00:00:00Z')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	_, err = db.Exec(`INSERT INTO realtime_events(id,organisation_id,conversation_id,message_id,occurred_at) VALUES('event','org','conversation','message','2026-01-01T00:00:00Z')`)
	if err == nil || !strings.Contains(err.Error(), "realtime_events.sequence is required by the current schema; use the current binary") {
		t.Fatalf("old-binary insert error = %v", err)
	}
	var eventCount, sequenceCount int
	if err := db.QueryRow(`SELECT count(*) FROM realtime_events WHERE id='event'`).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM realtime_event_sequences`).Scan(&sequenceCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 0 || sequenceCount != 0 {
		t.Fatalf("rejected insert wrote event=%d allocator sequences=%d", eventCount, sequenceCount)
	}
}

func TestOpen_MessageEditMigrationPreservesDeletedEventSequenceHighWater(t *testing.T) {
	path := filepath.Join(t.TempDir(), "version-ten-high-water.db")
	db, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`DROP TRIGGER realtime_events_require_sequence`,
		`DROP TABLE message_update_events`,
		`DROP TABLE realtime_event_sequences`,
		`ALTER TABLE realtime_events DROP COLUMN creation_payload`,
		`ALTER TABLE messages DROP COLUMN edited_at`,
		`DELETE FROM schema_migrations WHERE version=11`,
		`INSERT INTO organisations(id,name,created_at) VALUES('org','Org','2026-01-01T00:00:00Z')`,
		`INSERT INTO users(id,kind,name,created_at) VALUES('author','bot','Author','2026-01-01T00:00:00Z')`,
		`INSERT INTO conversations(id,organisation_id,name,visibility,created_at) VALUES('conversation','org','general','organisation','2026-01-01T00:00:00Z')`,
		`INSERT INTO messages(id,conversation_id,author_id,body,created_at) VALUES('survivor','conversation','author','survivor','2026-01-01T00:00:00Z'),('deleted','conversation','author','deleted','2026-01-01T00:00:01Z')`,
		`INSERT INTO realtime_events(sequence,id,organisation_id,conversation_id,message_id,occurred_at) VALUES(1,'survivor-event','org','conversation','survivor','2026-01-01T00:00:00Z'),(100,'deleted-event','org','conversation','deleted','2026-01-01T00:00:01Z')`,
		`DELETE FROM messages WHERE id='deleted'`,
	} {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			t.Fatalf("prepare version ten database with deleted high-water event: %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = database.Open(path)
	if err != nil {
		t.Fatalf("upgrade version ten database: %v", err)
	}
	defer db.Close()
	result, err := db.Exec(`INSERT INTO realtime_event_sequences(sequence) VALUES(NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	nextSequence, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if nextSequence != 101 {
		t.Fatalf("next sequence = %d, want 101", nextSequence)
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
		CREATE TABLE conversations(id TEXT PRIMARY KEY,organisation_id TEXT NOT NULL REFERENCES organisations(id),visibility TEXT NOT NULL);
		CREATE TABLE conversation_members(conversation_id TEXT NOT NULL,user_id TEXT NOT NULL,PRIMARY KEY(conversation_id,user_id));
		CREATE TABLE realtime_events(sequence INTEGER PRIMARY KEY AUTOINCREMENT,conversation_id TEXT NOT NULL);
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
