package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/vhrboliveira/ama-go-react/internal/store/pgstore"
)

func (h apiHandler) createRoom(w http.ResponseWriter, r *http.Request) {
	type _body struct {
		Name string `json:"name"`
	}

	var body _body
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	roomID, err := h.q.InsertRoom(r.Context(), body.Name)
	if err != nil {
		slog.Error("error creating room", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	type response struct {
		ID string `json:"id"`
	}

	sendJSON(w, response{ID: roomID.String()})
}

func (h apiHandler) getRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.q.GetRooms(r.Context())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "no rooms found", http.StatusBadRequest)
			return
		}

		slog.Error("error getting rooms list", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if rooms == nil {
		rooms = []pgstore.GetRoomsRow{}
	}

	sendJSON(w, rooms)
}

func (h apiHandler) createRoomMessage(w http.ResponseWriter, r *http.Request) {
	_, rawRoomID, roomID, ok := h.readRoom(w, r)
	if !ok {
		return
	}

	type _body struct {
		Message string `json:"message"`
	}

	var body _body
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	messageID, err := h.q.InsertMessage(r.Context(), pgstore.InsertMessageParams{
		RoomID:  roomID,
		Message: body.Message,
	})

	if err != nil {
		slog.Error("error inserting message", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	type response struct {
		ID string `json:"id"`
	}

	sendJSON(w, response{ID: messageID.String()})

	go h.notifyClient(Message{
		Kind:   MessageKindMessageCreated,
		Value:  MessageCreated{ID: messageID.String(), Message: body.Message},
		RoomID: rawRoomID,
	})
}

func (h apiHandler) getRoomMessages(w http.ResponseWriter, r *http.Request) {
	_, _, roomID, ok := h.readRoom(w, r)
	if !ok {
		return
	}

	roomMessages, err := h.q.GetRoomMessages(r.Context(), roomID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "room messages not found", http.StatusBadRequest)
			return
		}

		slog.Error("error getting room messages", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if roomMessages == nil {
		roomMessages = []pgstore.GetRoomMessagesRow{}
	}

	sendJSON(w, roomMessages)
}

func (h apiHandler) getRoomMessage(w http.ResponseWriter, r *http.Request) {
	_, _, _, ok := h.readRoom(w, r)
	if !ok {
		return
	}

	rawMessageID := chi.URLParam(r, "message_id")
	messageID, err := uuid.Parse(rawMessageID)
	if err != nil {
		slog.Error("unable to parse message id", "error", err)
		http.Error(w, "invalid message id", http.StatusBadRequest)
		return
	}

	message, err := h.q.GetMessage(r.Context(), messageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Error("message not found", "error", err)
			http.Error(w, "message not found", http.StatusBadRequest)
			return
		}

		slog.Error("unable to get message", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	sendJSON(w, message)
}

func (h apiHandler) reactionToMessage(w http.ResponseWriter, r *http.Request) {
	_, rawRoomID, _, ok := h.readRoom(w, r)
	if !ok {
		return
	}

	rawMessageID := chi.URLParam(r, "message_id")
	messageID, err := uuid.Parse(rawMessageID)
	if err != nil {
		slog.Error("unable to parse message id", "error", err)
		http.Error(w, "invalid message id", http.StatusBadRequest)
		return
	}

	_, err = h.q.GetMessage(r.Context(), messageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Error("message not found", "error", err)
			http.Error(w, "message not found", http.StatusBadRequest)
			return
		}

		slog.Error("unable to get message", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	count, err := h.q.ReactToMessage(r.Context(), messageID)
	if err != nil {
		slog.Error("unable to react to message", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	type response struct {
		Count int32 `json:"count"`
	}

	sendJSON(w, response{Count: count})

	go h.notifyClient(Message{
		Kind:   MessageKindMessageReactionAdd,
		RoomID: rawRoomID,
		Value: MessageReactionAdded{
			ID:    rawMessageID,
			Count: count,
		},
	})
}

func (h apiHandler) removeReactionFromMessage(w http.ResponseWriter, r *http.Request) {
	_, rawRoomID, _, ok := h.readRoom(w, r)
	if !ok {
		return
	}

	rawMessageID := chi.URLParam(r, "message_id")
	messageID, err := uuid.Parse(rawMessageID)
	if err != nil {
		slog.Error("unable to parse message id", "error", err)
		http.Error(w, "invalid message id", http.StatusBadRequest)
		return
	}

	_, err = h.q.GetMessage(r.Context(), messageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Error("message not found", "error", err)
			http.Error(w, "message not found", http.StatusBadRequest)
			return
		}

		slog.Error("unable to get message", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	count, err := h.q.RemoveReactionFromMessage(r.Context(), messageID)
	if err != nil {
		slog.Error("unable to react to message", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	type response struct {
		Count int32 `json:"count"`
	}

	sendJSON(w, response{Count: count})

	go h.notifyClient(Message{
		Kind:   MessageKindMessageReactionRemoved,
		RoomID: rawRoomID,
		Value: MessageReactionRemoved{
			ID:    rawMessageID,
			Count: count,
		},
	})
}

func (h apiHandler) setMessageToAnswered(w http.ResponseWriter, r *http.Request) {
	_, rawRoomID, _, ok := h.readRoom(w, r)
	if !ok {
		return
	}

	rawMessageID := chi.URLParam(r, "message_id")
	messageID, err := uuid.Parse(rawMessageID)
	if err != nil {
		slog.Error("unable to parse message id", "error", err)
		http.Error(w, "invalid message id", http.StatusBadRequest)
		return
	}

	_, err = h.q.GetMessage(r.Context(), messageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Error("message not found", "error", err)
			http.Error(w, "message not found", http.StatusBadRequest)
			return
		}

		slog.Error("unable to get message", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	err = h.q.MarkMessageAsAnswered(r.Context(), messageID)
	if err != nil {
		slog.Error("unable to react to message", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)

	go h.notifyClient(Message{
		Kind:   MessageKindMessageAnswered,
		RoomID: rawRoomID,
		Value: MessageAnswered{
			ID: rawMessageID,
		},
	})
}
