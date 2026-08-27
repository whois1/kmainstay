package database

import (
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/text/unicode/norm"
	_ "modernc.org/sqlite"
)

//go:embed migrations/001_initial.sql
var initialMigration string

//go:embed migrations/002_membership_roles.sql
var membershipRolesMigration string

//go:embed migrations/003_canonical_names.sql
var canonicalNamesMigration string

//go:embed migrations/004_conversation_read_positions.sql
var conversationReadPositionsMigration string

//go:embed migrations/005_message_mentions.sql
var messageMentionsMigration string

//go:embed migrations/006_message_attachments.sql
var messageAttachmentsMigration string

//go:embed migrations/006_relax_message_body.sql
var relaxMessageBodyMigration string

//go:embed migrations/007_conversation_titles.sql
var conversationTitlesMigration string

//go:embed migrations/008_message_replies.sql
var messageRepliesMigration string

//go:embed migrations/009_conversation_archives.sql
var conversationArchivesMigration string

//go:embed migrations/010_attachment_positions.sql
var attachmentPositionsMigration string

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return err
	}
	var count int
	if err = tx.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version=1`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if _, err = tx.Exec(initialMigration); err != nil {
			return fmt.Errorf("migration 1: %w", err)
		}
		if _, err = tx.Exec(`INSERT INTO schema_migrations(version,applied_at) VALUES(1,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`); err != nil {
			return err
		}
	}
	if err = tx.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version=2`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if _, err = tx.Exec(membershipRolesMigration); err != nil {
			return fmt.Errorf("migration 2: %w", err)
		}
		if err = migrateMembershipRoles(tx); err != nil {
			return fmt.Errorf("migration 2 memberships: %w", err)
		}
		if _, err = tx.Exec(`INSERT INTO schema_migrations(version,applied_at) VALUES(2,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`); err != nil {
			return err
		}
	}
	if err = tx.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version=3`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if _, err = tx.Exec(canonicalNamesMigration); err != nil {
			return fmt.Errorf("migration 3: %w", err)
		}
		if err = migrateCanonicalNames(tx); err != nil {
			return fmt.Errorf("migration 3 names: %w", err)
		}
		if _, err = tx.Exec(`INSERT INTO schema_migrations(version,applied_at) VALUES(3,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`); err != nil {
			return err
		}
	}
	if err = tx.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version=4`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if _, err = tx.Exec(conversationReadPositionsMigration); err != nil {
			return fmt.Errorf("migration 4: %w", err)
		}
		if _, err = tx.Exec(`INSERT INTO schema_migrations(version,applied_at) VALUES(4,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`); err != nil {
			return err
		}
	}
	if err = tx.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version=5`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if _, err = tx.Exec(messageMentionsMigration); err != nil {
			return fmt.Errorf("migration 5: %w", err)
		}
		if _, err = tx.Exec(`INSERT INTO schema_migrations(version,applied_at) VALUES(5,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`); err != nil {
			return err
		}
	}
	if err = tx.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version=6`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		var messagesTableCount int
		if err = tx.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='messages'`).Scan(&messagesTableCount); err != nil {
			return err
		}
		if messagesTableCount == 1 {
			if _, err = tx.Exec(relaxMessageBodyMigration); err != nil {
				return fmt.Errorf("migration 6 message body: %w", err)
			}
		}
		if _, err = tx.Exec(messageAttachmentsMigration); err != nil {
			return fmt.Errorf("migration 6: %w", err)
		}
		if _, err = tx.Exec(`INSERT INTO schema_migrations(version,applied_at) VALUES(6,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`); err != nil {
			return err
		}
	}
	if err = tx.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version=7`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if _, err = tx.Exec(conversationTitlesMigration); err != nil {
			return fmt.Errorf("migration 7: %w", err)
		}
		var conversationNameColumnCount int
		if err = tx.QueryRow(`SELECT count(*) FROM pragma_table_info('conversations') WHERE name='name'`).Scan(&conversationNameColumnCount); err != nil {
			return fmt.Errorf("migration 7 conversation columns: %w", err)
		}
		if conversationNameColumnCount == 1 {
			if _, err = tx.Exec(`UPDATE conversations SET title=name`); err != nil {
				return fmt.Errorf("migration 7 conversation titles: %w", err)
			}
		}
		if _, err = tx.Exec(`INSERT INTO schema_migrations(version,applied_at) VALUES(7,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`); err != nil {
			return err
		}
	}
	if err = tx.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version=8`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		var messagesTableCount int
		if err = tx.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='messages'`).Scan(&messagesTableCount); err != nil {
			return err
		}
		if messagesTableCount == 1 {
			if _, err = tx.Exec(messageRepliesMigration); err != nil {
				return fmt.Errorf("migration 8: %w", err)
			}
		}
		if _, err = tx.Exec(`INSERT INTO schema_migrations(version,applied_at) VALUES(8,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`); err != nil {
			return err
		}
	}
	if err = tx.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version=9`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if _, err = tx.Exec(conversationArchivesMigration); err != nil {
			return fmt.Errorf("migration 9: %w", err)
		}
		if _, err = tx.Exec(`INSERT INTO schema_migrations(version,applied_at) VALUES(9,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`); err != nil {
			return err
		}
	}
	if err = tx.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version=10`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if _, err = tx.Exec(attachmentPositionsMigration); err != nil {
			return fmt.Errorf("migration 10: %w", err)
		}
		if _, err = tx.Exec(`INSERT INTO schema_migrations(version,applied_at) VALUES(10,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type legacyMembership struct {
	OrganisationID string
	UserID         string
	Kind           string
	Name           string
	CreatedAt      string
}

type legacyUser struct {
	ID       string
	Kind     string
	Name     string
	Earliest string
	Orgs     map[string]string
}

func migrateMembershipRoles(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT m.organisation_id,m.user_id,u.kind,u.name,m.created_at FROM organisation_memberships_legacy m JOIN users u ON u.id=m.user_id ORDER BY m.created_at,m.user_id,m.organisation_id`)
	if err != nil {
		return err
	}
	var memberships []legacyMembership
	for rows.Next() {
		var membership legacyMembership
		if err := rows.Scan(&membership.OrganisationID, &membership.UserID, &membership.Kind, &membership.Name, &membership.CreatedAt); err != nil {
			rows.Close()
			return err
		}
		memberships = append(memberships, membership)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	users := map[string]*legacyUser{}
	admins := map[string]legacyMembership{}
	for _, membership := range memberships {
		user := users[membership.UserID]
		if user == nil {
			user = &legacyUser{ID: membership.UserID, Kind: membership.Kind, Name: membership.Name, Earliest: membership.CreatedAt, Orgs: map[string]string{}}
			users[membership.UserID] = user
		}
		user.Orgs[membership.OrganisationID] = membership.CreatedAt
		if membership.CreatedAt < user.Earliest {
			user.Earliest = membership.CreatedAt
		}
		admin, exists := admins[membership.OrganisationID]
		if membership.Kind == "human" && (!exists || membership.CreatedAt < admin.CreatedAt || (membership.CreatedAt == admin.CreatedAt && membership.UserID < admin.UserID)) {
			admins[membership.OrganisationID] = membership
		}
	}
	orderedUsers := make([]*legacyUser, 0, len(users))
	for _, user := range users {
		orderedUsers = append(orderedUsers, user)
	}
	sort.Slice(orderedUsers, func(i, j int) bool {
		if orderedUsers[i].Earliest == orderedUsers[j].Earliest {
			return orderedUsers[i].ID < orderedUsers[j].ID
		}
		return orderedUsers[i].Earliest < orderedUsers[j].Earliest
	})
	usedNames := map[string]map[string]bool{}
	for _, user := range orderedUsers {
		base := strings.TrimSpace(user.Name)
		if base == "" {
			return fmt.Errorf("user %s has an empty name", user.ID)
		}
		candidate := base
		for suffix := 2; ; suffix++ {
			normalized := NormalizeName(candidate)
			conflict := false
			for organisationID := range user.Orgs {
				if usedNames[organisationID][normalized] {
					conflict = true
					break
				}
			}
			if !conflict {
				break
			}
			candidate = fmt.Sprintf("%s %d", base, suffix)
		}
		normalized := NormalizeName(candidate)
		if _, err := tx.Exec(`UPDATE users SET name=? WHERE id=?`, candidate, user.ID); err != nil {
			return err
		}
		organisationIDs := make([]string, 0, len(user.Orgs))
		for organisationID := range user.Orgs {
			organisationIDs = append(organisationIDs, organisationID)
		}
		sort.Strings(organisationIDs)
		for _, organisationID := range organisationIDs {
			if usedNames[organisationID] == nil {
				usedNames[organisationID] = map[string]bool{}
			}
			usedNames[organisationID][normalized] = true
			role := "member"
			if admin, exists := admins[organisationID]; exists && admin.UserID == user.ID {
				role = "admin"
			}
			if _, err := tx.Exec(`INSERT INTO organisation_memberships(organisation_id,user_id,role,name_normalized,created_at) VALUES(?,?,?,?,?)`, organisationID, user.ID, role, normalized, user.Orgs[organisationID]); err != nil {
				return err
			}
		}
	}
	_, err = tx.Exec(`DROP TABLE organisation_memberships_legacy`)
	return err
}

type canonicalMembership struct {
	OrganisationID string
	Role           string
	CreatedAt      string
}

type canonicalUser struct {
	ID          string
	Name        string
	Earliest    string
	Memberships map[string]canonicalMembership
}

func migrateCanonicalNames(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT m.organisation_id,m.user_id,m.role,u.name,m.created_at
		FROM organisation_memberships_canonical_legacy m
		JOIN users u ON u.id=m.user_id
		ORDER BY m.created_at,m.user_id,m.organisation_id`)
	if err != nil {
		return err
	}
	users := map[string]*canonicalUser{}
	for rows.Next() {
		var organisationID, userID, role, name, createdAt string
		if err := rows.Scan(&organisationID, &userID, &role, &name, &createdAt); err != nil {
			rows.Close()
			return err
		}
		user := users[userID]
		if user == nil {
			user = &canonicalUser{ID: userID, Name: name, Earliest: createdAt, Memberships: map[string]canonicalMembership{}}
			users[userID] = user
		}
		user.Memberships[organisationID] = canonicalMembership{OrganisationID: organisationID, Role: role, CreatedAt: createdAt}
		if createdAt < user.Earliest {
			user.Earliest = createdAt
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	orderedUsers := make([]*canonicalUser, 0, len(users))
	for _, user := range users {
		orderedUsers = append(orderedUsers, user)
	}
	sort.Slice(orderedUsers, func(i, j int) bool {
		if orderedUsers[i].Earliest == orderedUsers[j].Earliest {
			return orderedUsers[i].ID < orderedUsers[j].ID
		}
		return orderedUsers[i].Earliest < orderedUsers[j].Earliest
	})
	usedNames := map[string]map[string]bool{}
	for _, user := range orderedUsers {
		base := norm.NFC.String(strings.TrimSpace(user.Name))
		if base == "" {
			return fmt.Errorf("user %s has an empty name", user.ID)
		}
		candidate := base
		for suffix := 2; ; suffix++ {
			normalized := NormalizeName(candidate)
			conflict := false
			for organisationID := range user.Memberships {
				if usedNames[organisationID][normalized] {
					conflict = true
					break
				}
			}
			if !conflict {
				break
			}
			candidate = fmt.Sprintf("%s %d", base, suffix)
		}
		normalized := NormalizeName(candidate)
		if _, err := tx.Exec(`UPDATE users SET name=? WHERE id=?`, candidate, user.ID); err != nil {
			return err
		}
		organisationIDs := make([]string, 0, len(user.Memberships))
		for organisationID := range user.Memberships {
			organisationIDs = append(organisationIDs, organisationID)
		}
		sort.Strings(organisationIDs)
		for _, organisationID := range organisationIDs {
			if usedNames[organisationID] == nil {
				usedNames[organisationID] = map[string]bool{}
			}
			usedNames[organisationID][normalized] = true
			membership := user.Memberships[organisationID]
			if _, err := tx.Exec(`INSERT INTO organisation_memberships(organisation_id,user_id,role,name_normalized,created_at) VALUES(?,?,?,?,?)`, organisationID, user.ID, membership.Role, normalized, membership.CreatedAt); err != nil {
				return err
			}
		}
	}
	orphanRows, err := tx.Query(`SELECT id,name FROM users
		WHERE NOT EXISTS(SELECT 1 FROM organisation_memberships WHERE user_id=users.id)
		ORDER BY created_at,id`)
	if err != nil {
		return err
	}
	type orphanedUser struct{ ID, Name string }
	var orphanedUsers []orphanedUser
	for orphanRows.Next() {
		var user orphanedUser
		if err := orphanRows.Scan(&user.ID, &user.Name); err != nil {
			orphanRows.Close()
			return err
		}
		orphanedUsers = append(orphanedUsers, user)
	}
	if err := orphanRows.Err(); err != nil {
		orphanRows.Close()
		return err
	}
	orphanRows.Close()
	for _, user := range orphanedUsers {
		name := CanonicalName(user.Name)
		if name == "" {
			return fmt.Errorf("user %s has an empty name", user.ID)
		}
		if _, err := tx.Exec(`UPDATE users SET name=? WHERE id=?`, name, user.ID); err != nil {
			return err
		}
	}
	_, err = tx.Exec(`DROP TABLE organisation_memberships_canonical_legacy`)
	return err
}

func CanonicalName(name string) string {
	return norm.NFC.String(strings.TrimSpace(name))
}

func NormalizeName(name string) string {
	return strings.ToLower(CanonicalName(name))
}

func NewID(prefix string) string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return prefix + "_" + hex.EncodeToString(b)
}
