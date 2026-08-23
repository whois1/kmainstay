package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"kmainstay/internal/attachments"
	"kmainstay/internal/auth"
	"kmainstay/internal/database"
)

type Dependencies struct {
	DB             *sql.DB
	SecureCookies  bool
	AllowedOrigins []string
	Assets         http.Handler
	Attachments    attachments.Store
}

type server struct {
	db               *sql.DB
	secureCookies    bool
	allowedOrigins   map[string]bool
	hub              *hub
	mu               sync.Mutex
	lastMessage      map[string][]time.Time
	loginLimiter     *loginLimiter
	attachments      attachments.Store
	imageUploadSlots chan struct{}
}

type principal struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type contextKey struct{}

func New(deps Dependencies) http.Handler {
	s := &server{db: deps.DB, secureCookies: deps.SecureCookies, allowedOrigins: map[string]bool{}, hub: newHub(), lastMessage: map[string][]time.Time{}, loginLimiter: newLoginLimiter(5, time.Minute, 1024), attachments: deps.Attachments, imageUploadSlots: make(chan struct{}, 2)}
	for _, origin := range deps.AllowedOrigins {
		s.allowedOrigins[origin] = true
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health)
	if deps.DB != nil {
		mux.HandleFunc("POST /api/session", s.login)
		mux.Handle("DELETE /api/session", s.withAuth(http.HandlerFunc(s.logout)))
		mux.Handle("GET /api/me", s.withAuth(http.HandlerFunc(s.me)))
		mux.Handle("GET /api/organisations", s.withAuth(http.HandlerFunc(s.organisations)))
		mux.Handle("GET /api/organisations/{organisation}/conversations", s.withAuth(http.HandlerFunc(s.conversations)))
		mux.Handle("POST /api/organisations/{organisation}/conversations", s.withAuth(http.HandlerFunc(s.createConversation)))
		mux.Handle("POST /api/organisations/{organisation}/direct-conversations/{user}", s.withAuth(http.HandlerFunc(s.directConversation)))
		mux.Handle("PUT /api/conversations/{conversation}/title", s.withAuth(http.HandlerFunc(s.updateConversationTitle)))
		mux.Handle("DELETE /api/organisations/{organisation}/conversations/{conversation}", s.withAuth(http.HandlerFunc(s.deleteConversation)))
		mux.Handle("GET /api/organisations/{organisation}/users", s.withAuth(http.HandlerFunc(s.users)))
		mux.Handle("POST /api/organisations/{organisation}/users", s.withAuth(http.HandlerFunc(s.addOrganisationUser)))
		mux.Handle("GET /api/organisations/{organisation}/eligible-users", s.withAuth(http.HandlerFunc(s.eligibleOrganisationUsers)))
		mux.Handle("POST /api/organisations/{organisation}/bots", s.withAuth(http.HandlerFunc(s.createBot)))
		mux.Handle("DELETE /api/organisations/{organisation}/bots/{bot}", s.withAuth(http.HandlerFunc(s.removeBot)))
		mux.Handle("POST /api/bots/{bot}/key", s.withAuth(http.HandlerFunc(s.rotateBotKey)))
		mux.Handle("DELETE /api/bots/{bot}/key", s.withAuth(http.HandlerFunc(s.revokeBotKey)))
		mux.Handle("GET /api/conversations/{conversation}/messages", s.withAuth(http.HandlerFunc(s.messages)))
		mux.Handle("POST /api/conversations/{conversation}/messages", s.withAuth(http.HandlerFunc(s.postMessage)))
		mux.Handle("GET /api/attachments/{attachment}/content", s.withAuth(http.HandlerFunc(s.attachmentContent)))
		mux.Handle("PUT /api/conversations/{conversation}/read", s.withAuth(http.HandlerFunc(s.putConversationRead)))
		mux.Handle("GET /api/ws", s.withAuth(http.HandlerFunc(s.webSocket)))
	}
	if deps.Assets != nil {
		mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}
			deps.Assets.ServeHTTP(w, r)
		}))
	}
	return mux
}

func health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, viaCookie, err := s.authenticate(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if viaCookie && isMutation(r.Method) && !s.validOrigin(r) {
			writeError(w, http.StatusForbidden, "origin not allowed")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, p)))
	})
}

func (s *server) authenticate(r *http.Request) (principal, bool, error) {
	if bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "); bearer != "" {
		parts := strings.SplitN(bearer, "_", 4)
		if len(parts) != 4 || parts[0] != "km" || parts[1] != "live" {
			return principal{}, false, errors.New("bad bearer")
		}
		var p principal
		var verifier string
		err := s.db.QueryRowContext(r.Context(), `SELECT u.id,u.kind,u.name,k.secret_hash FROM api_keys k JOIN users u ON u.id=k.user_id WHERE k.lookup=? AND k.revoked_at IS NULL`, parts[2]).Scan(&p.ID, &p.Kind, &p.Name, &verifier)
		if err != nil || !auth.VerifySecret(verifier, parts[3]) {
			return principal{}, false, errors.New("bad bearer")
		}
		return p, false, nil
	}
	cookie, err := r.Cookie("kmainstay_session")
	if err != nil {
		return principal{}, true, err
	}
	digest := sha256.Sum256([]byte(cookie.Value))
	var p principal
	err = s.db.QueryRowContext(r.Context(), `SELECT u.id,u.kind,u.name FROM human_sessions h JOIN users u ON u.id=h.user_id WHERE h.token_hash=? AND h.expires_at>?`, digest[:], nowText()).Scan(&p.ID, &p.Kind, &p.Name)
	return p, true, err
}

func (s *server) validOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	if len(s.allowedOrigins) != 0 {
		return s.allowedOrigins[origin]
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host == r.Host && (u.Scheme == "http" || u.Scheme == "https")
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	if !s.validOrigin(r) {
		writeError(w, http.StatusForbidden, "origin not allowed")
		return
	}
	var in struct{ Email, Password string }
	if !decode(w, r, &in) {
		return
	}
	if !s.loginLimiter.allow(in.Email, time.Now()) {
		writeError(w, http.StatusTooManyRequests, "try again later")
		return
	}
	var p principal
	var passwordHash string
	err := s.db.QueryRowContext(r.Context(), `SELECT id,kind,name,password_hash FROM users WHERE kind='human' AND lower(email)=lower(?)`, strings.TrimSpace(in.Email)).Scan(&p.ID, &p.Kind, &p.Name, &passwordHash)
	if err != nil || !auth.VerifyPassword(passwordHash, in.Password) {
		time.Sleep(20 * time.Millisecond)
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	_, _ = s.db.ExecContext(r.Context(), `DELETE FROM human_sessions WHERE expires_at<=?`, nowText())
	token := randomToken(32)
	digest := sha256.Sum256([]byte(token))
	created := time.Now().UTC()
	if _, err := s.db.ExecContext(r.Context(), `INSERT INTO human_sessions(token_hash,user_id,expires_at,created_at) VALUES(?,?,?,?)`, digest[:], p.ID, created.Add(7*24*time.Hour).Format(time.RFC3339Nano), created.Format(time.RFC3339Nano)); err != nil {
		writeError(w, http.StatusInternalServerError, "could not create session")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "kmainstay_session", Value: token, Path: "/", HttpOnly: true, Secure: s.secureCookies, SameSite: http.SameSiteLaxMode, MaxAge: 7 * 24 * 60 * 60})
	writeJSON(w, http.StatusOK, p)
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("kmainstay_session"); err == nil {
		digest := sha256.Sum256([]byte(cookie.Value))
		_, _ = s.db.ExecContext(r.Context(), `DELETE FROM human_sessions WHERE token_hash=?`, digest[:])
	}
	http.SetCookie(w, &http.Cookie{Name: "kmainstay_session", Path: "/", HttpOnly: true, Secure: s.secureCookies, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) me(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, current(r)) }

func (s *server) organisations(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), `SELECT o.id,o.name,m.role FROM organisations o JOIN organisation_memberships m ON m.organisation_id=o.id WHERE m.user_id=? ORDER BY o.created_at,o.id`, current(r).ID)
	if err != nil {
		writeError(w, 500, "database error")
		return
	}
	defer rows.Close()
	items := []map[string]string{}
	for rows.Next() {
		var id, name, role string
		if rows.Scan(&id, &name, &role) == nil {
			items = append(items, map[string]string{"id": id, "name": name, "role": role})
		}
	}
	writeJSON(w, 200, items)
}

func (s *server) conversations(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), `SELECT c.id,coalesce(c.title,c.name),c.visibility,coalesce(crp.sequence,0),coalesce((SELECT max(event.sequence) FROM realtime_events event WHERE event.conversation_id=c.id),0),
		coalesce((SELECT event.occurred_at FROM realtime_events event WHERE event.conversation_id=c.id ORDER BY event.sequence DESC LIMIT 1),c.created_at),c.title_automatic,
		coalesce((SELECT json_group_array(user_id) FROM conversation_members WHERE conversation_id=c.id),'[]')
		FROM conversations c
		JOIN organisation_memberships om ON om.organisation_id=c.organisation_id
		LEFT JOIN conversation_read_positions crp ON crp.conversation_id=c.id AND crp.user_id=om.user_id
		WHERE c.organisation_id=? AND om.user_id=? AND (c.visibility='organisation' OR (
			(SELECT count(*) FROM conversation_members member_count WHERE member_count.conversation_id=c.id)>=2
			AND EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=c.id AND cm.user_id=?)))
		ORDER BY julianday(coalesce((SELECT event.occurred_at FROM realtime_events event WHERE event.conversation_id=c.id ORDER BY event.sequence DESC LIMIT 1),c.created_at)) DESC,
		coalesce((SELECT max(event.sequence) FROM realtime_events event WHERE event.conversation_id=c.id),0) DESC,
		coalesce((SELECT event.occurred_at FROM realtime_events event WHERE event.conversation_id=c.id ORDER BY event.sequence DESC LIMIT 1),c.created_at) DESC,c.id DESC`, r.PathValue("organisation"), current(r).ID, current(r).ID)
	if err != nil {
		writeError(w, 500, "database error")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, name, visibility, activityAt, memberIDsJSON string
		var readSequence, latestSequence int64
		var titleAutomatic bool
		if rows.Scan(&id, &name, &visibility, &readSequence, &latestSequence, &activityAt, &titleAutomatic, &memberIDsJSON) == nil {
			var memberIDs []string
			_ = json.Unmarshal([]byte(memberIDsJSON), &memberIDs)
			items = append(items, map[string]any{"id": id, "name": name, "visibility": visibility, "member_ids": memberIDs, "read_sequence": readSequence, "latest_sequence": latestSequence, "activity_at": activityAt, "title_automatic": titleAutomatic})
		}
	}
	writeJSON(w, 200, items)
}

func (s *server) users(w http.ResponseWriter, r *http.Request) {
	if !s.organisationMember(r.Context(), r.PathValue("organisation"), current(r).ID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT u.id,u.kind,u.name,m.role FROM users u JOIN organisation_memberships m ON m.user_id=u.id WHERE m.organisation_id=? ORDER BY u.created_at,u.id`, r.PathValue("organisation"))
	if err != nil {
		returnServerError(w)
		return
	}
	defer rows.Close()
	items := []map[string]string{}
	for rows.Next() {
		var id, kind, name, role string
		if rows.Scan(&id, &kind, &name, &role) == nil {
			items = append(items, map[string]string{"id": id, "kind": kind, "name": name, "role": role})
		}
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *server) createConversation(w http.ResponseWriter, r *http.Request) {
	p := current(r)
	orgID := r.PathValue("organisation")
	if !s.organisationMember(r.Context(), orgID, p.ID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	var in struct {
		Name           string   `json:"name"`
		Visibility     string   `json:"visibility"`
		MemberIDs      []string `json:"member_ids"`
		AutomaticTitle bool     `json:"automatic_title"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || (in.Visibility != "organisation" && in.Visibility != "members") {
		writeError(w, http.StatusBadRequest, "invalid conversation")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		returnServerError(w)
		return
	}
	defer tx.Rollback()
	id, now := database.NewID("con"), nowText()
	internalName := in.Name
	if in.AutomaticTitle {
		internalName = "topic:" + id
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO conversations(id,organisation_id,name,title,title_automatic,visibility,created_at) VALUES(?,?,?,?,?,?,?)`, id, orgID, internalName, in.Name, in.AutomaticTitle, in.Visibility, now); err != nil {
		writeError(w, http.StatusConflict, "conversation already exists")
		return
	}
	if in.Visibility == "members" {
		members := append([]string{p.ID}, in.MemberIDs...)
		for _, memberID := range members {
			result, execErr := tx.ExecContext(r.Context(), `INSERT OR IGNORE INTO conversation_members(conversation_id,user_id) SELECT ?,user_id FROM organisation_memberships WHERE organisation_id=? AND user_id=?`, id, orgID, memberID)
			if execErr != nil {
				returnServerError(w)
				return
			}
			if affected, _ := result.RowsAffected(); affected == 0 && memberID != p.ID {
				writeError(w, http.StatusBadRequest, "member is not in organisation")
				return
			}
		}
	}
	if err := tx.Commit(); err != nil {
		returnServerError(w)
		return
	}
	returnedMemberIDs := []string{}
	if in.Visibility == "members" {
		returnedMemberIDs = append(returnedMemberIDs, p.ID)
		seen := map[string]bool{p.ID: true}
		for _, memberID := range in.MemberIDs {
			if !seen[memberID] {
				returnedMemberIDs = append(returnedMemberIDs, memberID)
				seen[memberID] = true
			}
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "name": in.Name, "visibility": in.Visibility, "member_ids": returnedMemberIDs, "activity_at": now, "title_automatic": in.AutomaticTitle})
}

func (s *server) updateConversationTitle(w http.ResponseWriter, r *http.Request) {
	conversationID := r.PathValue("conversation")
	var input struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len(input.Name) > 20000 {
		writeError(w, http.StatusBadRequest, "invalid title")
		return
	}
	principalID := current(r).ID
	result, err := s.db.ExecContext(r.Context(), `UPDATE conversations SET title=?,title_automatic=0
		WHERE id=?
		AND EXISTS(SELECT 1 FROM organisation_memberships membership WHERE membership.organisation_id=conversations.organisation_id AND membership.user_id=?)
		AND (visibility='organisation' OR (
			EXISTS(SELECT 1 FROM conversation_members member WHERE member.conversation_id=conversations.id AND member.user_id=?)
			AND (SELECT count(*) FROM conversation_members member_count WHERE member_count.conversation_id=conversations.id)>=2))`, input.Name, conversationID, principalID, principalID)
	if err != nil {
		returnServerError(w)
		return
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		returnServerError(w)
		return
	} else if affected != 1 {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": conversationID, "name": input.Name, "title_automatic": false})
}

func (s *server) directConversation(w http.ResponseWriter, r *http.Request) {
	p := current(r)
	organisationID := r.PathValue("organisation")
	otherUserID := r.PathValue("user")
	if otherUserID == p.ID || !s.organisationMember(r.Context(), organisationID, p.ID) || !s.organisationMember(r.Context(), organisationID, otherUserID) {
		writeError(w, http.StatusBadRequest, "direct conversation requires another organisation member")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	conversation, err := s.exactDirectConversation(r.Context(), organisationID, p.ID, otherUserID)
	if err == nil {
		writeJSON(w, http.StatusOK, conversation)
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		returnServerError(w)
		return
	}

	conversationID := database.NewID("con")
	name := "direct:" + conversationID
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		returnServerError(w)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO conversations(id,organisation_id,name,visibility,created_at) VALUES(?,?,?,'members',?)`, conversationID, organisationID, name, nowText()); err != nil {
		returnServerError(w)
		return
	}
	result, err := tx.ExecContext(r.Context(), `INSERT INTO conversation_members(conversation_id,user_id)
		SELECT ?,user_id FROM organisation_memberships
		WHERE organisation_id=? AND user_id IN (?,?)`, conversationID, organisationID, p.ID, otherUserID)
	if err != nil {
		returnServerError(w)
		return
	}
	insertedMembers, err := result.RowsAffected()
	if err != nil {
		returnServerError(w)
		return
	}
	if insertedMembers != 2 {
		writeError(w, http.StatusBadRequest, "direct conversation requires another organisation member")
		return
	}
	if err = tx.Commit(); err != nil {
		returnServerError(w)
		return
	}
	conversation, err = s.exactDirectConversation(r.Context(), organisationID, p.ID, otherUserID)
	if err != nil {
		returnServerError(w)
		return
	}
	writeJSON(w, http.StatusOK, conversation)
}

func (s *server) exactDirectConversation(ctx context.Context, organisationID, currentUserID, otherUserID string) (map[string]any, error) {
	var id, name, visibility, activityAt, memberIDsJSON string
	var readSequence, latestSequence int64
	var titleAutomatic bool
	err := s.db.QueryRowContext(ctx, `SELECT c.id,coalesce(c.title,c.name),c.visibility,coalesce(crp.sequence,0),coalesce((SELECT max(event.sequence) FROM realtime_events event WHERE event.conversation_id=c.id),0),
		coalesce((SELECT event.occurred_at FROM realtime_events event WHERE event.conversation_id=c.id ORDER BY event.sequence DESC LIMIT 1),c.created_at),c.title_automatic,
		coalesce((SELECT json_group_array(user_id) FROM conversation_members WHERE conversation_id=c.id),'[]')
		FROM conversations c
		LEFT JOIN conversation_read_positions crp ON crp.conversation_id=c.id AND crp.user_id=?
		WHERE c.organisation_id=? AND c.visibility='members'
		AND (SELECT count(*) FROM conversation_members member_count WHERE member_count.conversation_id=c.id)=2
		AND EXISTS(SELECT 1 FROM conversation_members member WHERE member.conversation_id=c.id AND member.user_id=?)
		AND EXISTS(SELECT 1 FROM conversation_members member WHERE member.conversation_id=c.id AND member.user_id=?)
		ORDER BY coalesce((SELECT max(event.sequence) FROM realtime_events event WHERE event.conversation_id=c.id),0) DESC,c.created_at DESC,c.id DESC LIMIT 1`, currentUserID, organisationID, currentUserID, otherUserID).Scan(&id, &name, &visibility, &readSequence, &latestSequence, &activityAt, &titleAutomatic, &memberIDsJSON)
	if err != nil {
		return nil, err
	}
	var memberIDs []string
	_ = json.Unmarshal([]byte(memberIDsJSON), &memberIDs)
	return map[string]any{"id": id, "name": name, "visibility": visibility, "member_ids": memberIDs, "read_sequence": readSequence, "latest_sequence": latestSequence, "activity_at": activityAt, "title_automatic": titleAutomatic}, nil
}

func (s *server) deleteConversation(w http.ResponseWriter, r *http.Request) {
	if current(r).Kind != "human" {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		returnServerError(w)
		return
	}
	defer tx.Rollback()
	storageKeys := []string{}
	if s.attachments != nil {
		rows, err := tx.QueryContext(r.Context(), `SELECT attachment.storage_key FROM attachments attachment JOIN messages message ON message.id=attachment.message_id WHERE message.conversation_id=?`, r.PathValue("conversation"))
		if err != nil {
			returnServerError(w)
			return
		}
		for rows.Next() {
			var storageKey string
			if err := rows.Scan(&storageKey); err != nil {
				rows.Close()
				returnServerError(w)
				return
			}
			storageKeys = append(storageKeys, storageKey)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			returnServerError(w)
			return
		}
		rows.Close()
	}
	result, err := tx.ExecContext(r.Context(), `DELETE FROM conversations
		WHERE id=? AND organisation_id=?
		AND EXISTS(SELECT 1 FROM organisation_memberships
			WHERE organisation_id=? AND user_id=? AND role='admin')`,
		r.PathValue("conversation"), r.PathValue("organisation"), r.PathValue("organisation"), current(r).ID)
	if err != nil {
		returnServerError(w)
		return
	}
	affected, err := result.RowsAffected()
	if err != nil {
		returnServerError(w)
		return
	}
	if affected != 1 {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	if err := tx.Commit(); err != nil {
		returnServerError(w)
		return
	}
	for _, storageKey := range storageKeys {
		if err := s.attachments.Delete(storageKey); err != nil {
			log.Printf("delete attachment object: %v", err)
		}
	}
	s.hub.publishConversationDeleted(r.PathValue("organisation"), r.PathValue("conversation"))
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) eligibleOrganisationUsers(w http.ResponseWriter, r *http.Request) {
	organisationID := r.PathValue("organisation")
	if current(r).Kind != "human" || s.organisationRole(r.Context(), organisationID, current(r).ID) != "admin" {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	email := strings.TrimSpace(r.URL.Query().Get("email"))
	if email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}
	nameRows, err := s.db.QueryContext(r.Context(), `SELECT name_normalized FROM organisation_memberships WHERE organisation_id=?`, organisationID)
	if err != nil {
		returnServerError(w)
		return
	}
	usedNames := map[string]bool{}
	for nameRows.Next() {
		var name string
		if err := nameRows.Scan(&name); err != nil {
			nameRows.Close()
			returnServerError(w)
			return
		}
		usedNames[name] = true
	}
	if err := nameRows.Err(); err != nil {
		nameRows.Close()
		returnServerError(w)
		return
	}
	nameRows.Close()
	rows, err := s.db.QueryContext(r.Context(), `SELECT u.id,u.name,u.email
		FROM users u
		WHERE u.kind='human'
		AND lower(u.email)=lower(?)
		AND NOT EXISTS(SELECT 1 FROM organisation_memberships om WHERE om.organisation_id=? AND om.user_id=u.id)
		ORDER BY u.name,u.id`, email, organisationID)
	if err != nil {
		returnServerError(w)
		return
	}
	defer rows.Close()
	users := make([]map[string]string, 0)
	for rows.Next() {
		var id, name, email string
		if err := rows.Scan(&id, &name, &email); err != nil {
			returnServerError(w)
			return
		}
		if usedNames[database.NormalizeName(name)] {
			continue
		}
		users = append(users, map[string]string{"id": id, "name": name, "email": email})
	}
	if err := rows.Err(); err != nil {
		returnServerError(w)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *server) addOrganisationUser(w http.ResponseWriter, r *http.Request) {
	if current(r).Kind != "human" {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	var in struct {
		UserID string `json:"user_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.UserID = strings.TrimSpace(in.UserID)
	if in.UserID == "" {
		writeError(w, http.StatusBadRequest, "user is required")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		returnServerError(w)
		return
	}
	defer tx.Rollback()
	organisationID := r.PathValue("organisation")
	var allowed int
	if err = tx.QueryRowContext(r.Context(), `SELECT count(*) FROM organisation_memberships WHERE organisation_id=? AND user_id=? AND role='admin'`, organisationID, current(r).ID).Scan(&allowed); err != nil {
		returnServerError(w)
		return
	}
	if allowed != 1 {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	var name string
	if err = tx.QueryRowContext(r.Context(), `SELECT name FROM users WHERE id=? AND kind='human'`, in.UserID).Scan(&name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "human user not found")
			return
		}
		returnServerError(w)
		return
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO organisation_memberships(organisation_id,user_id,role,name_normalized,created_at) VALUES(?,?,'member',?,?)`, organisationID, in.UserID, database.NormalizeName(name), nowText())
	if err != nil {
		if strings.Contains(err.Error(), "organisation_memberships.organisation_id, organisation_memberships.user_id") {
			writeError(w, http.StatusConflict, "user already belongs to organisation")
			return
		}
		if strings.Contains(err.Error(), "organisation_memberships.organisation_id, organisation_memberships.name_normalized") {
			writeError(w, http.StatusConflict, "user name already exists")
			return
		}
		returnServerError(w)
		return
	}
	if err = tx.Commit(); err != nil {
		returnServerError(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": in.UserID, "kind": "human", "name": name, "role": "member"})
}

func (s *server) createBot(w http.ResponseWriter, r *http.Request) {
	if current(r).Kind != "human" || s.organisationRole(r.Context(), r.PathValue("organisation"), current(r).ID) != "admin" {
		writeError(w, 403, "access denied")
		return
	}
	var in struct {
		Name            string   `json:"name"`
		ConversationIDs []string `json:"conversation_ids"`
	}
	if !decode(w, r, &in) {
		return
	}
	botName := database.CanonicalName(in.Name)
	if botName == "" {
		writeError(w, http.StatusBadRequest, "invalid bot name")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "database error")
		return
	}
	defer tx.Rollback()
	key, lookup, hash, err := newAPIKey()
	if err != nil {
		writeError(w, 500, "key generation failed")
		return
	}
	botID := database.NewID("usr")
	now := nowText()
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO users(id,kind,name,created_at) VALUES(?,'bot',?,?)`, botID, botName, now); err != nil {
		returnServerError(w)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO organisation_memberships(organisation_id,user_id,role,name_normalized,created_at) VALUES(?,?,'member',?,?)`, r.PathValue("organisation"), botID, database.NormalizeName(botName), now); err != nil {
		if strings.Contains(err.Error(), "organisation_memberships.organisation_id, organisation_memberships.name_normalized") {
			writeError(w, http.StatusConflict, "user name already exists")
			return
		}
		returnServerError(w)
		return
	}
	seenConversations := make(map[string]bool, len(in.ConversationIDs))
	for _, conversationID := range in.ConversationIDs {
		if seenConversations[conversationID] {
			continue
		}
		seenConversations[conversationID] = true
		result, execErr := tx.ExecContext(r.Context(), `INSERT INTO conversation_members(conversation_id,user_id)
			SELECT c.id,? FROM conversations c
			JOIN organisation_memberships om ON om.organisation_id=c.organisation_id AND om.user_id=?
			WHERE c.id=? AND c.organisation_id=?
			AND (c.visibility='organisation' OR EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=c.id AND cm.user_id=?))`, botID, current(r).ID, conversationID, r.PathValue("organisation"), current(r).ID)
		if execErr != nil {
			returnServerError(w)
			return
		}
		affected, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			returnServerError(w)
			return
		}
		if affected != 1 {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO api_keys(id,user_id,lookup,secret_hash,created_at) VALUES(?,?,?,?,?)`, database.NewID("key"), botID, lookup, hash, now); err != nil {
		returnServerError(w)
		return
	}
	if err = tx.Commit(); err != nil {
		returnServerError(w)
		return
	}
	writeJSON(w, 201, map[string]string{"id": botID, "name": botName, "kind": "bot", "role": "member", "api_key": key})
}

func (s *server) removeBot(w http.ResponseWriter, r *http.Request) {
	if current(r).Kind != "human" {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		returnServerError(w)
		return
	}
	defer tx.Rollback()
	organisationID := r.PathValue("organisation")
	botID := r.PathValue("bot")
	var allowed int
	err = tx.QueryRowContext(r.Context(), `SELECT count(*)
		FROM organisation_memberships target
		JOIN users bot ON bot.id=target.user_id AND bot.kind='bot'
		JOIN organisation_memberships administrator ON administrator.organisation_id=target.organisation_id AND administrator.user_id=? AND administrator.role='admin'
		WHERE target.organisation_id=? AND target.user_id=?`, current(r).ID, organisationID, botID).Scan(&allowed)
	if err != nil {
		returnServerError(w)
		return
	}
	if allowed != 1 {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `DELETE FROM conversation_members WHERE user_id=? AND conversation_id IN (SELECT id FROM conversations WHERE organisation_id=?)`, botID, organisationID); err != nil {
		returnServerError(w)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `DELETE FROM organisation_memberships WHERE organisation_id=? AND user_id=?`, organisationID, botID); err != nil {
		returnServerError(w)
		return
	}
	var remainingMemberships int
	if err = tx.QueryRowContext(r.Context(), `SELECT count(*) FROM organisation_memberships WHERE user_id=?`, botID).Scan(&remainingMemberships); err != nil {
		returnServerError(w)
		return
	}
	if remainingMemberships == 0 {
		if _, err = tx.ExecContext(r.Context(), `DELETE FROM api_keys WHERE user_id=?`, botID); err != nil {
			returnServerError(w)
			return
		}
		if _, err = tx.ExecContext(r.Context(), `DELETE FROM users WHERE id=? AND NOT EXISTS(SELECT 1 FROM messages WHERE author_id=?)`, botID, botID); err != nil {
			returnServerError(w)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		returnServerError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) rotateBotKey(w http.ResponseWriter, r *http.Request) { s.replaceBotKey(w, r, false) }
func (s *server) revokeBotKey(w http.ResponseWriter, r *http.Request) { s.replaceBotKey(w, r, true) }
func (s *server) replaceBotKey(w http.ResponseWriter, r *http.Request, revokeOnly bool) {
	if current(r).Kind != "human" || !s.canManageBot(r.Context(), r.PathValue("bot"), current(r).ID) {
		writeError(w, 403, "access denied")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		returnServerError(w)
		return
	}
	defer tx.Rollback()
	now := nowText()
	if _, err = tx.ExecContext(r.Context(), `UPDATE api_keys SET revoked_at=? WHERE user_id=? AND revoked_at IS NULL`, now, r.PathValue("bot")); err != nil {
		returnServerError(w)
		return
	}
	if revokeOnly {
		if err = tx.Commit(); err != nil {
			returnServerError(w)
			return
		}
		w.WriteHeader(204)
		return
	}
	key, lookup, hash, err := newAPIKey()
	if err != nil {
		returnServerError(w)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO api_keys(id,user_id,lookup,secret_hash,created_at) VALUES(?,?,?,?,?)`, database.NewID("key"), r.PathValue("bot"), lookup, hash, now); err != nil {
		returnServerError(w)
		return
	}
	if err = tx.Commit(); err != nil {
		returnServerError(w)
		return
	}
	writeJSON(w, 201, map[string]string{"api_key": key})
}

type message struct {
	ID             string          `json:"id"`
	ConversationID string          `json:"conversation_id"`
	AuthorID       string          `json:"author_id"`
	AuthorName     string          `json:"author_name"`
	AuthorKind     string          `json:"author_kind"`
	Body           string          `json:"body"`
	ClientID       string          `json:"client_id,omitempty"`
	CreatedAt      string          `json:"created_at"`
	Sequence       int64           `json:"sequence"`
	Mentions       []mentionedUser `json:"mentions"`
	Attachments    []attachment    `json:"attachments"`
}

func (s *server) messages(w http.ResponseWriter, r *http.Request) {
	if !s.canAccess(r.Context(), r.PathValue("conversation"), current(r).ID) {
		writeError(w, 403, "access denied")
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		var err error
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
	}
	before := r.URL.Query().Get("before")
	afterRaw := r.URL.Query().Get("after_sequence")
	if before != "" && afterRaw != "" {
		writeError(w, http.StatusBadRequest, "invalid cursor")
		return
	}
	if before != "" {
		var exists int
		if err := s.db.QueryRowContext(r.Context(), `SELECT count(*) FROM messages WHERE id=? AND conversation_id=?`, before, r.PathValue("conversation")).Scan(&exists); err != nil {
			returnServerError(w)
			return
		}
		if exists != 1 {
			writeError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
	}
	var rows *sql.Rows
	var err error
	if afterRaw != "" {
		afterSequence, parseErr := strconv.ParseInt(afterRaw, 10, 64)
		if parseErr != nil || afterSequence < 0 {
			writeError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		rows, err = s.db.QueryContext(r.Context(), `SELECT m.id,m.conversation_id,m.author_id,u.name,u.kind,m.body,coalesce(m.client_id,''),m.created_at,e.sequence
			FROM messages m JOIN users u ON u.id=m.author_id JOIN realtime_events e ON e.message_id=m.id
			WHERE m.conversation_id=? AND e.sequence>?
			ORDER BY e.sequence LIMIT ?`, r.PathValue("conversation"), afterSequence, limit)
	} else {
		rows, err = s.db.QueryContext(r.Context(), `WITH page AS (
			SELECT m.id,m.conversation_id,m.author_id,u.name AS author_name,u.kind AS author_kind,m.body,coalesce(m.client_id,'') AS client_id,m.created_at,e.sequence
			FROM messages m JOIN users u ON u.id=m.author_id JOIN realtime_events e ON e.message_id=m.id
			WHERE m.conversation_id=? AND (?='' OR (m.created_at,m.id) < (SELECT created_at,id FROM messages WHERE id=? AND conversation_id=m.conversation_id))
			ORDER BY m.created_at DESC,m.id DESC LIMIT ?
		) SELECT id,conversation_id,author_id,author_name,author_kind,body,client_id,created_at,sequence FROM page ORDER BY created_at,id`, r.PathValue("conversation"), before, before, limit)
	}
	if err != nil {
		returnServerError(w)
		return
	}
	defer rows.Close()
	items := []message{}
	for rows.Next() {
		var m message
		if rows.Scan(&m.ID, &m.ConversationID, &m.AuthorID, &m.AuthorName, &m.AuthorKind, &m.Body, &m.ClientID, &m.CreatedAt, &m.Sequence) == nil {
			items = append(items, m)
		}
	}
	rows.Close()
	for index := range items {
		items[index].Mentions, err = s.messageMentions(r.Context(), items[index].ID)
		if err != nil {
			returnServerError(w)
			return
		}
		items[index].Attachments, err = s.messageAttachments(r.Context(), items[index].ID)
		if err != nil {
			returnServerError(w)
			return
		}
	}
	writeJSON(w, 200, items)
}

func (s *server) postMessage(w http.ResponseWriter, r *http.Request) {
	p := current(r)
	conversationID := r.PathValue("conversation")
	if !s.canAccess(r.Context(), conversationID, p.ID) {
		writeError(w, 403, "access denied")
		return
	}
	if !s.allowMessage(p.ID) {
		writeError(w, 429, "rate limit exceeded")
		return
	}
	in, ok := s.decodeMessageInput(w, r)
	if !ok {
		return
	}
	if (strings.TrimSpace(in.Body) == "" && in.Attachment == nil) || len(in.Body) > 20000 || len(in.ClientID) > 200 {
		writeError(w, 400, "invalid message")
		return
	}
	if in.Attachment != nil {
		if s.attachments == nil {
			returnServerError(w)
			return
		}
		if err := s.attachments.Save(in.Attachment.StorageKey, bytes.NewReader(in.Attachment.Bytes)); err != nil {
			returnServerError(w)
			return
		}
	}
	m, created, err := s.insertMessageWithAttachment(r.Context(), conversationID, p, in.Body, in.ClientID, in.Attachment)
	if err != nil {
		if in.Attachment != nil {
			_ = s.attachments.Delete(in.Attachment.StorageKey)
		}
		if errors.Is(err, errMessageAccessDenied) {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		returnServerError(w)
		return
	}
	if !created && in.Attachment != nil {
		_ = s.attachments.Delete(in.Attachment.StorageKey)
	}
	if created {
		s.hub.publish(m.Sequence)
	}
	status := 200
	if created {
		status = 201
	}
	writeJSON(w, status, m)
}

func (s *server) putConversationRead(w http.ResponseWriter, r *http.Request) {
	conversationID := r.PathValue("conversation")
	p := current(r)
	if !s.canAccess(r.Context(), conversationID, p.ID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	var input struct {
		Sequence *int64 `json:"sequence"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.Sequence == nil || *input.Sequence < 0 {
		writeError(w, http.StatusBadRequest, "invalid sequence")
		return
	}
	sequence := *input.Sequence
	if sequence > 0 {
		var exists int
		if err := s.db.QueryRowContext(r.Context(), `SELECT count(*)
			FROM realtime_events event
			JOIN messages message ON message.id=event.message_id
			WHERE event.sequence=? AND message.conversation_id=?`, sequence, conversationID).Scan(&exists); err != nil {
			returnServerError(w)
			return
		}
		if exists != 1 {
			writeError(w, http.StatusBadRequest, "sequence is not in conversation")
			return
		}
	}
	if _, err := s.db.ExecContext(r.Context(), `INSERT INTO conversation_read_positions(user_id,conversation_id,sequence,updated_at)
		VALUES(?,?,?,?)
		ON CONFLICT(user_id,conversation_id) DO UPDATE SET
			sequence=max(sequence,excluded.sequence),
			updated_at=CASE WHEN excluded.sequence>sequence THEN excluded.updated_at ELSE updated_at END`, p.ID, conversationID, sequence, nowText()); err != nil {
		returnServerError(w)
		return
	}
	var persisted int64
	if err := s.db.QueryRowContext(r.Context(), `SELECT sequence FROM conversation_read_positions WHERE user_id=? AND conversation_id=?`, p.ID, conversationID).Scan(&persisted); err != nil {
		returnServerError(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"sequence": persisted})
}

var errMessageAccessDenied = errors.New("message access denied")

func (s *server) insertMessage(ctx context.Context, conversationID string, p principal, body, clientID string) (message, bool, error) {
	return s.insertMessageWithAttachment(ctx, conversationID, p, body, clientID, nil)
}

func (s *server) insertMessageWithAttachment(ctx context.Context, conversationID string, p principal, body, clientID string, pending *pendingAttachment) (message, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return message{}, false, err
	}
	defer tx.Rollback()
	m := message{ID: database.NewID("msg"), ConversationID: conversationID, AuthorID: p.ID, AuthorName: p.Name, AuthorKind: p.Kind, Body: body, ClientID: clientID, CreatedAt: nowText(), Mentions: []mentionedUser{}, Attachments: []attachment{}}
	result, insertErr := tx.ExecContext(ctx, `INSERT INTO messages(id,conversation_id,author_id,body,client_id,created_at)
		SELECT ?,c.id,?,?,nullif(?,''),?
		FROM conversations c
		JOIN organisation_memberships om ON om.organisation_id=c.organisation_id AND om.user_id=?
		WHERE c.id=? AND (c.visibility='organisation' OR (
			EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=c.id AND cm.user_id=?)
			AND (SELECT count(*) FROM conversation_members cm WHERE cm.conversation_id=c.id)>=2))`, m.ID, p.ID, body, clientID, m.CreatedAt, p.ID, conversationID, p.ID)
	if insertErr == nil {
		if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
			return message{}, false, rowsErr
		} else if affected != 1 {
			return message{}, false, errMessageAccessDenied
		}
	} else {
		if clientID != "" {
			var existing message
			e := tx.QueryRowContext(ctx, `SELECT m.id,m.conversation_id,m.author_id,u.name,u.kind,m.body,coalesce(m.client_id,''),m.created_at,e.sequence
				FROM messages m
				JOIN users u ON u.id=m.author_id
				JOIN realtime_events e ON e.message_id=m.id
				JOIN conversations c ON c.id=m.conversation_id
				JOIN organisation_memberships om ON om.organisation_id=c.organisation_id AND om.user_id=?
				WHERE m.conversation_id=? AND m.author_id=? AND m.client_id=?
				AND (c.visibility='organisation' OR (
					EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=c.id AND cm.user_id=?)
					AND (SELECT count(*) FROM conversation_members cm WHERE cm.conversation_id=c.id)>=2))`, p.ID, conversationID, p.ID, clientID, p.ID).Scan(&existing.ID, &existing.ConversationID, &existing.AuthorID, &existing.AuthorName, &existing.AuthorKind, &existing.Body, &existing.ClientID, &existing.CreatedAt, &existing.Sequence)
			if e == nil {
				existing.Mentions, e = messageMentionsInTransaction(ctx, tx, existing.ID)
				if e != nil {
					return message{}, false, e
				}
				existing.Attachments, e = queryMessageAttachments(ctx, tx, existing.ID)
				if e != nil {
					return message{}, false, e
				}
				return existing, false, nil
			}
			if !errors.Is(e, sql.ErrNoRows) {
				return message{}, false, e
			}
			var duplicateExists int
			if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM messages WHERE conversation_id=? AND author_id=? AND client_id=?`, conversationID, p.ID, clientID).Scan(&duplicateExists); e != nil {
				return message{}, false, e
			}
			if duplicateExists == 1 {
				return message{}, false, errMessageAccessDenied
			}
		}
		return message{}, false, insertErr
	}
	if title := strings.TrimSpace(body); title != "" {
		priorBodies, queryErr := tx.QueryContext(ctx, `SELECT body FROM messages WHERE conversation_id=? AND id<>?`, conversationID, m.ID)
		if queryErr != nil {
			return message{}, false, queryErr
		}
		hasPriorText := false
		for priorBodies.Next() {
			var priorBody string
			if scanErr := priorBodies.Scan(&priorBody); scanErr != nil {
				priorBodies.Close()
				return message{}, false, scanErr
			}
			if strings.TrimSpace(priorBody) != "" {
				hasPriorText = true
			}
		}
		if rowsErr := priorBodies.Err(); rowsErr != nil {
			priorBodies.Close()
			return message{}, false, rowsErr
		}
		priorBodies.Close()
		if !hasPriorText {
			if _, err = tx.ExecContext(ctx, `UPDATE conversations SET title=?,title_automatic=0 WHERE id=? AND title_automatic=1`, title, conversationID); err != nil {
				return message{}, false, err
			}
		}
	}
	var orgID string
	if err = tx.QueryRowContext(ctx, `SELECT organisation_id FROM conversations WHERE id=?`, conversationID).Scan(&orgID); err != nil {
		return message{}, false, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT users.id,users.name FROM users
		JOIN organisation_memberships membership ON membership.user_id=users.id
		WHERE membership.organisation_id=?`, orgID)
	if err != nil {
		return message{}, false, err
	}
	var candidates []mentionCandidate
	for rows.Next() {
		var candidate mentionCandidate
		if err = rows.Scan(&candidate.ID, &candidate.Name); err != nil {
			rows.Close()
			return message{}, false, err
		}
		candidates = append(candidates, candidate)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return message{}, false, err
	}
	rows.Close()
	m.Mentions = mentionedUsers(body, candidates)
	for _, mention := range m.Mentions {
		if _, err = tx.ExecContext(ctx, `INSERT INTO message_mentions(message_id,user_id,name) VALUES(?,?,?)`, m.ID, mention.ID, mention.Name); err != nil {
			return message{}, false, err
		}
	}
	if p.Kind == "human" {
		if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO message_bot_deliveries(message_id,user_id)
			SELECT ?,mention.user_id
			FROM message_mentions mention
			JOIN users target ON target.id=mention.user_id AND target.kind='bot'
			JOIN conversations conversation ON conversation.id=?
			WHERE mention.message_id=?
			AND (conversation.visibility='organisation' OR EXISTS(
				SELECT 1 FROM conversation_members member
				WHERE member.conversation_id=conversation.id AND member.user_id=target.id))`, m.ID, conversationID, m.ID); err != nil {
			return message{}, false, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO message_bot_deliveries(message_id,user_id)
			SELECT ?,target_member.user_id
			FROM conversations conversation
			JOIN conversation_members target_member ON target_member.conversation_id=conversation.id AND target_member.user_id<>?
			JOIN users target ON target.id=target_member.user_id AND target.kind='bot'
			WHERE conversation.id=? AND conversation.visibility='members'
			AND (SELECT count(*) FROM conversation_members member_count WHERE member_count.conversation_id=conversation.id)=2
			AND EXISTS(SELECT 1 FROM conversation_members author_member WHERE author_member.conversation_id=conversation.id AND author_member.user_id=?)`, m.ID, p.ID, conversationID, p.ID); err != nil {
			return message{}, false, err
		}
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO realtime_events(id,organisation_id,conversation_id,message_id,occurred_at) VALUES(?,?,?,?,?)`, database.NewID("evt"), orgID, conversationID, m.ID, m.CreatedAt)
	if err != nil {
		return message{}, false, err
	}
	m.Sequence, _ = res.LastInsertId()
	if pending != nil {
		pending.MessageID = m.ID
		pending.CreatedAt = m.CreatedAt
		if _, err = tx.ExecContext(ctx, `INSERT INTO attachments(id,message_id,storage_key,media_type,byte_size,width,height,original_filename,sha256,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, pending.ID, pending.MessageID, pending.StorageKey, pending.MediaType, pending.ByteSize, pending.Width, pending.Height, pending.OriginalFilename, pending.SHA256, pending.CreatedAt); err != nil {
			return message{}, false, err
		}
		m.Attachments = []attachment{pending.attachment}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO conversation_read_positions(user_id,conversation_id,sequence,updated_at)
		VALUES(?,?,?,?)
		ON CONFLICT(user_id,conversation_id) DO UPDATE SET sequence=max(sequence,excluded.sequence),updated_at=excluded.updated_at`, p.ID, conversationID, m.Sequence, m.CreatedAt); err != nil {
		return message{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return message{}, false, err
	}
	return m, true, nil
}

func (s *server) messageByClient(ctx context.Context, conversationID, authorID, clientID string) (message, error) {
	var m message
	err := s.db.QueryRowContext(ctx, `SELECT m.id,m.conversation_id,m.author_id,u.name,u.kind,m.body,coalesce(m.client_id,''),m.created_at,e.sequence FROM messages m JOIN users u ON u.id=m.author_id JOIN realtime_events e ON e.message_id=m.id WHERE m.conversation_id=? AND m.author_id=? AND m.client_id=?`, conversationID, authorID, clientID).Scan(&m.ID, &m.ConversationID, &m.AuthorID, &m.AuthorName, &m.AuthorKind, &m.Body, &m.ClientID, &m.CreatedAt, &m.Sequence)
	if err == nil {
		m.Mentions, err = s.messageMentions(ctx, m.ID)
	}
	if err == nil {
		m.Attachments, err = s.messageAttachments(ctx, m.ID)
	}
	return m, err
}

func (s *server) messageMentions(ctx context.Context, messageID string) ([]mentionedUser, error) {
	return queryMessageMentions(ctx, s.db, messageID)
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func messageMentionsInTransaction(ctx context.Context, tx *sql.Tx, messageID string) ([]mentionedUser, error) {
	return queryMessageMentions(ctx, tx, messageID)
}

func queryMessageMentions(ctx context.Context, queryable queryer, messageID string) ([]mentionedUser, error) {
	rows, err := queryable.QueryContext(ctx, `SELECT user_id,name FROM message_mentions WHERE message_id=? ORDER BY rowid`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	mentions := []mentionedUser{}
	for rows.Next() {
		var mention mentionedUser
		if err := rows.Scan(&mention.ID, &mention.Name); err != nil {
			return nil, err
		}
		mentions = append(mentions, mention)
	}
	return mentions, rows.Err()
}

func (s *server) webSocket(w http.ResponseWriter, r *http.Request) {
	p := current(r)
	after := int64(0)
	_, _ = fmt.Sscan(r.URL.Query().Get("after"), &after)
	subscriptionID, sub, cancelSubscription := s.hub.subscribe()
	defer cancelSubscription()
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer c.CloseNow()
	ctx := c.CloseRead(r.Context())
	send := func(seq int64) error {
		m, err := s.eventFor(ctx, seq, p)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return wsjson.Write(writeCtx, c, map[string]any{"version": 1, "type": "message.created", "sequence": m.Sequence, "payload": m})
	}
	lastDelivered := after
	replay := func() error {
		sequences, err := s.eligibleEventSequences(ctx, p, lastDelivered)
		if err != nil {
			return err
		}
		for _, seq := range sequences {
			if err := send(seq); err != nil {
				return err
			}
			lastDelivered = seq
		}
		return nil
	}
	if err := replay(); err != nil {
		_ = c.Close(websocket.StatusInternalError, "replay failed")
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-sub:
			for organisationID, conversationIDs := range s.hub.takeChangedConversations(subscriptionID) {
				if s.organisationMember(ctx, organisationID, p.ID) {
					for conversationID := range conversationIDs {
						writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
						err := wsjson.Write(writeCtx, c, map[string]any{"version": 1, "type": "conversation.deleted", "payload": map[string]string{"id": conversationID}})
						cancel()
						if err != nil {
							return
						}
					}
				}
			}
			if err := replay(); err != nil {
				return
			}
		}
	}
}

const botDeliveryCondition = `(target.kind!='bot' OR (author.kind='human' AND EXISTS(
	SELECT 1 FROM message_bot_deliveries delivery WHERE delivery.message_id=m.id AND delivery.user_id=target.id
)))`

func (s *server) eligibleEventSequences(ctx context.Context, target principal, after int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT e.sequence FROM realtime_events e
		JOIN messages m ON m.id=e.message_id
		JOIN users author ON author.id=m.author_id
		JOIN conversations c ON c.id=e.conversation_id
		JOIN organisation_memberships om ON om.organisation_id=c.organisation_id AND om.user_id=?
		JOIN users target ON target.id=om.user_id
		WHERE e.sequence>? AND (c.visibility='organisation' OR EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=c.id AND cm.user_id=target.id))
		AND `+botDeliveryCondition+` ORDER BY e.sequence`, target.ID, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sequences []int64
	for rows.Next() {
		var sequence int64
		if err := rows.Scan(&sequence); err != nil {
			return nil, err
		}
		sequences = append(sequences, sequence)
	}
	return sequences, rows.Err()
}

func (s *server) eventFor(ctx context.Context, seq int64, target principal) (message, error) {
	var m message
	err := s.db.QueryRowContext(ctx, `SELECT m.id,m.conversation_id,m.author_id,author.name,author.kind,m.body,coalesce(m.client_id,''),m.created_at,e.sequence
		FROM realtime_events e JOIN messages m ON m.id=e.message_id JOIN users author ON author.id=m.author_id
		JOIN conversations c ON c.id=m.conversation_id
		JOIN organisation_memberships om ON om.organisation_id=c.organisation_id AND om.user_id=?
		JOIN users target ON target.id=om.user_id
		WHERE e.sequence=? AND (c.visibility='organisation' OR EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=c.id AND cm.user_id=target.id))
		AND `+botDeliveryCondition, target.ID, seq).Scan(&m.ID, &m.ConversationID, &m.AuthorID, &m.AuthorName, &m.AuthorKind, &m.Body, &m.ClientID, &m.CreatedAt, &m.Sequence)
	if err == nil {
		m.Mentions, err = s.messageMentions(ctx, m.ID)
	}
	if err == nil {
		m.Attachments, err = s.messageAttachments(ctx, m.ID)
	}
	return m, err
}

func (s *server) organisationMember(ctx context.Context, org, user string) bool {
	var n int
	return s.db.QueryRowContext(ctx, `SELECT count(*) FROM organisation_memberships WHERE organisation_id=? AND user_id=?`, org, user).Scan(&n) == nil && n == 1
}
func (s *server) organisationRole(ctx context.Context, org, user string) string {
	var role string
	_ = s.db.QueryRowContext(ctx, `SELECT role FROM organisation_memberships WHERE organisation_id=? AND user_id=?`, org, user).Scan(&role)
	return role
}
func (s *server) canAccess(ctx context.Context, conversation, user string) bool {
	var n int
	return s.db.QueryRowContext(ctx, `SELECT count(*) FROM conversations c JOIN organisation_memberships om ON om.organisation_id=c.organisation_id AND om.user_id=? WHERE c.id=? AND (c.visibility='organisation' OR EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=c.id AND cm.user_id=?))`, user, conversation, user).Scan(&n) == nil && n == 1
}
func (s *server) canManageBot(ctx context.Context, bot, human string) bool {
	var n int
	return s.db.QueryRowContext(ctx, `SELECT count(*) FROM organisation_memberships b JOIN users u ON u.id=b.user_id AND u.kind='bot' JOIN organisation_memberships h ON h.organisation_id=b.organisation_id AND h.role='admin' WHERE b.user_id=? AND h.user_id=?`, bot, human).Scan(&n) == nil && n > 0
}
func (s *server) allowMessage(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	cut := time.Now().Add(-time.Second)
	kept := s.lastMessage[id][:0]
	for _, at := range s.lastMessage[id] {
		if at.After(cut) {
			kept = append(kept, at)
		}
	}
	if len(kept) >= 20 {
		s.lastMessage[id] = kept
		return false
	}
	s.lastMessage[id] = append(kept, time.Now())
	return true
}

type loginLimiter struct {
	mu         sync.Mutex
	attempts   map[string][]time.Time
	limit      int
	window     time.Duration
	maxEntries int
}

func newLoginLimiter(limit int, window time.Duration, maxEntries int) *loginLimiter {
	return &loginLimiter{attempts: make(map[string][]time.Time), limit: limit, window: window, maxEntries: maxEntries}
}

func (l *loginLimiter) allow(email string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cut := now.Add(-l.window)
	for key, attempts := range l.attempts {
		kept := attempts[:0]
		for _, at := range attempts {
			if at.After(cut) {
				kept = append(kept, at)
			}
		}
		if len(kept) == 0 {
			delete(l.attempts, key)
		} else {
			l.attempts[key] = kept
		}
	}
	key := strings.ToLower(strings.TrimSpace(email))
	if attempts := l.attempts[key]; len(attempts) >= l.limit {
		return false
	}
	if _, exists := l.attempts[key]; !exists && len(l.attempts) >= l.maxEntries {
		var oldestKey string
		var oldest time.Time
		for candidate, attempts := range l.attempts {
			last := attempts[len(attempts)-1]
			if oldestKey == "" || last.Before(oldest) {
				oldestKey, oldest = candidate, last
			}
		}
		delete(l.attempts, oldestKey)
	}
	l.attempts[key] = append(l.attempts[key], now)
	return true
}

func (l *loginLimiter) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.attempts)
}

type hub struct {
	mu   sync.Mutex
	next int
	subs map[int]*hubSubscriber
}

type hubSubscriber struct {
	wake                 chan struct{}
	changedConversations map[string]map[string]struct{}
}

func newHub() *hub { return &hub{subs: map[int]*hubSubscriber{}} }
func (h *hub) subscribe() (int, <-chan struct{}, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	id := h.next
	h.next++
	subscriber := &hubSubscriber{wake: make(chan struct{}, 64), changedConversations: map[string]map[string]struct{}{}}
	h.subs[id] = subscriber
	return id, subscriber.wake, func() { h.mu.Lock(); delete(h.subs, id); h.mu.Unlock() }
}
func (h *hub) publish(_ int64) {
	h.publishEvent("", "")
}
func (h *hub) publishConversationDeleted(organisationID, conversationID string) {
	h.publishEvent(organisationID, conversationID)
}
func (h *hub) publishEvent(organisationID, conversationID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, subscriber := range h.subs {
		if organisationID != "" {
			if subscriber.changedConversations[organisationID] == nil {
				subscriber.changedConversations[organisationID] = map[string]struct{}{}
			}
			subscriber.changedConversations[organisationID][conversationID] = struct{}{}
		}
		select {
		case subscriber.wake <- struct{}{}:
		default:
		}
	}
}
func (h *hub) takeChangedConversations(subscriptionID int) map[string]map[string]struct{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	subscriber := h.subs[subscriptionID]
	if subscriber == nil || len(subscriber.changedConversations) == 0 {
		return nil
	}
	changed := subscriber.changedConversations
	subscriber.changedConversations = map[string]map[string]struct{}{}
	return changed
}

func newAPIKey() (string, string, string, error) {
	lookup := hex.EncodeToString(randomBytes(8))
	secret := base64.RawURLEncoding.EncodeToString(randomBytes(32))
	return "km_live_" + lookup + "_" + secret, lookup, auth.HashSecret(secret), nil
}
func randomToken(n int) string { return base64.RawURLEncoding.EncodeToString(randomBytes(n)) }
func randomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}
func current(r *http.Request) principal { return r.Context().Value(contextKey{}).(principal) }
func nowText() string                   { return time.Now().UTC().Format(time.RFC3339Nano) }
func isMutation(method string) bool {
	return method != "GET" && method != "HEAD" && method != "OPTIONS"
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		writeError(w, 400, "invalid JSON")
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func returnServerError(w http.ResponseWriter) { writeError(w, 500, "database error") }
