package app_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"kmainstay/internal/app"
	"kmainstay/internal/database"
)

func TestBootstrap_CreatesInitialHumanOrganisationAndEveryoneExactlyOnce(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	got, err := app.Bootstrap(context.Background(), db, "MICHAEL@example.com", "Michaél", "long-enough-password", "Mainstay")
	if err != nil {
		t.Fatal(err)
	}
	var kind, email, name, conversation, role, normalisedName string
	var isEveryone bool
	if err := db.QueryRow(`SELECT u.kind,u.email,u.name,coalesce(c.title,c.name),m.role,m.name_normalized,c.is_everyone FROM users u JOIN organisation_memberships m ON m.user_id=u.id JOIN conversations c ON c.organisation_id=m.organisation_id WHERE u.id=?`, got.UserID).Scan(&kind, &email, &name, &conversation, &role, &normalisedName, &isEveryone); err != nil {
		t.Fatal(err)
	}
	if kind != "human" || email != "michael@example.com" || name != "Michaél" || conversation != "Everyone" || role != "admin" || normalisedName != "michaél" || !isEveryone {
		t.Fatalf("got %q %q %q %q %q %q everyone=%v", kind, email, name, conversation, role, normalisedName, isEveryone)
	}
	if _, err := app.Bootstrap(context.Background(), db, "other@example.com", "Other", "long-enough-password", "Other"); !errors.Is(err, app.ErrAlreadyBootstrapped) {
		t.Fatalf("second bootstrap error = %v", err)
	}
}
