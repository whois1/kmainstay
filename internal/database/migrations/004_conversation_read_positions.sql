CREATE TABLE conversation_read_positions (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sequence INTEGER NOT NULL CHECK(sequence >= 0),
    updated_at TEXT NOT NULL,
    PRIMARY KEY(user_id, conversation_id)
);

INSERT INTO conversation_read_positions(user_id, conversation_id, sequence, updated_at)
SELECT membership.user_id,
       conversation.id,
       COALESCE(MAX(event.sequence), 0),
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
FROM conversations conversation
JOIN organisation_memberships membership
  ON membership.organisation_id = conversation.organisation_id
LEFT JOIN realtime_events event
  ON event.conversation_id = conversation.id
WHERE conversation.visibility = 'organisation'
   OR EXISTS (
       SELECT 1
       FROM conversation_members conversation_member
       WHERE conversation_member.conversation_id = conversation.id
         AND conversation_member.user_id = membership.user_id
   )
GROUP BY membership.user_id, conversation.id;
