package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"kmainstay/internal/auth"
	"kmainstay/internal/database"
)

type Dependencies struct {
	DB             *sql.DB
	SecureCookies  bool
	AllowedOrigins []string
	Assets         http.Handler
}

type server struct {
	db             *sql.DB
	secureCookies  bool
	allowedOrigins map[string]bool
	hub            *hub
	mu             sync.Mutex
	lastMessage    map[string][]time.Time
	loginLimiter   *loginLimiter
}

type principal struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type contextKey struct{}

func New(deps Dependencies) http.Handler {
	s := &server{db: deps.DB, secureCookies: deps.SecureCookies, allowedOrigins: map[string]bool{}, hub: newHub(), lastMessage: map[string][]time.Time{}, loginLimiter: newLoginLimiter(5, time.Minute, 1024)}
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
	rows, err := s.db.QueryContext(r.Context(), `SELECT c.id,c.name,c.visibility,coalesce(crp.sequence,0),coalesce((SELECT max(event.sequence) FROM realtime_events event WHERE event.conversation_id=c.id),0)
		FROM conversations c
		JOIN organisation_memberships om ON om.organisation_id=c.organisation_id
		LEFT JOIN conversation_read_positions crp ON crp.conversation_id=c.id AND crp.user_id=om.user_id
		WHERE c.organisation_id=? AND om.user_id=? AND (c.visibility='organisation' OR EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=c.id AND cm.user_id=?))
		ORDER BY c.created_at,c.id`, r.PathValue("organisation"), current(r).ID, current(r).ID)
	if err != nil {
		writeError(w, 500, "database error")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, name, visibility string
		var readSequence, latestSequence int64
		if rows.Scan(&id, &name, &visibility, &readSequence, &latestSequence) == nil {
			items = append(items, map[string]any{"id": id, "name": name, "visibility": visibility, "read_sequence": readSequence, "latest_sequence": latestSequence})
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
		Name       string   `json:"name"`
		Visibility string   `json:"visibility"`
		MemberIDs  []string `json:"member_ids"`
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
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO conversations(id,organisation_id,name,visibility,created_at) VALUES(?,?,?,?,?)`, id, orgID, in.Name, in.Visibility, now); err != nil {
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
	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "name": in.Name, "visibility": in.Visibility})
}

func (s *server) deleteConversation(w http.ResponseWriter, r *http.Request) {
	if current(r).Kind != "human" {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	result, err := s.db.ExecContext(r.Context(), `DELETE FROM conversations
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
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	AuthorID       string `json:"author_id"`
	AuthorName     string `json:"author_name"`
	AuthorKind     string `json:"author_kind"`
	Body           string `json:"body"`
	ClientID       string `json:"client_id,omitempty"`
	CreatedAt      string `json:"created_at"`
	Sequence       int64  `json:"sequence"`
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
	var in struct {
		Body     string `json:"body"`
		ClientID string `json:"client_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Body) == "" || len(in.Body) > 20000 || len(in.ClientID) > 200 {
		writeError(w, 400, "invalid message")
		return
	}
	m, created, err := s.insertMessage(r.Context(), conversationID, p, in.Body, in.ClientID)
	if err != nil {
		if errors.Is(err, errMessageAccessDenied) {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		returnServerError(w)
		return
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return message{}, false, err
	}
	defer tx.Rollback()
	m := message{ID: database.NewID("msg"), ConversationID: conversationID, AuthorID: p.ID, AuthorName: p.Name, AuthorKind: p.Kind, Body: body, ClientID: clientID, CreatedAt: nowText()}
	result, insertErr := tx.ExecContext(ctx, `INSERT INTO messages(id,conversation_id,author_id,body,client_id,created_at)
		SELECT ?,c.id,?,?,nullif(?,''),?
		FROM conversations c
		JOIN organisation_memberships om ON om.organisation_id=c.organisation_id AND om.user_id=?
		WHERE c.id=? AND (c.visibility='organisation' OR EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=c.id AND cm.user_id=?))`, m.ID, p.ID, body, clientID, m.CreatedAt, p.ID, conversationID, p.ID)
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
				AND (c.visibility='organisation' OR EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=c.id AND cm.user_id=?))`, p.ID, conversationID, p.ID, clientID, p.ID).Scan(&existing.ID, &existing.ConversationID, &existing.AuthorID, &existing.AuthorName, &existing.AuthorKind, &existing.Body, &existing.ClientID, &existing.CreatedAt, &existing.Sequence)
			if e == nil {
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
	var orgID string
	if err = tx.QueryRowContext(ctx, `SELECT organisation_id FROM conversations WHERE id=?`, conversationID).Scan(&orgID); err != nil {
		return message{}, false, err
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO realtime_events(id,organisation_id,conversation_id,message_id,occurred_at) VALUES(?,?,?,?,?)`, database.NewID("evt"), orgID, conversationID, m.ID, m.CreatedAt)
	if err != nil {
		return message{}, false, err
	}
	m.Sequence, _ = res.LastInsertId()
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
	return m, err
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
		m, err := s.eventFor(ctx, seq, p.ID)
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
		rows, err := s.db.QueryContext(ctx, `SELECT e.sequence FROM realtime_events e JOIN conversations c ON c.id=e.conversation_id JOIN organisation_memberships om ON om.organisation_id=c.organisation_id AND om.user_id=? WHERE e.sequence>? AND (c.visibility='organisation' OR EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=c.id AND cm.user_id=?)) ORDER BY e.sequence`, p.ID, lastDelivered, p.ID)
		if err != nil {
			return err
		}
		var sequences []int64
		for rows.Next() {
			var seq int64
			if err := rows.Scan(&seq); err != nil {
				rows.Close()
				return err
			}
			sequences = append(sequences, seq)
		}
		err = rows.Err()
		rows.Close()
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

func (s *server) eventFor(ctx context.Context, seq int64, userID string) (message, error) {
	var m message
	err := s.db.QueryRowContext(ctx, `SELECT m.id,m.conversation_id,m.author_id,u.name,u.kind,m.body,coalesce(m.client_id,''),m.created_at,e.sequence FROM realtime_events e JOIN messages m ON m.id=e.message_id JOIN users u ON u.id=m.author_id JOIN conversations c ON c.id=m.conversation_id JOIN organisation_memberships om ON om.organisation_id=c.organisation_id AND om.user_id=? WHERE e.sequence=? AND (c.visibility='organisation' OR EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=c.id AND cm.user_id=?))`, userID, seq, userID).Scan(&m.ID, &m.ConversationID, &m.AuthorID, &m.AuthorName, &m.AuthorKind, &m.Body, &m.ClientID, &m.CreatedAt, &m.Sequence)
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
