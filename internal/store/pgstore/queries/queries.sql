-- name: GetRoom :one
SELECT * FROM rooms WHERE id = $1;

-- name: GetRoomWithUser :one
SELECT
  r."id", r."name", r."description", r."created_at", r."updated_at", u."email", u."name" as "creator_name", u."photo", u."enable_picture"
FROM rooms r
LEFT JOIN users u ON r.user_id = u.id
WHERE r.id = $1;

-- name: GetRooms :many
SELECT r.*, u."name" as "creator_name" FROM rooms r
LEFT JOIN users u on r.user_id = u.id
ORDER BY r.created_at ASC;

-- name: InsertRoom :one
INSERT INTO rooms
  ("name", "user_id", "description") VALUES
  ($1, $2, $3)
RETURNING "id", "created_at";

-- name: GetMessage :one
SELECT * FROM messages WHERE id = $1;

-- name: GetRoomMessages :many
SELECT * FROM messages WHERE room_id = $1 ORDER BY created_at DESC;

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