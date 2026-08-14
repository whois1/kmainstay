package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	var privateConversation map[string]string
	decodeResponse(t, privateResponse, http.StatusCreated, &privateConversation)
	privateDenied := requestJSON(t, http.DefaultClient, http.MethodGet, server.URL+"/api/conversations/"+privateConversation["id"]+"/messages", "", bot.APIKey, nil)
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
	humanPost := requestJSON(t, client, http.MethodPost, server.URL+"/api/conversations/"+boot.ConversationID+"/messages", server.URL, "", map[string]any{"body": "  Hello **Hector**  ", "client_id": "browser-1"})
	decodeResponse(t, humanPost, http.StatusCreated, &map[string]any{})
	botEvent := readEvent(t, botSocket)
	if bodyOf(botEvent) != "  Hello **Hector**  " {
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
	var secondConversation map[string]string
	decodeResponse(t, secondConversationResponse, http.StatusCreated, &secondConversation)
	sameClientIDElsewhere := requestJSON(t, http.DefaultClient, http.MethodPost, server.URL+"/api/conversations/"+secondConversation["id"]+"/messages", "", bot.APIKey, map[string]any{"body": "different conversation", "client_id": "hector-1"})
	var secondMessage map[string]any
	decodeResponse(t, sameClientIDElsewhere, http.StatusCreated, &secondMessage)
	if secondMessage["conversation_id"] != secondConversation["id"] {
		t.Fatalf("idempotency returned message from %v, want %s", secondMessage["conversation_id"], secondConversation["id"])
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
	if err := db.QueryRow(`SELECT count(*) FROM realtime_events WHERE sequence > ? AND conversation_id=?`, humanSequence, boot.ConversationID).Scan(&replayable); err != nil || replayable != 1 {
		t.Fatalf("replayable events = %d, want 1: %v", replayable, err)
	}
	replaySocket := dialSocket(t, server.URL+fmt.Sprintf("/api/ws?after=%d", humanSequence), http.Header{"Authorization": {"Bearer " + bot.APIKey}})
	replayed := readEvent(t, replaySocket)
	replaySocket.CloseNow()
	if bodyOf(replayed) != "Hello Michael" {
		t.Fatalf("replayed event = %#v", replayed)
	}
	messagesResponse := requestJSON(t, client, http.MethodGet, server.URL+"/api/conversations/"+boot.ConversationID+"/messages", "", "", nil)
	var messages []map[string]any
	decodeResponse(t, messagesResponse, http.StatusOK, &messages)
	if len(messages) != 2 || messages[0]["body"] != "  Hello **Hector**  " || messages[1]["body"] != "Hello Michael" {
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
	if len(previous) != 1 || previous[0]["body"] != "  Hello **Hector**  " {
		t.Fatalf("previous page = %#v", previous)
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
		{`INSERT INTO organisation_memberships(organisation_id,user_id,created_at) VALUES(?,'usr_other',?)`, []any{boot.OrganisationID, now}},
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
