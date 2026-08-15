package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"kmainstay/internal/auth"
	"kmainstay/internal/database"
)

var ErrAlreadyBootstrapped = errors.New("application is already bootstrapped")

type BootstrapResult struct {
	UserID, OrganisationID, ConversationID string
}

func Bootstrap(ctx context.Context, db *sql.DB, email, name, password, organisation string) (BootstrapResult, error) {
	var out BootstrapResult
	email, name, organisation = strings.TrimSpace(strings.ToLower(email)), strings.TrimSpace(name), strings.TrimSpace(organisation)
	if email == "" || name == "" || organisation == "" {
		return out, fmt.Errorf("email, name, and organisation are required")
	}
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return out, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&count); err != nil {
		return out, err
	}
	if count != 0 {
		return out, ErrAlreadyBootstrapped
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	out = BootstrapResult{database.NewID("usr"), database.NewID("org"), database.NewID("con")}
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO organisations(id,name,created_at) VALUES(?,?,?)`, []any{out.OrganisationID, organisation, now}},
		{`INSERT INTO users(id,kind,email,name,password_hash,created_at) VALUES(?,'human',?,?,?,?)`, []any{out.UserID, email, name, passwordHash, now}},
		{`INSERT INTO organisation_memberships(organisation_id,user_id,role,name_normalized,created_at) VALUES(?,?,'admin',?,?)`, []any{out.OrganisationID, out.UserID, database.NormalizeName(name), now}},
		{`INSERT INTO conversations(id,organisation_id,name,visibility,created_at) VALUES(?,?,?,'organisation',?)`, []any{out.ConversationID, out.OrganisationID, "general", now}},
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement.query, statement.args...); err != nil {
			return BootstrapResult{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return BootstrapResult{}, err
	}
	return out, nil
}
