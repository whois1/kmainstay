package httpapi_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"kmainstay/internal/app"
	"kmainstay/internal/database"
	"kmainstay/internal/httpapi"
)

func TestConversations_IncludeDefaultReadSequence(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "read-position.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	boot, err := app.Bootstrap(context.Background(), db, "reader@example.com", "Reader", "correct horse battery staple", "Mainstay")
	if err != nil {
		t.Fatal(err)
	}
	token := "read-position-session"
	digest := sha256.Sum256([]byte(token))
	if _, err := db.Exec(`INSERT INTO human_sessions(token_hash,user_id,expires_at,created_at) VALUES(?,?,?,?)`, digest[:], boot.UserID, time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	handler := httpapi.New(httpapi.Dependencies{DB: db})
	request := httptest.NewRequest(http.MethodGet, "/api/organisations/"+boot.OrganisationID+"/conversations", nil)
	request.AddCookie(&http.Cookie{Name: "kmainstay_session", Value: token})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	var conversations []map[string]any
	response := recorder.Result()
	decodeResponse(t, response, http.StatusOK, &conversations)
	if len(conversations) != 1 || conversations[0]["id"] != boot.ConversationID || conversations[0]["read_sequence"] != float64(0) || conversations[0]["latest_sequence"] != float64(0) {
		t.Fatalf("conversations = %#v", conversations)
	}
}

func TestDirectConversation_IsIdempotentAndValidatesMembers(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "direct-conversation.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	boot, err := app.Bootstrap(context.Background(), db, "owner@example.com", "Owner", "correct horse battery staple", "Mainstay")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(httpapi.New(httpapi.Dependencies{DB: db}))
	defer server.Close()
	client := newCookieClient(t)
	decodeResponse(t, requestJSON(t, client, http.MethodPost, server.URL+"/api/session", server.URL, "", map[string]any{"email": "owner@example.com", "password": "correct horse battery staple"}), http.StatusOK, &map[string]any{})

	var hector, mary map[string]any
	decodeResponse(t, requestJSON(t, client, http.MethodPost, server.URL+"/api/organisations/"+boot.OrganisationID+"/bots", server.URL, "", map[string]any{"name": "Hector"}), http.StatusCreated, &hector)
	decodeResponse(t, requestJSON(t, client, http.MethodPost, server.URL+"/api/organisations/"+boot.OrganisationID+"/bots", server.URL, "", map[string]any{"name": "Mary"}), http.StatusCreated, &mary)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO conversations(id,organisation_id,name,visibility,created_at) VALUES('legacy-direct',?,'Old Hector title','members',?)`, boot.OrganisationID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO conversation_members(conversation_id,user_id) VALUES('legacy-direct',?),('legacy-direct',?)`, boot.UserID, hector["id"]); err != nil {
		t.Fatal(err)
	}
	endpoint := server.URL + "/api/organisations/" + boot.OrganisationID + "/direct-conversations/" + hector["id"].(string)
	for range 2 {
		var conversation map[string]any
		decodeResponse(t, requestJSON(t, client, http.MethodPost, endpoint, server.URL, "", nil), http.StatusOK, &conversation)
		if conversation["id"] != "legacy-direct" || conversation["name"] != "Old Hector title" {
			t.Fatalf("legacy conversation = %#v", conversation)
		}
	}

	firstUserID, secondUserID := boot.UserID, mary["id"].(string)
	if firstUserID > secondUserID {
		firstUserID, secondUserID = secondUserID, firstUserID
	}
	oldReservedPairName := "direct:" + firstUserID + ":" + secondUserID
	decodeResponse(t, requestJSON(t, client, http.MethodPost, server.URL+"/api/organisations/"+boot.OrganisationID+"/conversations", server.URL, "", map[string]any{"name": oldReservedPairName, "visibility": "members", "member_ids": []string{hector["id"].(string), mary["id"].(string)}}), http.StatusCreated, &map[string]any{})
	maryEndpoint := server.URL + "/api/organisations/" + boot.OrganisationID + "/direct-conversations/" + mary["id"].(string)
	responses := make(chan *http.Response, 2)
	for range 2 {
		go func() { responses <- requestJSON(t, client, http.MethodPost, maryEndpoint, server.URL, "", nil) }()
	}
	var createdIDs []string
	for range 2 {
		var conversation map[string]any
		decodeResponse(t, <-responses, http.StatusOK, &conversation)
		createdIDs = append(createdIDs, conversation["id"].(string))
	}
	if createdIDs[0] != createdIDs[1] {
		t.Fatalf("concurrent calls returned %v", createdIDs)
	}
	var exactPairCount int
	if err := db.QueryRow(`SELECT count(*) FROM conversations c WHERE c.organisation_id=? AND c.visibility='members' AND (SELECT count(*) FROM conversation_members cm WHERE cm.conversation_id=c.id)=2 AND EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=c.id AND cm.user_id=?) AND EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=c.id AND cm.user_id=?)`, boot.OrganisationID, boot.UserID, mary["id"]).Scan(&exactPairCount); err != nil || exactPairCount != 1 {
		t.Fatalf("exact pair count = %d, err = %v", exactPairCount, err)
	}

	for _, target := range []string{boot.UserID, "missing-user"} {
		response := requestJSON(t, client, http.MethodPost, server.URL+"/api/organisations/"+boot.OrganisationID+"/direct-conversations/"+target, server.URL, "", nil)
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("target %q status = %d, want 400: %s", target, response.StatusCode, readBody(response))
		}
		response.Body.Close()
	}
}

func TestDirectConversation_RollsBackWhenMembershipDisappearsDuringCreation(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "direct-conversation-membership-race.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	boot, err := app.Bootstrap(context.Background(), db, "owner@example.com", "Owner", "correct horse battery staple", "Mainstay")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(httpapi.New(httpapi.Dependencies{DB: db}))
	defer server.Close()
	client := newCookieClient(t)
	decodeResponse(t, requestJSON(t, client, http.MethodPost, server.URL+"/api/session", server.URL, "", map[string]any{"email": "owner@example.com", "password": "correct horse battery staple"}), http.StatusOK, &map[string]any{})

	var bot map[string]any
	decodeResponse(t, requestJSON(t, client, http.MethodPost, server.URL+"/api/organisations/"+boot.OrganisationID+"/bots", server.URL, "", map[string]any{"name": "Hector"}), http.StatusCreated, &bot)
	botID := bot["id"].(string)
	if _, err := db.Exec(fmt.Sprintf(`CREATE TEMP TRIGGER remove_direct_conversation_target
		AFTER INSERT ON conversations
		WHEN NEW.visibility='members' AND NEW.name LIKE 'direct:%%'
		BEGIN
			DELETE FROM organisation_memberships WHERE organisation_id=NEW.organisation_id AND user_id=%q;
		END`, botID)); err != nil {
		t.Fatal(err)
	}

	response := requestJSON(t, client, http.MethodPost, server.URL+"/api/organisations/"+boot.OrganisationID+"/direct-conversations/"+botID, server.URL, "", nil)
	if response.StatusCode < 400 || response.StatusCode >= 500 {
		t.Fatalf("status = %d, want non-500 client error: %s", response.StatusCode, readBody(response))
	}
	response.Body.Close()

	var conversationCount, conversationMemberCount, membershipCount int
	if err := db.QueryRow(`SELECT count(*) FROM conversations WHERE organisation_id=? AND name LIKE 'direct:%'`, boot.OrganisationID).Scan(&conversationCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM conversation_members WHERE conversation_id IN (SELECT id FROM conversations WHERE organisation_id=? AND name LIKE 'direct:%')`, boot.OrganisationID).Scan(&conversationMemberCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM organisation_memberships WHERE organisation_id=? AND user_id=?`, boot.OrganisationID, botID).Scan(&membershipCount); err != nil {
		t.Fatal(err)
	}
	if conversationCount != 0 || conversationMemberCount != 0 || membershipCount != 1 {
		t.Fatalf("conversation count = %d, member count = %d, membership count = %d; want 0, 0, 1", conversationCount, conversationMemberCount, membershipCount)
	}
}

func TestPutConversationRead_PersistsValidSequence(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "mark-read.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	boot, err := app.Bootstrap(context.Background(), db, "reader@example.com", "Reader", "correct horse battery staple", "Mainstay")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO messages(id,conversation_id,author_id,body,created_at) VALUES('message',?,?,?,?)`, boot.ConversationID, boot.UserID, "hello", now); err != nil {
		t.Fatal(err)
	}
	result, err := db.Exec(`INSERT INTO realtime_events(id,organisation_id,conversation_id,message_id,occurred_at) VALUES('event',?,?,?,?)`, boot.OrganisationID, boot.ConversationID, "message", now)
	if err != nil {
		t.Fatal(err)
	}
	sequence, _ := result.LastInsertId()
	token := "mark-read-session"
	digest := sha256.Sum256([]byte(token))
	if _, err := db.Exec(`INSERT INTO human_sessions(token_hash,user_id,expires_at,created_at) VALUES(?,?,?,?)`, digest[:], boot.UserID, time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano), now); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"sequence": sequence})
	request := httptest.NewRequest(http.MethodPut, "http://example.com/api/conversations/"+boot.ConversationID+"/read", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://example.com")
	request.AddCookie(&http.Cookie{Name: "kmainstay_session", Value: token})
	recorder := httptest.NewRecorder()
	httpapi.New(httpapi.Dependencies{DB: db}).ServeHTTP(recorder, request)
	var response map[string]any
	decodeResponse(t, recorder.Result(), http.StatusOK, &response)
	if response["sequence"] != float64(sequence) {
		t.Fatalf("response = %#v", response)
	}
}

func TestPostMessage_AdvancesAuthorReadPosition(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "author-read.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	boot, err := app.Bootstrap(context.Background(), db, "author@example.com", "Author", "correct horse battery staple", "Mainstay")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	token := "author-read-session"
	digest := sha256.Sum256([]byte(token))
	if _, err := db.Exec(`INSERT INTO human_sessions(token_hash,user_id,expires_at,created_at) VALUES(?,?,?,?)`, digest[:], boot.UserID, time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano), now); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"body": "hello", "client_id": "author-read"})
	request := httptest.NewRequest(http.MethodPost, "http://example.com/api/conversations/"+boot.ConversationID+"/messages", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://example.com")
	request.AddCookie(&http.Cookie{Name: "kmainstay_session", Value: token})
	recorder := httptest.NewRecorder()
	httpapi.New(httpapi.Dependencies{DB: db}).ServeHTTP(recorder, request)
	var message map[string]any
	decodeResponse(t, recorder.Result(), http.StatusCreated, &message)
	var readSequence int64
	if err := db.QueryRow(`SELECT sequence FROM conversation_read_positions WHERE user_id=? AND conversation_id=?`, boot.UserID, boot.ConversationID).Scan(&readSequence); err != nil {
		t.Fatal(err)
	}
	if readSequence != int64(message["sequence"].(float64)) {
		t.Fatalf("read sequence = %d, message = %#v", readSequence, message)
	}
}

func TestPutConversationRead_DoesNotMoveBackwards(t *testing.T) {
	fixture := newReadPositionFixture(t)
	defer fixture.db.Close()
	decodeResponse(t, fixture.put(t, fixture.conversationID, fixture.sequence), http.StatusOK, &map[string]any{})
	var response map[string]any
	decodeResponse(t, fixture.put(t, fixture.conversationID, 0), http.StatusOK, &response)
	if response["sequence"] != float64(fixture.sequence) {
		t.Fatalf("response = %#v", response)
	}
}

func TestPutConversationRead_RequiresSequence(t *testing.T) {
	fixture := newReadPositionFixture(t)
	defer fixture.db.Close()
	request := httptest.NewRequest(http.MethodPut, "http://example.com/api/conversations/"+fixture.conversationID+"/read", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://example.com")
	request.AddCookie(&http.Cookie{Name: "kmainstay_session", Value: fixture.token})
	recorder := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", recorder.Code, recorder.Body.String())
	}
}

func TestPutConversationRead_RejectsSequenceFromAnotherConversation(t *testing.T) {
	fixture := newReadPositionFixture(t)
	defer fixture.db.Close()
	response := fixture.put(t, fixture.conversationID, fixture.otherSequence)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", response.StatusCode, readBody(response))
	}
}

func TestPutConversationRead_DeniesInaccessibleConversation(t *testing.T) {
	fixture := newReadPositionFixture(t)
	defer fixture.db.Close()
	response := fixture.put(t, fixture.inaccessibleConversationID, fixture.inaccessibleSequence)
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", response.StatusCode, readBody(response))
	}
}

type readPositionFixture struct {
	db                         *sql.DB
	handler                    http.Handler
	token                      string
	conversationID             string
	inaccessibleConversationID string
	sequence                   int64
	otherSequence              int64
	inaccessibleSequence       int64
}

func newReadPositionFixture(t *testing.T) readPositionFixture {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "read-fixture.db"))
	if err != nil {
		t.Fatal(err)
	}
	boot, err := app.Bootstrap(context.Background(), db, "reader@example.com", "Reader", "correct horse battery staple", "Mainstay")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO conversations(id,organisation_id,name,visibility,created_at) VALUES('other',?,'Other','organisation',?),('private',?,'Private','members',?)`, boot.OrganisationID, now, boot.OrganisationID, now); err != nil {
		t.Fatal(err)
	}
	conversationIDs := []string{boot.ConversationID, "other", "private"}
	sequences := make([]int64, 0, len(conversationIDs))
	for index, conversationID := range conversationIDs {
		messageID := fmt.Sprintf("fixture-message-%d", index)
		if _, err := db.Exec(`INSERT INTO messages(id,conversation_id,author_id,body,created_at) VALUES(?,?,?,'message',?)`, messageID, conversationID, boot.UserID, now); err != nil {
			t.Fatal(err)
		}
		result, err := db.Exec(`INSERT INTO realtime_events(id,organisation_id,conversation_id,message_id,occurred_at) VALUES(?,?,?,?,?)`, fmt.Sprintf("fixture-event-%d", index), boot.OrganisationID, conversationID, messageID, now)
		if err != nil {
			t.Fatal(err)
		}
		sequence, _ := result.LastInsertId()
		sequences = append(sequences, sequence)
	}
	token := "read-fixture-session"
	digest := sha256.Sum256([]byte(token))
	if _, err := db.Exec(`INSERT INTO human_sessions(token_hash,user_id,expires_at,created_at) VALUES(?,?,?,?)`, digest[:], boot.UserID, time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano), now); err != nil {
		t.Fatal(err)
	}
	return readPositionFixture{db: db, handler: httpapi.New(httpapi.Dependencies{DB: db}), token: token, conversationID: boot.ConversationID, inaccessibleConversationID: "private", sequence: sequences[0], otherSequence: sequences[1], inaccessibleSequence: sequences[2]}
}

func (fixture readPositionFixture) put(t *testing.T, conversationID string, sequence int64) *http.Response {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"sequence": sequence})
	request := httptest.NewRequest(http.MethodPut, "http://example.com/api/conversations/"+conversationID+"/read", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://example.com")
	request.AddCookie(&http.Cookie{Name: "kmainstay_session", Value: fixture.token})
	recorder := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recorder, request)
	return recorder.Result()
}

func TestBDD_MichaelAndHectorExchangePersistentRealtimeMessagesAndRevocationSurvivesRestart(t *testing.T) {
	// Given Michael, his organisation, and general were bootstrapped.
	databasePath := filepath.Join(t.TempDir(), "kmainstay.db")
	db, err := database.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	boot, err := app.Bootstrap(context.Background(), db, "michael@example.com", "Michael", "correct horse battery staple", "Mainstay")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(httpapi.New(httpapi.Dependencies{DB: db}))

	// When Michael logs in and creates Hector, the API key is returned once.
	client := newCookieClient(t)
	login := requestJSON(t, client, http.MethodPost, server.URL+"/api/session", server.URL, "", map[string]any{"email": "michael@example.com", "password": "correct horse battery staple"})
	if login.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d: %s", login.StatusCode, readBody(login))
	}
	me := requestJSON(t, client, http.MethodGet, server.URL+"/api/me", "", "", nil)
	var meBody map[string]any
	decodeResponse(t, me, http.StatusOK, &meBody)
	if meBody["kind"] != "human" {
		t.Fatalf("me = %#v", meBody)
	}
	botResponse := requestJSON(t, client, http.MethodPost, server.URL+"/api/organisations/"+boot.OrganisationID+"/bots", server.URL, "", map[string]any{"name": "Hector"})
	var bot struct {
		ID     string `json:"id"`
		APIKey string `json:"api_key"`
	}
	decodeResponse(t, botResponse, http.StatusCreated, &bot)
	if !strings.HasPrefix(bot.APIKey, "km_live_") {
		t.Fatalf("api key = %q", bot.APIKey)
	}
	botConversation := requestJSON(t, http.DefaultClient, http.MethodPost, server.URL+"/api/organisations/"+boot.OrganisationID+"/conversations", "", bot.APIKey, map[string]any{"name": "bot-created", "visibility": "organisation", "member_ids": []string{}})
	if botConversation.StatusCode != http.StatusCreated {
		t.Fatalf("bot-created conversation status = %d, want 201: %s", botConversation.StatusCode, readBody(botConversation))
	}
	malformedBot := requestJSON(t, client, http.MethodPost, server.URL+"/api/organisations/"+boot.OrganisationID+"/bots", server.URL, "", map[string]any{"name": "   "})
	if malformedBot.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed bot status = %d, want 400: %s", malformedBot.StatusCode, readBody(malformedBot))
	}
	privateResponse := requestJSON(t, client, http.MethodPost, server.URL+"/api/organisations/"+boot.OrganisationID+"/conversations", server.URL, "", map[string]any{"name": "humans-only", "visibility": "members"})
	var privateConversation map[string]any
	decodeResponse(t, privateResponse, http.StatusCreated, &privateConversation)
	privateDenied := requestJSON(t, http.DefaultClient, http.MethodGet, server.URL+"/api/conversations/"+privateConversation["id"].(string)+"/messages", "", bot.APIKey, nil)
	if privateDenied.StatusCode != http.StatusForbidden {
		t.Fatalf("private conversation status = %d: %s", privateDenied.StatusCode, readBody(privateDenied))
	}
	var leaked int
	if err := db.QueryRow(`SELECT count(*) FROM api_keys WHERE CAST(secret_hash AS TEXT) LIKE '%' || ? || '%'`, bot.APIKey).Scan(&leaked); err != nil || leaked != 0 {
		t.Fatalf("copy-once key persisted: count=%d err=%v", leaked, err)
	}

	// Given both principals have authenticated realtime connections.
	botSocket := dialSocket(t, server.URL+"/api/ws?after=0", http.Header{"Authorization": {"Bearer " + bot.APIKey}})
	defer botSocket.CloseNow()
	var sessionCookie *http.Cookie
	serverURL, _ := url.Parse(server.URL)
	for _, cookie := range client.Jar.Cookies(serverURL) {
		if cookie.Name == "kmainstay_session" {
			sessionCookie = cookie
		}
	}
	if sessionCookie == nil {
		t.Fatal("session cookie missing")
	}
	browserSocket := dialSocket(t, server.URL+"/api/ws?after=0", http.Header{"Cookie": {sessionCookie.String()}})
	defer browserSocket.CloseNow()

	// When the human posts, the bot sees the persisted event.
	humanPost := requestJSON(t, client, http.MethodPost, server.URL+"/api/conversations/"+boot.ConversationID+"/messages", server.URL, "", map[string]any{"body": "  Hello @Hector **please**  ", "client_id": "browser-1"})
	decodeResponse(t, humanPost, http.StatusCreated, &map[string]any{})
	botEvent := readEvent(t, botSocket)
	if bodyOf(botEvent) != "  Hello @Hector **please**  " {
		t.Fatalf("bot event = %#v", botEvent)
	}
	humanSequence := int64(botEvent["sequence"].(float64))
	_ = readEvent(t, browserSocket) // the browser also receives Michael's own durable event

	// When Hector replies using the same principal model, the browser sees it live.
	botPost := requestJSON(t, http.DefaultClient, http.MethodPost, server.URL+"/api/conversations/"+boot.ConversationID+"/messages", "", bot.APIKey, map[string]any{"body": "Hello Michael", "client_id": "hector-1"})
	decodeResponse(t, botPost, http.StatusCreated, &map[string]any{})
	browserEvent := readEvent(t, browserSocket)
	if bodyOf(browserEvent) != "Hello Michael" {
		t.Fatalf("browser event = %#v", browserEvent)
	}
	botSequence := int64(browserEvent["sequence"].(float64))
	if botSequence <= humanSequence {
		t.Fatalf("bot sequence = %d, want greater than human sequence %d", botSequence, humanSequence)
	}

	// Cookie-authenticated mutations without a same-origin signal are rejected.
	csrfDenied := requestJSON(t, client, http.MethodPost, server.URL+"/api/conversations/"+boot.ConversationID+"/messages", "", "", map[string]any{"body": "forged", "client_id": "forged-1"})
	if csrfDenied.StatusCode != http.StatusForbidden {
		t.Fatalf("CSRF status = %d: %s", csrfDenied.StatusCode, readBody(csrfDenied))
	}

	// And retrying Hector's client ID is idempotent.
	retry := requestJSON(t, http.DefaultClient, http.MethodPost, server.URL+"/api/conversations/"+boot.ConversationID+"/messages", "", bot.APIKey, map[string]any{"body": "ignored retry body", "client_id": "hector-1"})
	decodeResponse(t, retry, http.StatusOK, &map[string]any{})
	secondConversationResponse := requestJSON(t, client, http.MethodPost, server.URL+"/api/organisations/"+boot.OrganisationID+"/conversations", server.URL, "", map[string]any{"name": "second", "visibility": "organisation"})
	var secondConversation map[string]any
	decodeResponse(t, secondConversationResponse, http.StatusCreated, &secondConversation)
	sameClientIDElsewhere := requestJSON(t, http.DefaultClient, http.MethodPost, server.URL+"/api/conversations/"+secondConversation["id"].(string)+"/messages", "", bot.APIKey, map[string]any{"body": "different conversation", "client_id": "hector-1"})
	var secondMessage map[string]any
	decodeResponse(t, sameClientIDElsewhere, http.StatusCreated, &secondMessage)
	if secondMessage["conversation_id"] != secondConversation["id"] {
		t.Fatalf("idempotency returned message from %v, want %v", secondMessage["conversation_id"], secondConversation["id"])
	}

	// When the process restarts, sessions, messages, and key revocation remain durable.
	server.Close()
	db.Close()
	db, err = database.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server = httptest.NewServer(httpapi.New(httpapi.Dependencies{DB: db}))
	defer server.Close()
	var replayable int
	if err := db.QueryRow(`SELECT count(*) FROM realtime_events WHERE conversation_id=?`, boot.ConversationID).Scan(&replayable); err != nil || replayable != 2 {
		t.Fatalf("durable events = %d, want 2: %v", replayable, err)
	}
	replaySocket := dialSocket(t, server.URL+"/api/ws?after=0", http.Header{"Authorization": {"Bearer " + bot.APIKey}})
	replayed := readEvent(t, replaySocket)
	replaySocket.CloseNow()
	if bodyOf(replayed) != "  Hello @Hector **please**  " {
		t.Fatalf("replayed event = %#v", replayed)
	}
	messagesResponse := requestJSON(t, client, http.MethodGet, server.URL+"/api/conversations/"+boot.ConversationID+"/messages", "", "", nil)
	var messages []map[string]any
	decodeResponse(t, messagesResponse, http.StatusOK, &messages)
	if len(messages) != 2 || messages[0]["body"] != "  Hello @Hector **please**  " || messages[1]["body"] != "Hello Michael" {
		t.Fatalf("persisted messages = %#v", messages)
	}
	latestResponse := requestJSON(t, client, http.MethodGet, server.URL+"/api/conversations/"+boot.ConversationID+"/messages?limit=1", "", "", nil)
	var latest []map[string]any
	decodeResponse(t, latestResponse, http.StatusOK, &latest)
	if len(latest) != 1 || latest[0]["body"] != "Hello Michael" {
		t.Fatalf("latest page = %#v", latest)
	}
	before := url.QueryEscape(latest[0]["id"].(string))
	previousResponse := requestJSON(t, client, http.MethodGet, server.URL+"/api/conversations/"+boot.ConversationID+"/messages?limit=1&before="+before, "", "", nil)
	var previous []map[string]any
	decodeResponse(t, previousResponse, http.StatusOK, &previous)
	if len(previous) != 1 || previous[0]["body"] != "  Hello @Hector **please**  " {
		t.Fatalf("previous page = %#v", previous)
	}
	unreadResponse := requestJSON(t, client, http.MethodGet, server.URL+fmt.Sprintf("/api/conversations/%s/messages?limit=100&after_sequence=%d", boot.ConversationID, humanSequence), "", "", nil)
	var unread []map[string]any
	decodeResponse(t, unreadResponse, http.StatusOK, &unread)
	if len(unread) != 1 || unread[0]["body"] != "Hello Michael" {
		t.Fatalf("messages after sequence = %#v", unread)
	}
	badCursor := requestJSON(t, client, http.MethodGet, server.URL+"/api/conversations/"+boot.ConversationID+"/messages?before=msg_missing", "", "", nil)
	if badCursor.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad cursor status = %d: %s", badCursor.StatusCode, readBody(badCursor))
	}
	rotate := requestJSON(t, client, http.MethodPost, server.URL+"/api/bots/"+bot.ID+"/key", server.URL, "", nil)
	var rotated map[string]string
	decodeResponse(t, rotate, http.StatusCreated, &rotated)
	oldDenied := requestJSON(t, http.DefaultClient, http.MethodGet, server.URL+"/api/conversations/"+boot.ConversationID+"/messages", "", bot.APIKey, nil)
	if oldDenied.StatusCode != http.StatusUnauthorized {
		t.Fatalf("rotated old key status = %d: %s", oldDenied.StatusCode, readBody(oldDenied))
	}
	newAllowed := requestJSON(t, http.DefaultClient, http.MethodGet, server.URL+"/api/conversations/"+boot.ConversationID+"/messages", "", rotated["api_key"], nil)
	if newAllowed.StatusCode != http.StatusOK {
		t.Fatalf("rotated new key status = %d: %s", newAllowed.StatusCode, readBody(newAllowed))
	}
	revoke := requestJSON(t, client, http.MethodDelete, server.URL+"/api/bots/"+bot.ID+"/key", server.URL, "", nil)
	if revoke.StatusCode != http.StatusNoContent {
		t.Fatalf("revoke status = %d: %s", revoke.StatusCode, readBody(revoke))
	}
	denied := requestJSON(t, http.DefaultClient, http.MethodGet, server.URL+"/api/conversations/"+boot.ConversationID+"/messages", "", rotated["api_key"], nil)
	if denied.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked key status = %d: %s", denied.StatusCode, readBody(denied))
	}
	logout := requestJSON(t, client, http.MethodDelete, server.URL+"/api/session", server.URL, "", nil)
	if logout.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status = %d: %s", logout.StatusCode, readBody(logout))
	}
	afterLogout := requestJSON(t, client, http.MethodGet, server.URL+"/api/me", "", "", nil)
	if afterLogout.StatusCode != http.StatusUnauthorized {
		t.Fatalf("after logout status = %d: %s", afterLogout.StatusCode, readBody(afterLogout))
	}
}

func TestCreateBot_RejectsInaccessibleConversationsAndRollsBack(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "authorization.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	boot, err := app.Bootstrap(context.Background(), db, "owner@example.com", "Owner", "correct horse battery staple", "Owners")
	if err != nil {
		t.Fatal(err)
	}
	var passwordHash string
	if err := db.QueryRow(`SELECT password_hash FROM users WHERE id=?`, boot.UserID).Scan(&passwordHash); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO users(id,kind,email,name,password_hash,created_at) VALUES('usr_other','human','other@example.com','Other',?,?)`, []any{passwordHash, now}},
		{`INSERT INTO organisation_memberships(organisation_id,user_id,role,name_normalized,created_at) VALUES(?,'usr_other','member','other',?)`, []any{boot.OrganisationID, now}},
		{`INSERT INTO conversations(id,organisation_id,name,visibility,created_at) VALUES('con_private',?,'private','members',?)`, []any{boot.OrganisationID, now}},
		{`INSERT INTO conversation_members(conversation_id,user_id) VALUES('con_private','usr_other')`, nil},
		{`INSERT INTO organisations(id,name,created_at) VALUES('org_other','Other org',?)`, []any{now}},
		{`INSERT INTO conversations(id,organisation_id,name,visibility,created_at) VALUES('con_other','org_other','other','organisation',?)`, []any{now}},
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement.query, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	server := httptest.NewServer(httpapi.New(httpapi.Dependencies{DB: db}))
	defer server.Close()
	client := newCookieClient(t)
	client.Timeout = 3 * time.Second
	decodeResponse(t, requestJSON(t, client, http.MethodPost, server.URL+"/api/session", server.URL, "", map[string]any{"email": "owner@example.com", "password": "correct horse battery staple"}), http.StatusOK, &map[string]any{})
	for _, conversationID := range []string{"con_private", "con_other"} {
		response := requestJSON(t, client, http.MethodPost, server.URL+"/api/organisations/"+boot.OrganisationID+"/bots", server.URL, "", map[string]any{"name": "Denied " + conversationID, "conversation_ids": []string{conversationID}})
		if response.StatusCode != http.StatusForbidden {
			t.Fatalf("conversation %s status = %d, want 403: %s", conversationID, response.StatusCode, readBody(response))
		}
	}
	var bots int
	if err := db.QueryRow(`SELECT count(*) FROM users WHERE kind='bot'`).Scan(&bots); err != nil || bots != 0 {
		t.Fatalf("rolled-back bots = %d, err=%v", bots, err)
	}
}

func TestOrganisationAdministration_RequiresAdminAndRejectsDuplicateNames(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "roles.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	boot, err := app.Bootstrap(context.Background(), db, "michael@example.com", "Michael", "correct horse battery staple", "Mainstay")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(httpapi.New(httpapi.Dependencies{DB: db}))
	defer server.Close()
	admin := newCookieClient(t)
	decodeResponse(t, requestJSON(t, admin, http.MethodPost, server.URL+"/api/session", server.URL, "", map[string]any{"email": "michael@example.com", "password": "correct horse battery staple"}), http.StatusOK, &map[string]any{})

	var organisations []map[string]string
	decodeResponse(t, requestJSON(t, admin, http.MethodGet, server.URL+"/api/organisations", "", "", nil), http.StatusOK, &organisations)
	if len(organisations) != 1 || organisations[0]["role"] != "admin" {
		t.Fatalf("organisations = %#v", organisations)
	}
	var users []map[string]string
	decodeResponse(t, requestJSON(t, admin, http.MethodGet, server.URL+"/api/organisations/"+boot.OrganisationID+"/users", "", "", nil), http.StatusOK, &users)
	if len(users) != 1 || users[0]["name"] != "Michael" || users[0]["role"] != "admin" {
		t.Fatalf("users = %#v", users)
	}

	created := requestJSON(t, admin, http.MethodPost, server.URL+"/api/organisations/"+boot.OrganisationID+"/bots", server.URL, "", map[string]any{"name": "Hector"})
	var bot struct {
		ID     string `json:"id"`
		APIKey string `json:"api_key"`
		Role   string `json:"role"`
	}
	decodeResponse(t, created, http.StatusCreated, &bot)
	if bot.Role != "member" {
		t.Fatalf("bot role = %q", bot.Role)
	}
	duplicate := requestJSON(t, admin, http.MethodPost, server.URL+"/api/organisations/"+boot.OrganisationID+"/bots", server.URL, "", map[string]any{"name": "  heCTOR  "})
	if duplicate.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate status = %d: %s", duplicate.StatusCode, readBody(duplicate))
	}
	decodeResponse(t, requestJSON(t, admin, http.MethodPost, server.URL+"/api/organisations/"+boot.OrganisationID+"/bots", server.URL, "", map[string]any{"name": "Élodie"}), http.StatusCreated, &map[string]any{})
	unicodeDuplicate := requestJSON(t, admin, http.MethodPost, server.URL+"/api/organisations/"+boot.OrganisationID+"/bots", server.URL, "", map[string]any{"name": "\u2003élodie\u00a0"})
	if unicodeDuplicate.StatusCode != http.StatusConflict {
		t.Fatalf("unicode duplicate status = %d: %s", unicodeDuplicate.StatusCode, readBody(unicodeDuplicate))
	}

	var passwordHash string
	if err := db.QueryRow(`SELECT password_hash FROM users WHERE id=?`, boot.UserID).Scan(&passwordHash); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO users(id,kind,email,name,password_hash,created_at) VALUES('usr_member','human','member@example.com','Member',?,?)`, passwordHash, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO organisation_memberships(organisation_id,user_id,role,name_normalized,created_at) VALUES(?,'usr_member','member','member',?)`, boot.OrganisationID, now); err != nil {
		t.Fatal(err)
	}
	member := newCookieClient(t)
	decodeResponse(t, requestJSON(t, member, http.MethodPost, server.URL+"/api/session", server.URL, "", map[string]any{"email": "member@example.com", "password": "correct horse battery staple"}), http.StatusOK, &map[string]any{})
	for action, response := range map[string]*http.Response{
		"create bot":      requestJSON(t, member, http.MethodPost, server.URL+"/api/organisations/"+boot.OrganisationID+"/bots", server.URL, "", map[string]any{"name": "Denied"}),
		"rotate key":      requestJSON(t, member, http.MethodPost, server.URL+"/api/bots/"+bot.ID+"/key", server.URL, "", nil),
		"revoke key":      requestJSON(t, member, http.MethodDelete, server.URL+"/api/bots/"+bot.ID+"/key", server.URL, "", nil),
		"bot creates bot": requestJSON(t, http.DefaultClient, http.MethodPost, server.URL+"/api/organisations/"+boot.OrganisationID+"/bots", "", bot.APIKey, map[string]any{"name": "Denied bot"}),
	} {
		if response.StatusCode != http.StatusForbidden {
			t.Fatalf("%s status = %d: %s", action, response.StatusCode, readBody(response))
		}
		response.Body.Close()
	}
}

func TestAddExistingHuman_ListsEligibleAccountsAndRequiresAdmin(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "add-existing-human.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	boot, err := app.Bootstrap(context.Background(), db, "michael@example.com", "Michael", "correct horse battery staple", "Mainstay")
	if err != nil {
		t.Fatal(err)
	}
	var passwordHash string
	if err := db.QueryRow(`SELECT password_hash FROM users WHERE id=?`, boot.UserID).Scan(&passwordHash); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, account := range []struct{ id, email, name string }{
		{"usr_member", "member@example.com", "Member"},
		{"usr_casey", "casey@example.com", "Casey"},
		{"usr_collision", "collision@example.com", " ÉLODIE "},
	} {
		if _, err := db.Exec(`INSERT INTO users(id,kind,email,name,password_hash,created_at) VALUES(?,'human',?,?,?,?)`, account.id, account.email, account.name, passwordHash, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO organisation_memberships(organisation_id,user_id,role,name_normalized,created_at) VALUES(?,'usr_member','member','member',?)`, boot.OrganisationID, now); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(httpapi.New(httpapi.Dependencies{DB: db}))
	defer server.Close()
	admin := newCookieClient(t)
	decodeResponse(t, requestJSON(t, admin, http.MethodPost, server.URL+"/api/session", server.URL, "", map[string]any{"email": "michael@example.com", "password": "correct horse battery staple"}), http.StatusOK, &map[string]any{})
	member := newCookieClient(t)
	decodeResponse(t, requestJSON(t, member, http.MethodPost, server.URL+"/api/session", server.URL, "", map[string]any{"email": "member@example.com", "password": "correct horse battery staple"}), http.StatusOK, &map[string]any{})
	var bot struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		APIKey string `json:"api_key"`
	}
	decodeResponse(t, requestJSON(t, admin, http.MethodPost, server.URL+"/api/organisations/"+boot.OrganisationID+"/bots", server.URL, "", map[string]any{"name": "Élodie"}), http.StatusCreated, &bot)
	if bot.Name != "Élodie" {
		t.Fatalf("bot display name = %q", bot.Name)
	}
	eligibleEndpoint := server.URL + "/api/organisations/" + boot.OrganisationID + "/eligible-users"
	caseySearch := eligibleEndpoint + "?email=" + url.QueryEscape("casey@example.com")
	addEndpoint := server.URL + "/api/organisations/" + boot.OrganisationID + "/users"

	for name, response := range map[string]*http.Response{
		"member searches eligible users": requestJSON(t, member, http.MethodGet, caseySearch, "", "", nil),
		"bot searches eligible users":    requestJSON(t, http.DefaultClient, http.MethodGet, caseySearch, "", bot.APIKey, nil),
		"member adds a user":             requestJSON(t, member, http.MethodPost, addEndpoint, server.URL, "", map[string]any{"user_id": "usr_casey"}),
	} {
		if response.StatusCode != http.StatusForbidden {
			t.Fatalf("%s status = %d: %s", name, response.StatusCode, readBody(response))
		}
		response.Body.Close()
	}

	missingEmail := requestJSON(t, admin, http.MethodGet, eligibleEndpoint, "", "", nil)
	if missingEmail.StatusCode != http.StatusBadRequest {
		t.Fatalf("missing email status = %d: %s", missingEmail.StatusCode, readBody(missingEmail))
	}
	missingEmail.Body.Close()
	var eligible []map[string]any
	decodeResponse(t, requestJSON(t, admin, http.MethodGet, caseySearch, "", "", nil), http.StatusOK, &eligible)
	if len(eligible) != 1 || eligible[0]["id"] != "usr_casey" {
		t.Fatalf("eligible = %#v, want only Casey; conflicting names must be excluded", eligible)
	}
	for _, account := range eligible {
		if len(account) != 3 || account["id"] == nil || account["name"] == nil || account["email"] == nil {
			t.Fatalf("eligible account exposes wrong fields: %#v", account)
		}
		if account["password_hash"] != nil {
			t.Fatalf("eligible account exposed password hash: %#v", account)
		}
	}
	var conflicting []map[string]any
	decodeResponse(t, requestJSON(t, admin, http.MethodGet, eligibleEndpoint+"?email="+url.QueryEscape("collision@example.com"), "", "", nil), http.StatusOK, &conflicting)
	if len(conflicting) != 0 {
		t.Fatalf("conflicting account was offered: %#v", conflicting)
	}

	var added struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
		Name string `json:"name"`
		Role string `json:"role"`
	}
	decodeResponse(t, requestJSON(t, admin, http.MethodPost, addEndpoint, server.URL, "", map[string]any{"user_id": "usr_casey"}), http.StatusCreated, &added)
	if added.ID != "usr_casey" || added.Kind != "human" || added.Name != "Casey" || added.Role != "member" {
		t.Fatalf("added user = %#v", added)
	}
	for name, userID := range map[string]string{"duplicate membership": "usr_casey", "normalised name collision": "usr_collision"} {
		response := requestJSON(t, admin, http.MethodPost, addEndpoint, server.URL, "", map[string]any{"user_id": userID})
		if response.StatusCode != http.StatusConflict {
			t.Fatalf("%s status = %d: %s", name, response.StatusCode, readBody(response))
		}
		response.Body.Close()
	}
}

func TestRemoveBot_RequiresAdminRevokesAccessAndPreservesMessages(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "remove-bot.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	boot, err := app.Bootstrap(context.Background(), db, "michael@example.com", "Michael", "correct horse battery staple", "Mainstay")
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.New(httpapi.Dependencies{DB: db})
	server := httptest.NewServer(handler)
	defer server.Close()
	admin := newCookieClient(t)
	decodeResponse(t, requestJSON(t, admin, http.MethodPost, server.URL+"/api/session", server.URL, "", map[string]any{"email": "michael@example.com", "password": "correct horse battery staple"}), http.StatusOK, &map[string]any{})
	var bot struct {
		ID     string `json:"id"`
		APIKey string `json:"api_key"`
	}
	decodeResponse(t, requestJSON(t, admin, http.MethodPost, server.URL+"/api/organisations/"+boot.OrganisationID+"/bots", server.URL, "", map[string]any{"name": "Hector"}), http.StatusCreated, &bot)
	decodeResponse(t, requestJSON(t, http.DefaultClient, http.MethodPost, server.URL+"/api/conversations/"+boot.ConversationID+"/messages", "", bot.APIKey, map[string]any{"body": "Keep this attribution", "client_id": "before-removal"}), http.StatusCreated, &map[string]any{})
	var privateConversation map[string]any
	decodeResponse(t, requestJSON(t, admin, http.MethodPost, server.URL+"/api/organisations/"+boot.OrganisationID+"/conversations", server.URL, "", map[string]any{"name": "private", "visibility": "members", "member_ids": []string{bot.ID}}), http.StatusCreated, &privateConversation)

	var passwordHash string
	if err := db.QueryRow(`SELECT password_hash FROM users WHERE id=?`, boot.UserID).Scan(&passwordHash); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO users(id,kind,email,name,password_hash,created_at) VALUES('usr_member','human','member@example.com','Member',?,?)`, passwordHash, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO organisation_memberships(organisation_id,user_id,role,name_normalized,created_at) VALUES(?,'usr_member','member','member',?)`, boot.OrganisationID, now); err != nil {
		t.Fatal(err)
	}
	member := newCookieClient(t)
	decodeResponse(t, requestJSON(t, member, http.MethodPost, server.URL+"/api/session", server.URL, "", map[string]any{"email": "member@example.com", "password": "correct horse battery staple"}), http.StatusOK, &map[string]any{})
	endpoint := server.URL + "/api/organisations/" + boot.OrganisationID + "/bots/" + bot.ID
	for action, response := range map[string]*http.Response{
		"member removes bot": requestJSON(t, member, http.MethodDelete, endpoint, server.URL, "", nil),
		"bot removes itself": requestJSON(t, http.DefaultClient, http.MethodDelete, endpoint, "", bot.APIKey, nil),
	} {
		if response.StatusCode != http.StatusForbidden {
			t.Fatalf("%s status = %d: %s", action, response.StatusCode, readBody(response))
		}
		response.Body.Close()
	}

	slowBody := &delayedJSONBody{started: make(chan struct{}), release: make(chan struct{}), body: []byte(`{"body":"Too late","client_id":"after-removal"}`)}
	slowRequest := httptest.NewRequest(http.MethodPost, server.URL+"/api/conversations/"+boot.ConversationID+"/messages", slowBody)
	slowRequest.Header.Set("Authorization", "Bearer "+bot.APIKey)
	slowRecorder := httptest.NewRecorder()
	slowDone := make(chan struct{})
	go func() {
		handler.ServeHTTP(slowRecorder, slowRequest)
		close(slowDone)
	}()
	<-slowBody.started
	retryBody := &delayedJSONBody{started: make(chan struct{}), release: make(chan struct{}), body: []byte(`{"body":"Retry","client_id":"before-removal"}`)}
	retryRequest := httptest.NewRequest(http.MethodPost, server.URL+"/api/conversations/"+boot.ConversationID+"/messages", retryBody)
	retryRequest.Header.Set("Authorization", "Bearer "+bot.APIKey)
	retryRecorder := httptest.NewRecorder()
	retryDone := make(chan struct{})
	go func() {
		handler.ServeHTTP(retryRecorder, retryRequest)
		close(retryDone)
	}()
	<-retryBody.started

	removed := requestJSON(t, admin, http.MethodDelete, endpoint, server.URL, "", nil)
	if removed.StatusCode != http.StatusNoContent {
		t.Fatalf("remove status = %d: %s", removed.StatusCode, readBody(removed))
	}
	removed.Body.Close()
	privateConversationID := privateConversation["id"].(string)
	remainingMemberPost := requestJSON(t, admin, http.MethodPost, server.URL+"/api/conversations/"+privateConversationID+"/messages", server.URL, "", map[string]any{"body": "Nobody else is here", "client_id": "one-member-private"})
	if remainingMemberPost.StatusCode != http.StatusForbidden {
		t.Fatalf("one-member private post status = %d, want 403: %s", remainingMemberPost.StatusCode, readBody(remainingMemberPost))
	}
	remainingMemberPost.Body.Close()
	var privateMessageCount, privateEventCount int
	if err := db.QueryRow(`SELECT count(*) FROM messages WHERE conversation_id=?`, privateConversationID).Scan(&privateMessageCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM realtime_events WHERE conversation_id=?`, privateConversationID).Scan(&privateEventCount); err != nil {
		t.Fatal(err)
	}
	if privateMessageCount != 0 || privateEventCount != 0 {
		t.Fatalf("one-member private inserts: messages=%d events=%d", privateMessageCount, privateEventCount)
	}
	close(slowBody.release)
	close(retryBody.release)
	<-slowDone
	<-retryDone
	if slowRecorder.Code != http.StatusForbidden {
		t.Fatalf("in-flight post status = %d: %s", slowRecorder.Code, slowRecorder.Body.String())
	}
	if retryRecorder.Code != http.StatusForbidden {
		t.Fatalf("in-flight retry status = %d: %s", retryRecorder.Code, retryRecorder.Body.String())
	}
	var lateMessages int
	if err := db.QueryRow(`SELECT count(*) FROM messages WHERE client_id='after-removal'`).Scan(&lateMessages); err != nil || lateMessages != 0 {
		t.Fatalf("messages after removal = %d, err=%v", lateMessages, err)
	}
	oldKey := requestJSON(t, http.DefaultClient, http.MethodGet, server.URL+"/api/organisations", "", bot.APIKey, nil)
	if oldKey.StatusCode != http.StatusUnauthorized {
		t.Fatalf("removed key status = %d: %s", oldKey.StatusCode, readBody(oldKey))
	}
	oldKey.Body.Close()
	for label, query := range map[string]string{
		"memberships":         `SELECT count(*) FROM organisation_memberships WHERE user_id=?`,
		"api keys":            `SELECT count(*) FROM api_keys WHERE user_id=?`,
		"conversation access": `SELECT count(*) FROM conversation_members WHERE user_id=?`,
	} {
		var count int
		if err := db.QueryRow(query, bot.ID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s = %d, err=%v", label, count, err)
		}
	}
	var authorName string
	if err := db.QueryRow(`SELECT u.name FROM messages m JOIN users u ON u.id=m.author_id WHERE m.client_id='before-removal'`).Scan(&authorName); err != nil || authorName != "Hector" {
		t.Fatalf("preserved author = %q, err=%v", authorName, err)
	}
}

func TestDeleteConversation_AdminRemovesItForEveryoneAndPreventsFurtherMessages(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "delete-conversation.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	boot, err := app.Bootstrap(context.Background(), db, "owner@example.com", "Owner", "correct horse battery staple", "Mainstay")
	if err != nil {
		t.Fatal(err)
	}
	var passwordHash string
	if err := db.QueryRow(`SELECT password_hash FROM users WHERE id=?`, boot.UserID).Scan(&passwordHash); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO users(id,kind,email,name,password_hash,created_at) VALUES('usr_member','human','member@example.com','Member',?,?)`, passwordHash, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO organisation_memberships(organisation_id,user_id,role,name_normalized,created_at) VALUES(?,'usr_member','member','member',?)`, boot.OrganisationID, now); err != nil {
		t.Fatal(err)
	}

	handler := httpapi.New(httpapi.Dependencies{DB: db})
	server := httptest.NewServer(handler)
	defer server.Close()
	admin := newCookieClient(t)
	member := newCookieClient(t)
	decodeResponse(t, requestJSON(t, admin, http.MethodPost, server.URL+"/api/session", server.URL, "", map[string]any{"email": "owner@example.com", "password": "correct horse battery staple"}), http.StatusOK, &map[string]any{})
	decodeResponse(t, requestJSON(t, member, http.MethodPost, server.URL+"/api/session", server.URL, "", map[string]any{"email": "member@example.com", "password": "correct horse battery staple"}), http.StatusOK, &map[string]any{})
	memberSocket := dialSocket(t, server.URL+"/api/ws?after=0", http.Header{"Cookie": {sessionCookieFor(t, member, server.URL).String()}})
	defer memberSocket.CloseNow()
	decodeResponse(t, requestJSON(t, member, http.MethodPost, server.URL+"/api/conversations/"+boot.ConversationID+"/messages", server.URL, "", map[string]any{"body": "This conversation will be deleted", "client_id": "before-delete"}), http.StatusCreated, &map[string]any{})
	_ = readEvent(t, memberSocket)

	endpoint := server.URL + "/api/organisations/" + boot.OrganisationID + "/conversations/" + boot.ConversationID
	denied := requestJSON(t, member, http.MethodDelete, endpoint, server.URL, "", nil)
	if denied.StatusCode != http.StatusForbidden {
		t.Fatalf("member delete status = %d, want 403: %s", denied.StatusCode, readBody(denied))
	}
	denied.Body.Close()

	slowBody := &delayedJSONBody{started: make(chan struct{}), release: make(chan struct{}), body: []byte(`{"body":"Too late","client_id":"after-delete"}`)}
	slowRequest := httptest.NewRequest(http.MethodPost, server.URL+"/api/conversations/"+boot.ConversationID+"/messages", slowBody)
	slowRequest.AddCookie(sessionCookieFor(t, member, server.URL))
	slowRequest.Header.Set("Origin", server.URL)
	slowRecorder := httptest.NewRecorder()
	slowDone := make(chan struct{})
	go func() {
		handler.ServeHTTP(slowRecorder, slowRequest)
		close(slowDone)
	}()
	<-slowBody.started

	deleted := requestJSON(t, admin, http.MethodDelete, endpoint, server.URL, "", nil)
	if deleted.StatusCode != http.StatusNoContent {
		t.Fatalf("admin delete status = %d, want 204: %s", deleted.StatusCode, readBody(deleted))
	}
	deleted.Body.Close()
	deletedEvent := readEvent(t, memberSocket)
	deletedPayload, _ := deletedEvent["payload"].(map[string]any)
	if deletedEvent["type"] != "conversation.deleted" || deletedPayload["id"] != boot.ConversationID {
		t.Fatalf("deleted event = %#v", deletedEvent)
	}
	close(slowBody.release)
	<-slowDone
	if slowRecorder.Code != http.StatusForbidden {
		t.Fatalf("in-flight post status = %d, want 403: %s", slowRecorder.Code, slowRecorder.Body.String())
	}

	var conversations []map[string]string
	decodeResponse(t, requestJSON(t, member, http.MethodGet, server.URL+"/api/organisations/"+boot.OrganisationID+"/conversations", "", "", nil), http.StatusOK, &conversations)
	if len(conversations) != 0 {
		t.Fatalf("conversations after delete = %#v", conversations)
	}
	for label, query := range map[string]string{
		"conversation":    `SELECT count(*) FROM conversations WHERE id=?`,
		"messages":        `SELECT count(*) FROM messages WHERE conversation_id=?`,
		"realtime events": `SELECT count(*) FROM realtime_events WHERE conversation_id=?`,
	} {
		var count int
		if err := db.QueryRow(query, boot.ConversationID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s after delete = %d, err=%v", label, count, err)
		}
	}
}

func sessionCookieFor(t *testing.T, client *http.Client, endpoint string) *http.Cookie {
	t.Helper()
	parsed, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	for _, cookie := range client.Jar.Cookies(parsed) {
		if cookie.Name == "kmainstay_session" {
			return cookie
		}
	}
	t.Fatal("session cookie missing")
	return nil
}

type delayedJSONBody struct {
	started, release chan struct{}
	body             []byte
	announced, sent  bool
}

func (b *delayedJSONBody) Read(p []byte) (int, error) {
	if b.sent {
		return 0, io.EOF
	}
	if !b.announced {
		close(b.started)
		b.announced = true
	}
	<-b.release
	b.sent = true
	return copy(p, b.body), nil
}

func (b *delayedJSONBody) Close() error { return nil }

func TestLogin_NormalisedEmailLimitAndExpiredSessionCleanup(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "login.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	boot, err := app.Bootstrap(context.Background(), db, "user@example.com", "User", "correct horse battery staple", "Org")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO human_sessions(token_hash,user_id,expires_at,created_at) VALUES(?,?,?,?)`, []byte("expired"), boot.UserID, time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano), time.Now().Add(-2*time.Hour).UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(httpapi.New(httpapi.Dependencies{DB: db}))
	defer server.Close()
	client := newCookieClient(t)
	for i := 0; i < 5; i++ {
		response := requestJSON(t, client, http.MethodPost, server.URL+"/api/session", server.URL, "", map[string]any{"email": " User@Example.COM ", "password": "wrong password"})
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d", i+1, response.StatusCode)
		}
		response.Body.Close()
	}
	limited := requestJSON(t, client, http.MethodPost, server.URL+"/api/session", server.URL, "", map[string]any{"email": "user@example.com", "password": "wrong password"})
	if limited.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("limited status = %d: %s", limited.StatusCode, readBody(limited))
	}
	limited.Body.Close()

	server.Close()
	server = httptest.NewServer(httpapi.New(httpapi.Dependencies{DB: db}))
	success := requestJSON(t, client, http.MethodPost, server.URL+"/api/session", server.URL, "", map[string]any{"email": "user@example.com", "password": "correct horse battery staple"})
	decodeResponse(t, success, http.StatusOK, &map[string]any{})
	var expired int
	if err := db.QueryRow(`SELECT count(*) FROM human_sessions WHERE expires_at<=?`, time.Now().UTC().Format(time.RFC3339Nano)).Scan(&expired); err != nil || expired != 0 {
		t.Fatalf("expired sessions = %d, err=%v", expired, err)
	}
}

func newCookieClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := newMemoryJar()
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar}
}

func newMemoryJar() (http.CookieJar, error) { return cookieJar() }

func requestJSON(t *testing.T, client *http.Client, method, endpoint, origin, bearer string, body any) *http.Response {
	t.Helper()
	var encoded []byte
	if body != nil {
		encoded, _ = json.Marshal(body)
	}
	req, err := http.NewRequest(method, endpoint, bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func decodeResponse(t *testing.T, response *http.Response, status int, out any) {
	t.Helper()
	defer response.Body.Close()
	if response.StatusCode != status {
		t.Fatalf("status = %d, want %d: %s", response.StatusCode, status, readBody(response))
	}
	if err := json.NewDecoder(response.Body).Decode(out); err != nil {
		t.Fatal(err)
	}
}

func readBody(response *http.Response) string {
	defer response.Body.Close()
	var value any
	_ = json.NewDecoder(response.Body).Decode(&value)
	return fmt.Sprint(value)
}

func dialSocket(t *testing.T, endpoint string, header http.Header) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	endpoint = "ws" + strings.TrimPrefix(endpoint, "http")
	connection, response, err := websocket.Dial(ctx, endpoint, &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		if response != nil {
			t.Fatalf("websocket status %d: %v", response.StatusCode, err)
		}
		t.Fatal(err)
	}
	return connection
}

func readEvent(t *testing.T, connection *websocket.Conn) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var event map[string]any
	if err := wsjson.Read(ctx, connection, &event); err != nil {
		t.Fatal(err)
	}
	return event
}

func bodyOf(event map[string]any) string {
	payload, _ := event["payload"].(map[string]any)
	body, _ := payload["body"].(string)
	return body
}
