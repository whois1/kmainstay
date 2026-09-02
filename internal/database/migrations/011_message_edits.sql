ALTER TABLE messages ADD COLUMN edited_at TEXT;
ALTER TABLE realtime_events ADD COLUMN creation_payload TEXT;

CREATE TABLE realtime_event_sequences (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT
);
INSERT INTO realtime_event_sequences(sequence)
SELECT sequence FROM realtime_events
UNION
SELECT seq FROM sqlite_sequence WHERE name = 'realtime_events' AND seq > 0
ORDER BY sequence;

CREATE TRIGGER realtime_events_require_sequence
BEFORE INSERT ON realtime_events
WHEN NEW.sequence < 1
BEGIN
    SELECT RAISE(ABORT, 'realtime_events.sequence is required by the current schema; use the current binary');
END;

CREATE TABLE message_update_events (
    sequence INTEGER PRIMARY KEY REFERENCES realtime_event_sequences(sequence) ON DELETE CASCADE,
    id TEXT NOT NULL UNIQUE,
    organisation_id TEXT NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    occurred_at TEXT NOT NULL
);
CREATE INDEX message_update_events_message ON message_update_events(message_id,sequence);
