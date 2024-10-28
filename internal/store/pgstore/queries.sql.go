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
  id = $2 AND answered = false
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

const deleteUser = `-- name: DeleteUser :one
DELETE FROM users
WHERE id = $1 RETURNING id
`

func (q *Queries) DeleteUser(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	row := q.db.QueryRow(ctx, deleteUser, id)
	err := row.Scan(&id)
	return id, err
}

const getMessage = `-- name: GetMessage :one
SELECT id, room_id, message, answered, created_at, updated_at, answer FROM messages WHERE id = $1
`

func (q *Queries) GetMessage(ctx context.Context, id uuid.UUID) (Message, error) {
	row := q.db.QueryRow(ctx, getMessage, id)
	var i Message
	err := row.Scan(
		&i.ID,
		&i.RoomID,
		&i.Message,
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

func (q *Queries) GetRoom(ctx context.Context, id int64) (Room, error) {
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
SELECT m.id, m.room_id, m.message, m.answered, m.created_at, m.updated_at, m.answer, COUNT(mr.message_id) AS reaction_count FROM messages m
LEFT JOIN messages_reactions mr ON mr.message_id = m.id
WHERE room_id = $1 GROUP BY m.id ORDER BY created_at DESC
`

type GetRoomMessagesRow struct {
	ID            uuid.UUID        `db:"id" json:"id"`
	RoomID        int64            `db:"room_id" json:"room_id"`
	Message       string           `db:"message" json:"message"`
	Answered      bool             `db:"answered" json:"answered"`
	CreatedAt     pgtype.Timestamp `db:"created_at" json:"created_at"`
	UpdatedAt     pgtype.Timestamp `db:"updated_at" json:"updated_at"`
	Answer        string           `db:"answer" json:"answer"`
	ReactionCount int64            `db:"reaction_count" json:"reaction_count"`
}

func (q *Queries) GetRoomMessages(ctx context.Context, roomID int64) ([]GetRoomMessagesRow, error) {
	rows, err := q.db.Query(ctx, getRoomMessages, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetRoomMessagesRow
	for rows.Next() {
		var i GetRoomMessagesRow
		if err := rows.Scan(
			&i.ID,
			&i.RoomID,
			&i.Message,
			&i.Answered,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.Answer,
			&i.ReactionCount,
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

const getRoomMessagesReactions = `-- name: GetRoomMessagesReactions :many
SELECT mr.message_id FROM messages_reactions mr 
LEFT JOIN messages m ON m.id = mr.message_id 
WHERE m.room_id = $1 AND mr.user_id = $2
`

type GetRoomMessagesReactionsParams struct {
	RoomID int64     `db:"room_id" json:"room_id"`
	UserID uuid.UUID `db:"user_id" json:"user_id"`
}

func (q *Queries) GetRoomMessagesReactions(ctx context.Context, arg GetRoomMessagesReactionsParams) ([]uuid.UUID, error) {
	rows, err := q.db.Query(ctx, getRoomMessagesReactions, arg.RoomID, arg.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []uuid.UUID
	for rows.Next() {
		var message_id uuid.UUID
		if err := rows.Scan(&message_id); err != nil {
			return nil, err
		}
		items = append(items, message_id)
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
	ID            int64            `db:"id" json:"id"`
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

func (q *Queries) GetRoomWithUser(ctx context.Context, id int64) (GetRoomWithUserRow, error) {
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
SELECT r.id, r.name, r.created_at, r.updated_at, r.user_id, r.description, u."name" AS "creator_name" FROM rooms r
LEFT JOIN users u ON r.user_id = u.id
ORDER BY r.created_at ASC
`

type GetRoomsRow struct {
	ID          int64            `db:"id" json:"id"`
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
	RoomID  int64  `db:"room_id" json:"room_id"`
	Message string `db:"message" json:"message"`
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

const insertMessageReaction = `-- name: InsertMessageReaction :one
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
FROM inserted
`

type InsertMessageReactionParams struct {
	MessageID uuid.UUID `db:"message_id" json:"message_id"`
	UserID    uuid.UUID `db:"user_id" json:"user_id"`
}

func (q *Queries) InsertMessageReaction(ctx context.Context, arg InsertMessageReactionParams) (int32, error) {
	row := q.db.QueryRow(ctx, insertMessageReaction, arg.MessageID, arg.UserID)
	var total_reactions int32
	err := row.Scan(&total_reactions)
	return total_reactions, err
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
	ID        int64            `db:"id" json:"id"`
	CreatedAt pgtype.Timestamp `db:"created_at" json:"created_at"`
}

func (q *Queries) InsertRoom(ctx context.Context, arg InsertRoomParams) (InsertRoomRow, error) {
	row := q.db.QueryRow(ctx, insertRoom, arg.Name, arg.UserID, arg.Description)
	var i InsertRoomRow
	err := row.Scan(&i.ID, &i.CreatedAt)
	return i, err
}

const removeMessageReaction = `-- name: RemoveMessageReaction :one
WITH mr_t AS (
  SELECT COUNT(*) AS total_count
  FROM messages_reactions mr
  WHERE mr."message_id" = $1
), deleted AS (
  DELETE FROM messages_reactions mr2
  WHERE mr2.message_id = $1 AND mr2.user_id = $2
  RETURNING message_id, user_id
)
SELECT (SELECT total_count FROM mr_t) - 1 AS total_reactions
FROM deleted
`

type RemoveMessageReactionParams struct {
	MessageID uuid.UUID `db:"message_id" json:"message_id"`
	UserID    uuid.UUID `db:"user_id" json:"user_id"`
}

func (q *Queries) RemoveMessageReaction(ctx context.Context, arg RemoveMessageReactionParams) (int32, error) {
	row := q.db.QueryRow(ctx, removeMessageReaction, arg.MessageID, arg.UserID)
	var total_reactions int32
	err := row.Scan(&total_reactions)
	return total_reactions, err
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

const userHasReacted = `-- name: UserHasReacted :one
SELECT message_id, user_id FROM messages_reactions
WHERE message_id = $1 AND user_id = $2
`

type UserHasReactedParams struct {
	MessageID uuid.UUID `db:"message_id" json:"message_id"`
	UserID    uuid.UUID `db:"user_id" json:"user_id"`
}

func (q *Queries) UserHasReacted(ctx context.Context, arg UserHasReactedParams) (MessagesReaction, error) {
	row := q.db.QueryRow(ctx, userHasReacted, arg.MessageID, arg.UserID)
	var i MessagesReaction
	err := row.Scan(&i.MessageID, &i.UserID)
	return i, err
}
