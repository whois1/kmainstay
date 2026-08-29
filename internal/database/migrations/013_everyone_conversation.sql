ALTER TABLE conversations ADD COLUMN is_everyone INTEGER NOT NULL DEFAULT 0 CHECK(is_everyone IN (0,1));
CREATE UNIQUE INDEX conversations_one_everyone_per_organisation ON conversations(organisation_id) WHERE is_everyone=1;
