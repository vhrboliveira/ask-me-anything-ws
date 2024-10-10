CREATE TABLE IF NOT EXISTS messages_reactions (
  "message_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,

  FOREIGN KEY (message_id) REFERENCES messages(id),
  FOREIGN KEY (user_id) REFERENCES users(id),

  PRIMARY KEY (message_id, user_id)
);

ALTER TABLE messages
  DROP COLUMN "reaction_count";

---- create above / drop below ----

DROP TABLE IF EXISTS messages_reactions;

ALTER TABLE messages
  ADD COLUMN "reaction_count" INT NOT NULL DEFAULT 0;