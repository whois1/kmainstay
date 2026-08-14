package app_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"kmainstay/internal/app"
	"kmainstay/internal/database"
)

func TestBootstrap_CreatesInitialHumanOrganisationAndGeneralExactlyOnce(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	got, err := app.Bootstrap(context.Background(), db, "MICHAEL@example.com", "Michael", "long-enough-password", "Mainstay")
	if err != nil {
		t.Fatal(err)
	}
	var kind, email, conversation string
	if err := db.QueryRow(`SELECT u.kind,u.email,c.name FROM users u JOIN organisation_memberships m ON m.user_id=u.id JOIN conversations c ON c.organisation_id=m.organisation_id WHERE u.id=?`, got.UserID).Scan(&kind, &email, &conversation); err != nil {
		t.Fatal(err)
	}
	if kind != "human" || email != "michael@example.com" || conversation != "general" {
		t.Fatalf("got %q %q %q", kind, email, conversation)
	}
	if _, err := app.Bootstrap(context.Background(), db, "other@example.com", "Other", "long-enough-password", "Other"); !errors.Is(err, app.ErrAlreadyBootstrapped) {
		t.Fatalf("second bootstrap error = %v", err)
	}
}
