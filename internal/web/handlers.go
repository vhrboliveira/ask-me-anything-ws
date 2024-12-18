package web

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/vhrboliveira/ama-go/internal/auth"
	"github.com/vhrboliveira/ama-go/internal/service"
	"github.com/vhrboliveira/ama-go/internal/store/pgstore"
	"github.com/vhrboliveira/ama-go/internal/types"
)

type Handlers struct {
	Router           *chi.Mux
	RoomService      *service.RoomService
	MessageService   *service.MessageService
	UserService      *service.UserService
	WebsocketService *service.WebSocketService
}

func sendJSON(w http.ResponseWriter, rawData any) {
	data, _ := json.Marshal(rawData)

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func NewHandler(
	roomService *service.RoomService,
	messageService *service.MessageService,
	userService *service.UserService,
	websocketService *service.WebSocketService,
) *Handlers {
	return &Handlers{
		Router:           chi.NewRouter(),
		RoomService:      roomService,
		MessageService:   messageService,
		UserService:      userService,
		WebsocketService: websocketService,
	}
}

func (h *Handlers) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.Router.ServeHTTP(w, r)
}

func (h *Handlers) CreateRoom(w http.ResponseWriter, r *http.Request) {
	type requestBody struct {
		Name        string `json:"name" validate:"required"`
		UserID      string `json:"user_id" validate:"required,uuid"`
		Description string `json:"description" validate:"max=255"`
	}

	var body requestBody
	validate := validator.New()

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		slog.Error("failed to decode body", "error", err)
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if err := validate.Struct(&body); err != nil {
		slog.Error("validation failed", "error", err)

		missingFields := []string{}
		for _, err := range err.(validator.ValidationErrors) {
			if err.Tag() == "required" {
				missingFields = append(missingFields, err.Field())
			}

			if err.Tag() == "uuid" && err.Field() == "UserID" {
				http.Error(w, "validation failed: UserID must be a valid UUID", http.StatusBadRequest)
				return
			}
		}

		http.Error(w, "validation failed, missing required field(s): "+strings.Join(missingFields, ", "), http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(body.UserID)
	if err != nil {
		slog.Error("invalid user ID", "error", err)
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	user, ok := r.Context().Value(auth.UserKey).(pgstore.User)
	if !ok {
		slog.Error("user not found on the session cookie")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if user.ID.String() != body.UserID {
		slog.Error("the provided user ID is different from the session")
		http.Error(w, "invalid user ID", http.StatusForbidden)
		return
	}

	room, err := h.RoomService.CreateRoom(r.Context(), body.Name, userID, body.Description)
	if err != nil {
		slog.Error("error creating room", "error", err)
		http.Error(w, "error creating room", http.StatusInternalServerError)
		return
	}

	type response struct {
		ID          int64  `json:"id"`
		UserID      string `json:"user_id"`
		CreatedAt   string `json:"created_at"`
		Description string `json:"description"`
	}

	createdAt := room.CreatedAt.Time.Format(time.RFC3339)

	w.WriteHeader(http.StatusCreated)
	sendJSON(w, response{ID: room.ID, UserID: userID.String(), CreatedAt: createdAt, Description: body.Description})

	go h.WebsocketService.NotifyRoomsListClients(types.Message{
		Kind:   types.MessageKindRoomCreated,
		RoomID: room.ID,
		Value: types.RoomCreated{
			ID:          room.ID,
			CreatedAt:   createdAt,
			Name:        body.Name,
			UserID:      userID.String(),
			CreatorName: user.Name,
			Description: body.Description,
		},
	})
}

func (h *Handlers) GetRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.RoomService.GetRooms(r.Context())
	if err != nil {
		slog.Error("error getting rooms list", "error", err)
		http.Error(w, "error getting rooms list", http.StatusInternalServerError)
		return
	}

	sendJSON(w, rooms)
}

func (h *Handlers) GetRoom(w http.ResponseWriter, r *http.Request) {
	rawRoomID := chi.URLParam(r, "room_id")
	roomID, err := strconv.ParseInt(rawRoomID, 10, 64)
	if err != nil {
		slog.Error("invalid room id", "error", err)
		http.Error(w, "invalid room id", http.StatusBadRequest)
		return
	}

	room, err := h.RoomService.GetRoom(r.Context(), roomID)
	if err != nil {
		slog.Error("error getting room", "error", err)
		http.Error(w, "error getting room", http.StatusInternalServerError)
		return
	}

	if room == (pgstore.GetRoomWithUserRow{}) {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}

	sendJSON(w, room)
}

func (h *Handlers) CreateRoomMessage(w http.ResponseWriter, r *http.Request) {
	type roomMessageRequestBody struct {
		Message string `json:"message" validate:"required"`
	}

	type response struct {
		ID        string `json:"id"`
		CreatedAt string `json:"created_at"`
	}

	rawRoomID := chi.URLParam(r, "room_id")
	roomID, err := strconv.ParseInt(rawRoomID, 10, 64)
	if err != nil {
		http.Error(w, "invalid room id", http.StatusBadRequest)
		return
	}

	var body roomMessageRequestBody
	validate := validator.New()
	err = json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if err := validate.Struct(&body); err != nil {
		msg := "validation failed: missing required field(s): message"
		slog.Error(msg, "error", err)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	status, err := h.RoomService.CheckRoomExists(ctx, roomID)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}

	message, err := h.MessageService.CreateMessage(ctx, roomID, body.Message)
	if err != nil {
		slog.Error("error inserting message", "error", err)
		http.Error(w, "error inserting message", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	sendJSON(w, response{ID: message.ID.String(), CreatedAt: message.CreatedAt.Time.Format(time.RFC3339)})

	go h.WebsocketService.NotifyRoomClient(types.Message{
		Kind:   types.MessageKindMessageCreated,
		Value:  types.MessageCreated{ID: message.ID.String(), CreatedAt: message.CreatedAt.Time.Format(time.RFC3339), Message: body.Message},
		RoomID: roomID,
	})
}

func (h *Handlers) GetRoomMessages(w http.ResponseWriter, r *http.Request) {
	rawRoomID := chi.URLParam(r, "room_id")
	roomID, err := strconv.ParseInt(rawRoomID, 10, 64)
	if err != nil {
		http.Error(w, "invalid room id", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	status, err := h.RoomService.CheckRoomExists(ctx, roomID)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}

	roomMessages, err := h.MessageService.GetMessages(ctx, roomID)
	if err != nil {
		slog.Error("error getting room messages", "error", err)
		http.Error(w, "error getting room messages", http.StatusInternalServerError)
		return
	}

	sendJSON(w, roomMessages)
}

func (h *Handlers) GetRoomMessage(w http.ResponseWriter, r *http.Request) {
	rawRoomID := chi.URLParam(r, "room_id")
	roomID, err := strconv.ParseInt(rawRoomID, 10, 64)
	if err != nil {
		http.Error(w, "invalid room id", http.StatusBadRequest)
		return
	}

	rawMessageID := chi.URLParam(r, "message_id")
	messageID, err := uuid.Parse(rawMessageID)
	if err != nil {
		slog.Error("error parsing message id", "error", err)
		http.Error(w, "invalid message id", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	status, err := h.RoomService.CheckRoomExists(ctx, roomID)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}

	message, err := h.MessageService.GetMessage(ctx, messageID)
	if err != nil {
		slog.Error("error getting message", "error", err)
		http.Error(w, "error getting message", http.StatusInternalServerError)
		return
	}

	if message == (pgstore.Message{}) {
		http.Error(w, "message not found", http.StatusNotFound)
		return
	}

	sendJSON(w, message)
}

func (h *Handlers) ReactionToMessage(w http.ResponseWriter, r *http.Request) {
	type requestBody struct {
		UserID string `json:"user_id" validate:"required,uuid"`
	}

	type response struct {
		Count int32 `json:"count"`
	}

	rawRoomID := chi.URLParam(r, "room_id")
	roomID, err := strconv.ParseInt(rawRoomID, 10, 64)
	if err != nil {
		http.Error(w, "invalid room id", http.StatusBadRequest)
		return
	}

	rawMessageID := chi.URLParam(r, "message_id")
	messageID, err := uuid.Parse(rawMessageID)
	if err != nil {
		slog.Error("unable to parse message id", "error", err)
		http.Error(w, "invalid message id", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	status, err := h.RoomService.CheckRoomExists(ctx, roomID)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}

	status, err = h.MessageService.CheckMessageExists(ctx, messageID)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}

	var body requestBody
	validate := validator.New()
	err = json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		slog.Error("failed to decode body", "error", err)
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if err := validate.Struct(&body); err != nil {
		slog.Error("validation failed", "error", err)

		missingFields := []string{}
		uuidFields := map[string]string{
			"UserID": "UserID must be a valid UUID",
		}

		for _, err := range err.(validator.ValidationErrors) {
			switch err.Tag() {
			case "required":
				missingFields = append(missingFields, err.Field())
			case "uuid":
				if errMsg, ok := uuidFields[err.Field()]; ok {
					http.Error(w, "validation failed: "+errMsg, http.StatusBadRequest)
					return
				}
			}
		}

		http.Error(w, "validation failed, missing required field(s): "+strings.Join(missingFields, ", "), http.StatusBadRequest)
		return
	}

	user, ok := r.Context().Value(auth.UserKey).(pgstore.User)
	if !ok {
		slog.Error("user not found on the session cookie")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if user.ID.String() != body.UserID {
		slog.Error("the provided user ID is different from the session")
		http.Error(w, "invalid user ID", http.StatusForbidden)
		return
	}

	count, err := h.MessageService.ReactToMessage(ctx, messageID, user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSON(w, response{Count: count})

	go h.WebsocketService.NotifyRoomClient(types.Message{
		Kind:   types.MessageKindMessageReactionAdd,
		RoomID: roomID,
		Value: types.MessageReactionAdded{
			ID:    rawMessageID,
			Count: count,
		},
	})
}

func (h *Handlers) RemoveReactionFromMessage(w http.ResponseWriter, r *http.Request) {
	type requestBody struct {
		UserID string `json:"user_id" validate:"required,uuid"`
	}

	type response struct {
		Count int32 `json:"count"`
	}

	rawRoomID := chi.URLParam(r, "room_id")
	roomID, err := strconv.ParseInt(rawRoomID, 10, 64)
	if err != nil {
		http.Error(w, "invalid room id", http.StatusBadRequest)
		return
	}

	rawMessageID := chi.URLParam(r, "message_id")
	messageID, err := uuid.Parse(rawMessageID)
	if err != nil {
		slog.Error("unable to parse message id", "error", err)
		http.Error(w, "invalid message id", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	status, err := h.RoomService.CheckRoomExists(ctx, roomID)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}

	status, err = h.MessageService.CheckMessageExists(ctx, messageID)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}

	var body requestBody
	validate := validator.New()
	err = json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		slog.Error("failed to decode body", "error", err)
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if err := validate.Struct(&body); err != nil {
		slog.Error("validation failed", "error", err)

		missingFields := []string{}
		uuidFields := map[string]string{
			"UserID": "UserID must be a valid UUID",
		}

		for _, err := range err.(validator.ValidationErrors) {
			switch err.Tag() {
			case "required":
				missingFields = append(missingFields, err.Field())
			case "uuid":
				if errMsg, ok := uuidFields[err.Field()]; ok {
					http.Error(w, "validation failed: "+errMsg, http.StatusBadRequest)
					return
				}
			}
		}

		http.Error(w, "validation failed, missing required field(s): "+strings.Join(missingFields, ", "), http.StatusBadRequest)
		return
	}

	user, ok := r.Context().Value(auth.UserKey).(pgstore.User)
	if !ok {
		slog.Error("user not found on the session cookie")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if user.ID.String() != body.UserID {
		slog.Error("the provided user ID is different from the session")
		http.Error(w, "invalid user ID", http.StatusForbidden)
		return
	}

	count, err := h.MessageService.RemoveReactionFromMessage(ctx, messageID, user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSON(w, response{Count: count})

	go h.WebsocketService.NotifyRoomClient(types.Message{
		Kind:   types.MessageKindMessageReactionRemoved,
		RoomID: roomID,
		Value: types.MessageReactionRemoved{
			ID:    rawMessageID,
			Count: count,
		},
	})
}

func (h *Handlers) SetMessageToAnswered(w http.ResponseWriter, r *http.Request) {
	type requestBody struct {
		UserID string `json:"user_id" validate:"required,uuid"`
		Answer string `json:"answer"  validate:"required"`
	}

	rawRoomID := chi.URLParam(r, "room_id")
	roomID, err := strconv.ParseInt(rawRoomID, 10, 64)
	if err != nil {
		http.Error(w, "invalid room id", http.StatusBadRequest)
		return
	}

	rawMessageID := chi.URLParam(r, "message_id")
	messageID, err := uuid.Parse(rawMessageID)
	if err != nil {
		slog.Error("unable to parse message id", "error", err)
		http.Error(w, "invalid message id", http.StatusBadRequest)
		return
	}

	var body requestBody
	validate := validator.New()

	err = json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		slog.Error("failed to decode body", "error", err)
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	body.Answer = strings.TrimSpace(body.Answer)

	if err := validate.Struct(&body); err != nil {
		slog.Error("validation failed", "error", err)

		missingFields := []string{}
		for _, err := range err.(validator.ValidationErrors) {
			if err.Tag() == "required" {
				missingFields = append(missingFields, err.Field())
			}

			if err.Tag() == "uuid" && err.Field() == "UserID" {
				http.Error(w, "validation failed: UserID must be a valid UUID", http.StatusBadRequest)
				return
			}
		}

		http.Error(w, "validation failed, missing required field(s): "+strings.Join(missingFields, ", "), http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(body.UserID)
	if err != nil {
		slog.Error("invalid user ID", "error", err)
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	user, ok := r.Context().Value(auth.UserKey).(pgstore.User)
	if !ok {
		slog.Error("user not found on the session cookie")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if user.ID != userID {
		slog.Error("the provided user ID is different from the session", "session", user.ID, "givenUserID", userID)
		http.Error(w, "invalid user ID", http.StatusForbidden)
		return
	}

	ctx := r.Context()
	status, err := h.RoomService.CheckRoomExists(ctx, roomID)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}

	status, err = h.MessageService.CheckMessageExists(ctx, messageID)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}

	err = h.MessageService.AnswerMessage(ctx, messageID, body.Answer)
	if err != nil {
		slog.Error("error setting message to answered", "error", err)
		if errors.Is(pgx.ErrNoRows, err) {
			http.Error(w, "the message has already been answered", http.StatusInternalServerError)
			return
		}

		http.Error(w, "error setting message to answered", http.StatusInternalServerError)
		return
	}

	sendJSON(w, types.MessageAnswered{
		ID:     rawMessageID,
		Answer: body.Answer,
	})

	go h.WebsocketService.NotifyRoomClient(types.Message{
		Kind:   types.MessageKindMessageAnswered,
		RoomID: roomID,
		Value: types.MessageAnswered{
			ID:     rawMessageID,
			Answer: body.Answer,
		},
	})
}

func (h *Handlers) GetRoomMessagesReactions(w http.ResponseWriter, r *http.Request) {
	type response struct {
		IDS []string `json:"ids"`
	}

	rawRoomID := chi.URLParam(r, "room_id")
	roomID, err := strconv.ParseInt(rawRoomID, 10, 64)
	if err != nil {
		http.Error(w, "invalid room id", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	status, err := h.RoomService.CheckRoomExists(ctx, roomID)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		slog.Error("user_id not provided")
		http.Error(w, "validation failed, missing required field(s): UserID", http.StatusBadRequest)
		return
	}

	user, ok := r.Context().Value(auth.UserKey).(pgstore.User)
	if !ok {
		slog.Error("user not found on the session cookie")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if user.ID.String() != userID {
		slog.Error("the provided user ID is different from the session")
		http.Error(w, "invalid user ID", http.StatusForbidden)
		return
	}

	ids, err := h.MessageService.GetRoomMessagesReactions(ctx, roomID, user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(ids) == 0 {
		ids = []string{}
	}

	sendJSON(w, response{IDS: ids})
}

func (h *Handlers) GetUserInfo(w http.ResponseWriter, r *http.Request) {
	rawUser := r.Context().Value(auth.UserKey)
	user, ok := rawUser.(pgstore.User)
	if !ok {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}

	sendJSON(w, user)
}

func (h *Handlers) DeleteUserInfo(w http.ResponseWriter, r *http.Request) {
	rawUserID := chi.URLParam(r, "user_id")
	userID, err := uuid.Parse(rawUserID)
	if err != nil {
		slog.Error("unable to parse user id", "error", err)
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	user, ok := ctx.Value(auth.UserKey).(pgstore.User)
	if !ok {
		slog.Error("user not found on the session cookie")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if user.ID != userID {
		slog.Error("the provided user ID is different from the session")
		http.Error(w, "invalid user ID", http.StatusForbidden)
		return
	}

	deletedID, err := h.UserService.DeleteUserInfo(ctx, userID)
	if err != nil || deletedID != userID {
		slog.Error("error deleting user info", "error", err, "deletedID", deletedID, "userID", userID)
		http.Error(w, "error deleting user info", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, auth.GenerateExpiredCookie())
	err = auth.DeleteSession(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	return
}

func (h *Handlers) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	type requestBody struct {
		UserID        string `json:"user_id" validate:"required,uuid"`
		Name          string `json:"name" validate:"required"`
		EnablePicture *bool  `json:"enable_picture"`
	}

	var body requestBody
	validate := validator.New()

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		slog.Error("failed to decode body", "error", err)
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	missingFields := []string{}
	if body.EnablePicture == nil {
		missingFields = append(missingFields, "EnablePicture")
	}
	if err := validate.Struct(&body); err != nil || len(missingFields) > 0 {
		slog.Error("validation failed", "error", err)

		if err != nil {
			for _, err := range err.(validator.ValidationErrors) {
				if err.Tag() == "required" {
					missingFields = append(missingFields, err.Field())
				}

				if err.Tag() == "uuid" && err.Field() == "UserID" {
					http.Error(w, "validation failed: UserID must be a valid UUID", http.StatusBadRequest)
					return
				}
			}
		}

		http.Error(w, "validation failed, missing required field(s): "+strings.Join(missingFields, ", "), http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(body.UserID)
	if err != nil {
		slog.Error("invalid user ID", "error", err)
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	user, ok := ctx.Value(auth.UserKey).(pgstore.User)
	if !ok {
		slog.Error("user not found on the session cookie")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if user.ID != userID {
		slog.Error("the provided user ID is different from the session")
		http.Error(w, "invalid user ID", http.StatusForbidden)
		return
	}

	newUser, updatedAt, err := h.UserService.UpdateUser(ctx, user.ID, body.Name, *body.EnablePicture)
	if err != nil {
		slog.Error("error updating user", "error", err)
		http.Error(w, "error updating user", http.StatusInternalServerError)
		return
	}

	user.UpdatedAt = updatedAt
	user.NewUser = newUser
	user.Name = body.Name
	user.EnablePicture = *body.EnablePicture
	err = auth.SetSessionData(ctx, user)
	if err != nil {
		http.Error(w, "error updating user", http.StatusInternalServerError)
		return
	}

	type responseBody struct {
		ID            uuid.UUID `json:"id"`
		Name          string    `json:"name"`
		EnablePicture bool      `json:"enable_picture"`
		NewUser       bool      `json:"new_user"`
		UpdatedAt     string    `json:"updated_at"`
	}

	sendJSON(w, responseBody{
		ID:            user.ID,
		Name:          body.Name,
		EnablePicture: *body.EnablePicture,
		NewUser:       newUser,
		UpdatedAt:     updatedAt.Time.Format(time.RFC3339),
	})

	return
}

func (h *Handlers) SubscribeToRoom(w http.ResponseWriter, r *http.Request) {
	rawRoomID := chi.URLParam(r, "room_id")
	roomID, err := strconv.ParseInt(rawRoomID, 10, 64)
	if err != nil {
		http.Error(w, "invalid room id", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	status, err := h.RoomService.CheckRoomExists(ctx, roomID)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}

	c, err := h.WebsocketService.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("failed to upgrade connection", "error", err)
		http.Error(w, "failed to connect to ws connection", http.StatusBadRequest)
		return
	}

	defer c.Close()

	ctx, cancel := context.WithCancel(r.Context())
	h.WebsocketService.SubscribeToRoom(c, ctx, cancel, roomID, r.RemoteAddr)
}

func (h Handlers) SubscribeToRoomsList(w http.ResponseWriter, r *http.Request) {
	c, err := h.WebsocketService.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("failed to upgrade connection", "error", err)
		http.Error(w, "failed to connect to ws connection", http.StatusBadRequest)
		return
	}

	defer c.Close()

	ctx, cancel := context.WithCancel(r.Context())
	h.WebsocketService.SubscribeToRoomsList(c, ctx, cancel, r.RemoteAddr)
}
