ALTER TABLE conversations ADD COLUMN title TEXT;
ALTER TABLE conversations ADD COLUMN title_automatic INTEGER NOT NULL DEFAULT 0 CHECK(title_automatic IN (0,1));
