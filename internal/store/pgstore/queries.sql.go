// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.26.0
// source: queries.sql

package pgstore

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const answerMessage = `-- name: AnswerMessage :one
UPDATE messages
SET
  answered = true,
  answer = $1,
  updated_at = now()
WHERE
  id = $2 and answered = false
RETURNING answered
`

type AnswerMessageParams struct {
	Answer string    `db:"answer" json:"answer"`
	ID     uuid.UUID `db:"id" json:"id"`
}

func (q *Queries) AnswerMessage(ctx context.Context, arg AnswerMessageParams) (bool, error) {
	row := q.db.QueryRow(ctx, answerMessage, arg.Answer, arg.ID)
	var answered bool
	err := row.Scan(&answered)
	return answered, err
}

const createUser = `-- name: CreateUser :one
INSERT INTO users
  ("email", "name", "provider", "provider_user_id", "photo") VALUES
  ($1, $2, $3, $4, $5)
RETURNING "id", "created_at", "updated_at"
`

type CreateUserParams struct {
	Email          string `db:"email" json:"email"`
	Name           string `db:"name" json:"name"`
	Provider       string `db:"provider" json:"provider"`
	ProviderUserID string `db:"provider_user_id" json:"provider_user_id"`
	Photo          string `db:"photo" json:"photo"`
}

type CreateUserRow struct {
	ID        uuid.UUID        `db:"id" json:"id"`
	CreatedAt pgtype.Timestamp `db:"created_at" json:"created_at"`
	UpdatedAt pgtype.Timestamp `db:"updated_at" json:"updated_at"`
}

func (q *Queries) CreateUser(ctx context.Context, arg CreateUserParams) (CreateUserRow, error) {
	row := q.db.QueryRow(ctx, createUser,
		arg.Email,
		arg.Name,
		arg.Provider,
		arg.ProviderUserID,
		arg.Photo,
	)
	var i CreateUserRow
	err := row.Scan(&i.ID, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

const getMessage = `-- name: GetMessage :one
SELECT id, room_id, message, reaction_count, answered, created_at, updated_at, answer FROM messages WHERE id = $1
`

func (q *Queries) GetMessage(ctx context.Context, id uuid.UUID) (Message, error) {
	row := q.db.QueryRow(ctx, getMessage, id)
	var i Message
	err := row.Scan(
		&i.ID,
		&i.RoomID,
		&i.Message,
		&i.ReactionCount,
		&i.Answered,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.Answer,
	)
	return i, err
}

const getRoom = `-- name: GetRoom :one
SELECT id, name, created_at, updated_at, user_id, description FROM rooms WHERE id = $1
`

func (q *Queries) GetRoom(ctx context.Context, id uuid.UUID) (Room, error) {
	row := q.db.QueryRow(ctx, getRoom, id)
	var i Room
	err := row.Scan(
		&i.ID,
		&i.Name,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.UserID,
		&i.Description,
	)
	return i, err
}

const getRoomMessages = `-- name: GetRoomMessages :many
SELECT id, room_id, message, reaction_count, answered, created_at, updated_at, answer FROM messages WHERE room_id = $1 ORDER BY created_at DESC
`

func (q *Queries) GetRoomMessages(ctx context.Context, roomID uuid.UUID) ([]Message, error) {
	rows, err := q.db.Query(ctx, getRoomMessages, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Message
	for rows.Next() {
		var i Message
		if err := rows.Scan(
			&i.ID,
			&i.RoomID,
			&i.Message,
			&i.ReactionCount,
			&i.Answered,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.Answer,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getRoomWithUser = `-- name: GetRoomWithUser :one
SELECT
  r."id", r."name", r."description", r."created_at", r."updated_at", u."email", u."name" as "creator_name", u."id" as "user_id", u."photo", u."enable_picture"
FROM rooms r
LEFT JOIN users u ON r.user_id = u.id
WHERE r.id = $1
`

type GetRoomWithUserRow struct {
	ID            uuid.UUID        `db:"id" json:"id"`
	Name          string           `db:"name" json:"name"`
	Description   string           `db:"description" json:"description"`
	CreatedAt     pgtype.Timestamp `db:"created_at" json:"created_at"`
	UpdatedAt     pgtype.Timestamp `db:"updated_at" json:"updated_at"`
	Email         pgtype.Text      `db:"email" json:"email"`
	CreatorName   pgtype.Text      `db:"creator_name" json:"creator_name"`
	UserID        pgtype.UUID      `db:"user_id" json:"user_id"`
	Photo         pgtype.Text      `db:"photo" json:"photo"`
	EnablePicture pgtype.Bool      `db:"enable_picture" json:"enable_picture"`
}

func (q *Queries) GetRoomWithUser(ctx context.Context, id uuid.UUID) (GetRoomWithUserRow, error) {
	row := q.db.QueryRow(ctx, getRoomWithUser, id)
	var i GetRoomWithUserRow
	err := row.Scan(
		&i.ID,
		&i.Name,
		&i.Description,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.Email,
		&i.CreatorName,
		&i.UserID,
		&i.Photo,
		&i.EnablePicture,
	)
	return i, err
}

const getRooms = `-- name: GetRooms :many
SELECT r.id, r.name, r.created_at, r.updated_at, r.user_id, r.description, u."name" as "creator_name" FROM rooms r
LEFT JOIN users u on r.user_id = u.id
ORDER BY r.created_at ASC
`

type GetRoomsRow struct {
	ID          uuid.UUID        `db:"id" json:"id"`
	Name        string           `db:"name" json:"name"`
	CreatedAt   pgtype.Timestamp `db:"created_at" json:"created_at"`
	UpdatedAt   pgtype.Timestamp `db:"updated_at" json:"updated_at"`
	UserID      uuid.UUID        `db:"user_id" json:"user_id"`
	Description string           `db:"description" json:"description"`
	CreatorName pgtype.Text      `db:"creator_name" json:"creator_name"`
}

func (q *Queries) GetRooms(ctx context.Context) ([]GetRoomsRow, error) {
	rows, err := q.db.Query(ctx, getRooms)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetRoomsRow
	for rows.Next() {
		var i GetRoomsRow
		if err := rows.Scan(
			&i.ID,
			&i.Name,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.UserID,
			&i.Description,
			&i.CreatorName,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getUserByEmail = `-- name: GetUserByEmail :one
SELECT id, email, name, created_at, updated_at, photo, enable_picture, provider, provider_user_id, new_user FROM users WHERE email = $1 LIMIT 1
`

func (q *Queries) GetUserByEmail(ctx context.Context, email string) (User, error) {
	row := q.db.QueryRow(ctx, getUserByEmail, email)
	var i User
	err := row.Scan(
		&i.ID,
		&i.Email,
		&i.Name,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.Photo,
		&i.EnablePicture,
		&i.Provider,
		&i.ProviderUserID,
		&i.NewUser,
	)
	return i, err
}

const getUserById = `-- name: GetUserById :one
SELECT id, email, name, created_at, updated_at, photo, enable_picture, provider, provider_user_id, new_user FROM users WHERE id = $1 LIMIT 1
`

func (q *Queries) GetUserById(ctx context.Context, id uuid.UUID) (User, error) {
	row := q.db.QueryRow(ctx, getUserById, id)
	var i User
	err := row.Scan(
		&i.ID,
		&i.Email,
		&i.Name,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.Photo,
		&i.EnablePicture,
		&i.Provider,
		&i.ProviderUserID,
		&i.NewUser,
	)
	return i, err
}

const insertMessage = `-- name: InsertMessage :one
INSERT INTO messages
  ("room_id", "message") VALUES
  ($1, $2)
RETURNING "id", "created_at"
`

type InsertMessageParams struct {
	RoomID  uuid.UUID `db:"room_id" json:"room_id"`
	Message string    `db:"message" json:"message"`
}

type InsertMessageRow struct {
	ID        uuid.UUID        `db:"id" json:"id"`
	CreatedAt pgtype.Timestamp `db:"created_at" json:"created_at"`
}

func (q *Queries) InsertMessage(ctx context.Context, arg InsertMessageParams) (InsertMessageRow, error) {
	row := q.db.QueryRow(ctx, insertMessage, arg.RoomID, arg.Message)
	var i InsertMessageRow
	err := row.Scan(&i.ID, &i.CreatedAt)
	return i, err
}

const insertRoom = `-- name: InsertRoom :one
INSERT INTO rooms
  ("name", "user_id", "description") VALUES
  ($1, $2, $3)
RETURNING "id", "created_at"
`

type InsertRoomParams struct {
	Name        string    `db:"name" json:"name"`
	UserID      uuid.UUID `db:"user_id" json:"user_id"`
	Description string    `db:"description" json:"description"`
}

type InsertRoomRow struct {
	ID        uuid.UUID        `db:"id" json:"id"`
	CreatedAt pgtype.Timestamp `db:"created_at" json:"created_at"`
}

func (q *Queries) InsertRoom(ctx context.Context, arg InsertRoomParams) (InsertRoomRow, error) {
	row := q.db.QueryRow(ctx, insertRoom, arg.Name, arg.UserID, arg.Description)
	var i InsertRoomRow
	err := row.Scan(&i.ID, &i.CreatedAt)
	return i, err
}

const reactToMessage = `-- name: ReactToMessage :one
UPDATE messages
SET
  reaction_count = reaction_count + 1
WHERE
  id = $1
RETURNING reaction_count
`

func (q *Queries) ReactToMessage(ctx context.Context, id uuid.UUID) (int32, error) {
	row := q.db.QueryRow(ctx, reactToMessage, id)
	var reaction_count int32
	err := row.Scan(&reaction_count)
	return reaction_count, err
}

const removeReactionFromMessage = `-- name: RemoveReactionFromMessage :one
UPDATE messages
SET
  reaction_count = reaction_count - 1
WHERE
  id = $1 AND reaction_count > 0
RETURNING reaction_count
`

func (q *Queries) RemoveReactionFromMessage(ctx context.Context, id uuid.UUID) (int32, error) {
	row := q.db.QueryRow(ctx, removeReactionFromMessage, id)
	var reaction_count int32
	err := row.Scan(&reaction_count)
	return reaction_count, err
}

const updateUser = `-- name: UpdateUser :one
UPDATE users
SET
  name = $2,
  enable_picture = $3,
  new_user = false,
  updated_at = now()
WHERE
  id = $1
RETURNING new_user, updated_at
`

type UpdateUserParams struct {
	ID            uuid.UUID `db:"id" json:"id"`
	Name          string    `db:"name" json:"name"`
	EnablePicture bool      `db:"enable_picture" json:"enable_picture"`
}

type UpdateUserRow struct {
	NewUser   bool             `db:"new_user" json:"new_user"`
	UpdatedAt pgtype.Timestamp `db:"updated_at" json:"updated_at"`
}

func (q *Queries) UpdateUser(ctx context.Context, arg UpdateUserParams) (UpdateUserRow, error) {
	row := q.db.QueryRow(ctx, updateUser, arg.ID, arg.Name, arg.EnablePicture)
	var i UpdateUserRow
	err := row.Scan(&i.NewUser, &i.UpdatedAt)
	return i, err
}
