CREATE TABLE attachments (
    id TEXT PRIMARY KEY,
    message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    storage_key TEXT NOT NULL UNIQUE,
    media_type TEXT NOT NULL CHECK(media_type IN ('image/jpeg', 'image/png')),
    byte_size INTEGER NOT NULL CHECK(byte_size BETWEEN 1 AND 10485760),
    width INTEGER NOT NULL CHECK(width > 0),
    height INTEGER NOT NULL CHECK(height > 0),
    original_filename TEXT NOT NULL,
    sha256 TEXT NOT NULL CHECK(length(sha256) = 64),
    created_at TEXT NOT NULL
);

CREATE INDEX attachments_message ON attachments(message_id, created_at, id);
