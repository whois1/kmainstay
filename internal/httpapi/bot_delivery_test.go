package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"kmainstay/internal/database"
)

func TestBotDelivery_LiveAndReplayUseDurableMentions(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "delivery.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	statements := []string{
		`INSERT INTO organisations(id,name,created_at) VALUES('org','Mainstay','` + now + `')`,
		`INSERT INTO users(id,kind,name,created_at) VALUES('human','bot','Human fixture','` + now + `')`,
		`INSERT INTO users(id,kind,name,created_at) VALUES('bot','bot','Hector','` + now + `')`,
		`INSERT INTO users(id,kind,name,created_at) VALUES('other_bot','bot','Other Bot','` + now + `')`,
		`INSERT INTO organisation_memberships(organisation_id,user_id,role,name_normalized,created_at) VALUES('org','human','member','human fixture','` + now + `')`,
		`INSERT INTO organisation_memberships(organisation_id,user_id,role,name_normalized,created_at) VALUES('org','bot','member','hector','` + now + `')`,
		`INSERT INTO organisation_memberships(organisation_id,user_id,role,name_normalized,created_at) VALUES('org','other_bot','member','other bot','` + now + `')`,
		`INSERT INTO conversations(id,organisation_id,name,visibility,created_at) VALUES('group','org','Group','organisation','` + now + `')`,
		`INSERT INTO conversations(id,organisation_id,name,visibility,created_at) VALUES('direct','org','Direct','members','` + now + `')`,
		`INSERT INTO conversations(id,organisation_id,name,visibility,created_at) VALUES('private_group','org','Private group','members','` + now + `')`,
		`INSERT INTO conversation_members(conversation_id,user_id) VALUES('direct','human'),('direct','bot')`,
		`INSERT INTO conversation_members(conversation_id,user_id) VALUES('private_group','human'),('private_group','bot'),('private_group','other_bot')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	// The fixture author represents a human principal even though it avoids password fields above.
	if _, err := db.Exec(`UPDATE users SET kind='human',email='human@example.com',password_hash=x'01' WHERE id='human'`); err != nil {
		t.Fatal(err)
	}

	server := &server{db: db}
	human := principal{ID: "human", Kind: "human", Name: "Human fixture"}
	bot := principal{ID: "bot", Kind: "bot", Name: "Hector"}
	otherBot := principal{ID: "other_bot", Kind: "bot", Name: "Other Bot"}

	unmentioned, _, err := server.insertMessage(context.Background(), "group", human, "hello everyone", "group-unmentioned")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.eventFor(context.Background(), unmentioned.Sequence, bot); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("group unmentioned bot delivery error = %v, want no rows", err)
	}
	mentioned, _, err := server.insertMessage(context.Background(), "group", human, "hello @hEcToR!", "group-mentioned")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := server.eventFor(context.Background(), mentioned.Sequence, bot); err != nil || got.ID != mentioned.ID {
		t.Fatalf("group mentioned delivery = %#v, %v", got, err)
	}
	if len(mentioned.Mentions) != 1 || mentioned.Mentions[0].ID != bot.ID {
		t.Fatalf("mentions = %#v", mentioned.Mentions)
	}
	var persisted int
	if err := db.QueryRow(`SELECT count(*) FROM message_mentions WHERE message_id=? AND user_id=?`, mentioned.ID, bot.ID).Scan(&persisted); err != nil || persisted != 1 {
		t.Fatalf("persisted mention count = %d, %v", persisted, err)
	}
	direct, _, err := server.insertMessage(context.Background(), "direct", human, "no mention needed", "direct")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.eventFor(context.Background(), direct.Sequence, bot); err != nil {
		t.Fatalf("direct bot delivery: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO conversation_members(conversation_id,user_id) VALUES('direct','other_bot')`); err != nil {
		t.Fatal(err)
	}
	if _, err := server.eventFor(context.Background(), direct.Sequence, bot); err != nil {
		t.Fatalf("persisted direct delivery after membership change: %v", err)
	}
	privateGroup, _, err := server.insertMessage(context.Background(), "private_group", human, "no mention in a larger private chat", "private-group")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.eventFor(context.Background(), privateGroup.Sequence, bot); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("private group unmentioned delivery error = %v, want no rows", err)
	}
	if _, err := db.Exec(`DELETE FROM conversation_members WHERE conversation_id='private_group' AND user_id='other_bot'`); err != nil {
		t.Fatal(err)
	}
	if _, err := server.eventFor(context.Background(), privateGroup.Sequence, bot); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("private group became retroactively deliverable: %v", err)
	}
	botAuthored, _, err := server.insertMessage(context.Background(), "group", bot, "@Other Bot do work", "bot-authored")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.eventFor(context.Background(), botAuthored.Sequence, otherBot); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("bot-authored delivery error = %v, want no rows", err)
	}
	if _, err := server.eventFor(context.Background(), unmentioned.Sequence, human); err != nil {
		t.Fatalf("human delivery: %v", err)
	}

	sequences, err := server.eligibleEventSequences(context.Background(), bot, 0)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int64{mentioned.Sequence, direct.Sequence}; !reflect.DeepEqual(sequences, want) {
		t.Fatalf("replay sequences = %v, want %v", sequences, want)
	}
}
