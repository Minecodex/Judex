-- +goose Up
ALTER TABLE messages ADD COLUMN confirmation_title text NOT NULL DEFAULT '';
CREATE TABLE confirmation_responses (
  request_message_id uuid NOT NULL REFERENCES messages(id),
  user_id uuid NOT NULL REFERENCES users(id),
  response_message_id uuid NOT NULL UNIQUE REFERENCES messages(id),
  choice text NOT NULL CHECK (choice IN ('agree','changes','comment')),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (request_message_id,user_id)
);
-- Existing explicit replies already resolved the corresponding human branch.
INSERT INTO confirmation_responses(request_message_id,user_id,response_message_id,choice,created_at)
SELECT DISTINCT ON (q.id,r.author_user_id) q.id,r.author_user_id,r.id,'comment',r.created_at
FROM messages q JOIN message_mentions mm ON mm.message_id=q.id AND mm.kind='human' AND mm.requires_response
JOIN messages r ON r.reply_to=q.id AND r.author_user_id=mm.target_id AND r.role_agent_id IS NULL
ORDER BY q.id,r.author_user_id,r.seq;
