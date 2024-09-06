package web

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/vhrboliveira/ama-go/internal/store/pgstore"
)

const (
	MessageKindMessageCreated         = "message_created"
	MessageKindMessageReactionAdd     = "message_reaction_added"
	MessageKindMessageReactionRemoved = "message_reaction_removed"
	MessageKindMessageAnswered        = "message_answered"
	MessageKindRoomCreated            = "room_created"
)

type MessageCreated struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	Message   string `json:"message"`
}

type MessageReactionAdded struct {
	ID    string `json:"id"`
	Count int32  `json:"count"`
}

type MessageReactionRemoved struct {
	ID    string `json:"id"`
	Count int32  `json:"count"`
}

type MessageAnswered struct {
	ID string `json:"id"`
}

type Message struct {
	Kind   string `json:"kind"`
	Value  any    `json:"value"`
	RoomID string `json:"-"`
}

type RoomCreated struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	Name      string `json:"name"`
	UserID    string `json:"user_id"`
}

func (h Handlers) readRoom(w http.ResponseWriter, r *http.Request) (room pgstore.GetRoomRow, rawRoomID string, roomId uuid.UUID, ok bool) {
	rawRoomID = chi.URLParam(r, "room_id")
	roomId, err := uuid.Parse(rawRoomID)
	if err != nil {
		http.Error(w, "invalid room id", http.StatusBadRequest)
		return
	}

	room, err = h.Queries.GetRoom(r.Context(), roomId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "room not found", http.StatusBadRequest)
			return pgstore.GetRoomRow{}, "", uuid.UUID{}, false
		}

		slog.Error("error getting room", "room", roomId, "error", err)
		http.Error(w, "error getting room", http.StatusInternalServerError)
		return pgstore.GetRoomRow{}, "", uuid.UUID{}, false
	}

	return room, rawRoomID, roomId, true
}

func sendJSON(w http.ResponseWriter, rawData any) {
	data, _ := json.Marshal(rawData)

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
