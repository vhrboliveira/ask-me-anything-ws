-- name: GetRoom :one
SELECT 
  "id", "name"
FROM rooms
WHERE id = $1;

-- name: GetRooms :many
SELECT
  "id", "name", "created_at"
FROM rooms ORDER BY created_at ASC;

-- name: InsertRoom :one
INSERT INTO rooms
  ("name") VALUES
  ($1)
RETURNING "id", "created_at";

-- name: GetMessage :one
SELECT
  "id", "room_id", "message", "reaction_count", "answered", "created_at"
FROM messages
WHERE id = $1;

-- name: GetRoomMessages :many
SELECT
  "id", "room_id", "message", "reaction_count", "answered", "created_at"
FROM messages
WHERE room_id = $1 ORDER BY created_at DESC;

-- name: InsertMessage :one
INSERT INTO messages
  ("room_id", "message") VALUES
  ($1, $2)
RETURNING "id", "created_at";

-- name: ReactToMessage :one
UPDATE messages
SET
  reaction_count = reaction_count + 1
WHERE
  id = $1
RETURNING reaction_count;

-- name: RemoveReactionFromMessage :one
UPDATE messages
SET
  reaction_count = reaction_count - 1
WHERE
  id = $1 AND reaction_count > 0
RETURNING reaction_count;

-- name: MarkMessageAsAnswered :exec
UPDATE messages
SET
  answered = true,
  updated_at = now()
WHERE
  id = $1;

-- name: CreateUser :one
INSERT INTO users (email, password_hash, name, bio)
VALUES ($1, $2, $3, $4)
RETURNING id, email, name, bio, created_at;

-- name: GetUserByEmail :one
SELECT id, email, name, bio, password_hash, created_at
FROM users
WHERE email = $1 LIMIT 1;