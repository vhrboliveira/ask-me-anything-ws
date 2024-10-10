-- name: GetRoom :one
SELECT * FROM rooms WHERE id = $1;

-- name: GetRoomWithUser :one
SELECT
  r."id", r."name", r."description", r."created_at", r."updated_at", u."email", u."name" as "creator_name", u."id" as "user_id", u."photo", u."enable_picture"
FROM rooms r
LEFT JOIN users u ON r.user_id = u.id
WHERE r.id = $1;

-- name: GetRooms :many
SELECT r.*, u."name" AS "creator_name" FROM rooms r
LEFT JOIN users u ON r.user_id = u.id
ORDER BY r.created_at ASC;

-- name: InsertRoom :one
INSERT INTO rooms
  ("name", "user_id", "description") VALUES
  ($1, $2, $3)
RETURNING "id", "created_at";

-- name: GetMessage :one
SELECT * FROM messages WHERE id = $1;

-- name: GetRoomMessages :many
SELECT m.*, COUNT(mr.message_id) AS reaction_count FROM messages m
LEFT JOIN messages_reactions mr ON mr.message_id = m.id
WHERE room_id = $1 GROUP BY m.id ORDER BY created_at DESC;

-- name: InsertMessage :one
INSERT INTO messages
  ("room_id", "message") VALUES
  ($1, $2)
RETURNING "id", "created_at";

-- name: InsertMessageReaction :one
WITH mr_t AS (
  SELECT COUNT(*) AS total_count
  FROM messages_reactions mr
  WHERE mr."message_id" = $1
), inserted AS (
  INSERT INTO messages_reactions ("message_id", "user_id")
  VALUES ($1, $2)
  RETURNING "message_id"  
)
SELECT (SELECT total_count FROM mr_t) + COUNT(inserted."message_id") AS total_reactions
FROM inserted;

-- name: RemoveMessageReaction :one
WITH mr_t AS (
  SELECT COUNT(*) AS total_count
  FROM messages_reactions mr
  WHERE mr."message_id" = $1
), deleted AS (
  DELETE FROM messages_reactions mr2
  WHERE mr2.message_id = $1 AND mr2.user_id = $2
  RETURNING *
)
SELECT (SELECT total_count FROM mr_t) - 1 AS total_reactions
FROM deleted;

-- name: UserHasReacted :one
SELECT * FROM messages_reactions
WHERE message_id = $1 AND user_id = $2;

-- name: AnswerMessage :one
UPDATE messages
SET
  answered = true,
  answer = $1,
  updated_at = now()
WHERE
  id = $2 AND answered = false
RETURNING answered;

-- name: CreateUser :one
INSERT INTO users
  ("email", "name", "provider", "provider_user_id", "photo") VALUES
  ($1, $2, $3, $4, $5)
RETURNING "id", "created_at", "updated_at";

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1 LIMIT 1;

-- name: GetUserById :one
SELECT * FROM users WHERE id = $1 LIMIT 1;

-- name: UpdateUser :one
UPDATE users
SET
  name = $2,
  enable_picture = $3,
  new_user = false,
  updated_at = now()
WHERE
  id = $1
RETURNING new_user, updated_at;