CREATE TABLE message_mentions (
    message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    PRIMARY KEY(message_id, user_id)
);

CREATE INDEX message_mentions_user ON message_mentions(user_id, message_id);

CREATE TABLE message_bot_deliveries (
    message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    PRIMARY KEY(message_id, user_id)
);

CREATE INDEX message_bot_deliveries_user ON message_bot_deliveries(user_id, message_id);
