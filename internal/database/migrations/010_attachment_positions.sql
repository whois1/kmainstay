ALTER TABLE attachments ADD COLUMN position INTEGER NOT NULL DEFAULT 0 CHECK(position BETWEEN 0 AND 9);

UPDATE attachments
SET position = (
    SELECT count(*) - 1
    FROM attachments AS ordered
    WHERE ordered.message_id = attachments.message_id
      AND (ordered.created_at < attachments.created_at OR (ordered.created_at = attachments.created_at AND ordered.id <= attachments.id))
);

CREATE UNIQUE INDEX attachments_message_position ON attachments(message_id, position);
