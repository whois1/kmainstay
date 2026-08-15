ALTER TABLE organisation_memberships RENAME TO organisation_memberships_legacy;

CREATE TABLE organisation_memberships (
    organisation_id TEXT NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK(role IN ('admin','member')),
    name_normalized TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY(organisation_id,user_id),
    UNIQUE(organisation_id,name_normalized)
);
