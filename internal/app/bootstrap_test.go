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
	var kind, email, conversation, role, normalisedName string
	if err := db.QueryRow(`SELECT u.kind,u.email,c.name,m.role,m.name_normalized FROM users u JOIN organisation_memberships m ON m.user_id=u.id JOIN conversations c ON c.organisation_id=m.organisation_id WHERE u.id=?`, got.UserID).Scan(&kind, &email, &conversation, &role, &normalisedName); err != nil {
		t.Fatal(err)
	}
	if kind != "human" || email != "michael@example.com" || conversation != "general" || role != "admin" || normalisedName != "michael" {
		t.Fatalf("got %q %q %q %q %q", kind, email, conversation, role, normalisedName)
	}
	if _, err := app.Bootstrap(context.Background(), db, "other@example.com", "Other", "long-enough-password", "Other"); !errors.Is(err, app.ErrAlreadyBootstrapped) {
		t.Fatalf("second bootstrap error = %v", err)
	}
}
