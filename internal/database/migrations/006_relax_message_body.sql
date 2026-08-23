ALTER TABLE realtime_events RENAME TO realtime_events_legacy;
ALTER TABLE message_mentions RENAME TO message_mentions_legacy;
ALTER TABLE message_bot_deliveries RENAME TO message_bot_deliveries_legacy;
ALTER TABLE messages RENAME TO messages_legacy;

CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    author_id TEXT NOT NULL REFERENCES users(id),
    body TEXT NOT NULL CHECK(length(body) BETWEEN 0 AND 20000),
    client_id TEXT,
    created_at TEXT NOT NULL,
    UNIQUE(conversation_id,author_id,client_id)
);

INSERT INTO messages(id,conversation_id,author_id,body,client_id,created_at)
SELECT id,conversation_id,author_id,body,client_id,created_at FROM messages_legacy;

CREATE TABLE realtime_events (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    id TEXT NOT NULL UNIQUE,
    organisation_id TEXT NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    occurred_at TEXT NOT NULL
);

INSERT INTO realtime_events(sequence,id,organisation_id,conversation_id,message_id,occurred_at)
SELECT sequence,id,organisation_id,conversation_id,message_id,occurred_at FROM realtime_events_legacy;

CREATE TABLE message_mentions (
    message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    PRIMARY KEY(message_id, user_id)
);

INSERT INTO message_mentions(message_id,user_id,name)
SELECT message_id,user_id,name FROM message_mentions_legacy;

CREATE TABLE message_bot_deliveries (
    message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    PRIMARY KEY(message_id, user_id)
);

INSERT INTO message_bot_deliveries(message_id,user_id)
SELECT message_id,user_id FROM message_bot_deliveries_legacy;

DROP TABLE realtime_events_legacy;
DROP TABLE message_mentions_legacy;
DROP TABLE message_bot_deliveries_legacy;
DROP TABLE messages_legacy;

CREATE INDEX messages_order ON messages(conversation_id,created_at,id);
CREATE INDEX message_mentions_user ON message_mentions(user_id, message_id);
CREATE INDEX message_bot_deliveries_user ON message_bot_deliveries(user_id, message_id);
