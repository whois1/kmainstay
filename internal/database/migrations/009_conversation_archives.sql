CREATE TABLE conversation_archives (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    archived_at TEXT NOT NULL,
    PRIMARY KEY(user_id, conversation_id)
);
CREATE INDEX conversation_archives_conversation ON conversation_archives(conversation_id);
