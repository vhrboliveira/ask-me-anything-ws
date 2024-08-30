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

const getMessage = `-- name: GetMessage :one
SELECT
  "id", "room_id", "message", "reaction_count", "answered", "created_at"
FROM messages
WHERE id = $1
`

type GetMessageRow struct {
	ID            uuid.UUID   `db:"id" json:"id"`
	RoomID        uuid.UUID   `db:"room_id" json:"room_id"`
	Message       string      `db:"message" json:"message"`
	ReactionCount int32       `db:"reaction_count" json:"reaction_count"`
	Answered      bool        `db:"answered" json:"answered"`
	CreatedAt     pgtype.Date `db:"created_at" json:"created_at"`
}

func (q *Queries) GetMessage(ctx context.Context, id uuid.UUID) (GetMessageRow, error) {
	row := q.db.QueryRow(ctx, getMessage, id)
	var i GetMessageRow
	err := row.Scan(
		&i.ID,
		&i.RoomID,
		&i.Message,
		&i.ReactionCount,
		&i.Answered,
		&i.CreatedAt,
	)
	return i, err
}

const getRoom = `-- name: GetRoom :one
SELECT 
  "id", "name"
FROM rooms
WHERE id = $1
`

type GetRoomRow struct {
	ID   uuid.UUID `db:"id" json:"id"`
	Name string    `db:"name" json:"name"`
}

func (q *Queries) GetRoom(ctx context.Context, id uuid.UUID) (GetRoomRow, error) {
	row := q.db.QueryRow(ctx, getRoom, id)
	var i GetRoomRow
	err := row.Scan(&i.ID, &i.Name)
	return i, err
}

const getRoomMessages = `-- name: GetRoomMessages :many
SELECT
  "id", "room_id", "message", "reaction_count", "answered", "created_at"
FROM messages
WHERE room_id = $1
`

type GetRoomMessagesRow struct {
	ID            uuid.UUID   `db:"id" json:"id"`
	RoomID        uuid.UUID   `db:"room_id" json:"room_id"`
	Message       string      `db:"message" json:"message"`
	ReactionCount int32       `db:"reaction_count" json:"reaction_count"`
	Answered      bool        `db:"answered" json:"answered"`
	CreatedAt     pgtype.Date `db:"created_at" json:"created_at"`
}

func (q *Queries) GetRoomMessages(ctx context.Context, roomID uuid.UUID) ([]GetRoomMessagesRow, error) {
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
			&i.ReactionCount,
			&i.Answered,
			&i.CreatedAt,
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

const getRooms = `-- name: GetRooms :many
SELECT
  "id", "name"
FROM rooms
`

type GetRoomsRow struct {
	ID   uuid.UUID `db:"id" json:"id"`
	Name string    `db:"name" json:"name"`
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
		if err := rows.Scan(&i.ID, &i.Name); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const insertMessage = `-- name: InsertMessage :one
INSERT INTO messages
  ("room_id", "message") VALUES
  ($1, $2)
RETURNING "id"
`

type InsertMessageParams struct {
	RoomID  uuid.UUID `db:"room_id" json:"room_id"`
	Message string    `db:"message" json:"message"`
}

func (q *Queries) InsertMessage(ctx context.Context, arg InsertMessageParams) (uuid.UUID, error) {
	row := q.db.QueryRow(ctx, insertMessage, arg.RoomID, arg.Message)
	var id uuid.UUID
	err := row.Scan(&id)
	return id, err
}

const insertRoom = `-- name: InsertRoom :one
INSERT INTO rooms
  ("name") VALUES
  ($1)
RETURNING "id"
`

func (q *Queries) InsertRoom(ctx context.Context, name string) (uuid.UUID, error) {
	row := q.db.QueryRow(ctx, insertRoom, name)
	var id uuid.UUID
	err := row.Scan(&id)
	return id, err
}

const markMessageAsAnswered = `-- name: MarkMessageAsAnswered :exec
UPDATE messages
SET
  answered = true,
  updated_at = now()
WHERE
  id = $1
`

func (q *Queries) MarkMessageAsAnswered(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.Exec(ctx, markMessageAsAnswered, id)
	return err
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
