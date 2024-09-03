package web

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/vhrboliveira/ama-go/internal/auth"
	"github.com/vhrboliveira/ama-go/internal/store/pgstore"
	"golang.org/x/crypto/bcrypt"
)

type Handlers struct {
	Queries              *pgstore.Queries
	Router               *chi.Mux
	Upgrader             websocket.Upgrader
	RoomSubscribers      map[string]map[*websocket.Conn]context.CancelFunc
	RoomsListSubscribers map[*websocket.Conn]context.CancelFunc
	Mutex                *sync.RWMutex
}

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Bio   string `json:"bio"`
}

func NewHandler(q *pgstore.Queries) *Handlers {
	return &Handlers{
		Queries:              q,
		Router:               chi.NewRouter(),
		Upgrader:             websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		RoomSubscribers:      make(map[string]map[*websocket.Conn]context.CancelFunc),
		RoomsListSubscribers: make(map[*websocket.Conn]context.CancelFunc),
		Mutex:                &sync.RWMutex{},
	}
}

func (h Handlers) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.Router.ServeHTTP(w, r)
}

func (h Handlers) CreateRoom(w http.ResponseWriter, r *http.Request) {
	type requestBody struct {
		Name string `json:"name" validate:"required"`
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
		msg := "validation failed: missing required field(s): name"
		slog.Error(msg, "error", err)
		http.Error(w, msg, http.StatusBadRequest)
		return

	}

	roomID, err := h.Queries.InsertRoom(r.Context(), body.Name)
	if err != nil {
		slog.Error("error creating room", "error", err)
		http.Error(w, "error creating room", http.StatusInternalServerError)
		return
	}

	type response struct {
		ID string `json:"id"`
	}

	w.WriteHeader(http.StatusCreated)
	sendJSON(w, response{ID: roomID.String()})

	go h.NotifyRoomsListClients(Message{
		Kind:   MessageKindRoomCreated,
		RoomID: roomID.String(),
		Value: RoomCreated{
			ID:   roomID.String(),
			Name: body.Name,
		},
	})
}

func (h *Handlers) GetRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.Queries.GetRooms(r.Context())
	if err != nil {
		slog.Error("error getting rooms list", "error", err)
		http.Error(w, "error getting rooms list", http.StatusInternalServerError)
		return
	}

	if rooms == nil {
		rooms = []pgstore.GetRoomsRow{}
	}

	sendJSON(w, rooms)
}

func (h Handlers) CreateRoomMessage(w http.ResponseWriter, r *http.Request) {
	_, rawRoomID, roomID, ok := h.readRoom(w, r)
	if !ok {
		return
	}

	type roomMessageRequestBody struct {
		Message string `json:"message" validate:"required"`
	}

	var body roomMessageRequestBody
	validate := validator.New()

	err := json.NewDecoder(r.Body).Decode(&body)
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

	messageID, err := h.Queries.InsertMessage(r.Context(), pgstore.InsertMessageParams{
		RoomID:  roomID,
		Message: body.Message,
	})

	if err != nil {
		slog.Error("error inserting message", "error", err)
		http.Error(w, "error inserting message", http.StatusInternalServerError)
		return
	}

	type response struct {
		ID string `json:"id"`
	}

	w.WriteHeader(http.StatusCreated)
	sendJSON(w, response{ID: messageID.String()})

	go h.NotifyRoomClient(Message{
		Kind:   MessageKindMessageCreated,
		Value:  MessageCreated{ID: messageID.String(), Message: body.Message},
		RoomID: rawRoomID,
	})
}

func (h Handlers) GetRoomMessages(w http.ResponseWriter, r *http.Request) {
	_, _, roomID, ok := h.readRoom(w, r)
	if !ok {
		return
	}

	roomMessages, err := h.Queries.GetRoomMessages(r.Context(), roomID)
	if err != nil {
		slog.Error("error getting room messages", "error", err)
		http.Error(w, "error getting room messages", http.StatusInternalServerError)
		return
	}

	if roomMessages == nil {
		roomMessages = []pgstore.GetRoomMessagesRow{}
	}

	sendJSON(w, roomMessages)
}

func (h Handlers) GetRoomMessage(w http.ResponseWriter, r *http.Request) {
	_, _, _, ok := h.readRoom(w, r)
	if !ok {
		return
	}

	rawMessageID := chi.URLParam(r, "message_id")
	messageID, err := uuid.Parse(rawMessageID)
	if err != nil {
		slog.Error("error parsing message id", "error", err)
		http.Error(w, "invalid message id", http.StatusBadRequest)
		return
	}

	message, err := h.Queries.GetMessage(r.Context(), messageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Error("message not found", "error", err)
			http.Error(w, "message not found", http.StatusNotFound)
			return
		}

		slog.Error("error getting message", "error", err)
		http.Error(w, "error getting message", http.StatusInternalServerError)
		return
	}

	sendJSON(w, message)
}

func (h Handlers) ReactionToMessage(w http.ResponseWriter, r *http.Request) {
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

	_, err = h.Queries.GetMessage(r.Context(), messageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Error("message not found", "error", err)
			http.Error(w, "message not found", http.StatusNotFound)
			return
		}

		slog.Error("error getting message", "error", err)
		http.Error(w, "error getting message", http.StatusInternalServerError)
		return
	}

	count, err := h.Queries.ReactToMessage(r.Context(), messageID)
	if err != nil {
		slog.Error("error reacting to message", "error", err)
		http.Error(w, "error reacting to message", http.StatusInternalServerError)
		return
	}

	type response struct {
		Count int32 `json:"count"`
	}

	sendJSON(w, response{Count: count})

	go h.NotifyRoomClient(Message{
		Kind:   MessageKindMessageReactionAdd,
		RoomID: rawRoomID,
		Value: MessageReactionAdded{
			ID:    rawMessageID,
			Count: count,
		},
	})
}

func (h Handlers) RemoveReactionFromMessage(w http.ResponseWriter, r *http.Request) {
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

	_, err = h.Queries.GetMessage(r.Context(), messageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Error("message not found", "error", err)
			http.Error(w, "message not found", http.StatusNotFound)
			return
		}

		slog.Error("error getting message", "error", err)
		http.Error(w, "error getting message", http.StatusInternalServerError)
		return
	}

	count, err := h.Queries.RemoveReactionFromMessage(r.Context(), messageID)
	if err != nil {
		count = 0

		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("error reacting to message", "error", err)
			http.Error(w, "error reacting to message", http.StatusInternalServerError)
			return
		}
	}

	type response struct {
		Count int32 `json:"count"`
	}

	sendJSON(w, response{Count: count})

	go h.NotifyRoomClient(Message{
		Kind:   MessageKindMessageReactionRemoved,
		RoomID: rawRoomID,
		Value: MessageReactionRemoved{
			ID:    rawMessageID,
			Count: count,
		},
	})
}

func (h Handlers) SetMessageToAnswered(w http.ResponseWriter, r *http.Request) {
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

	_, err = h.Queries.GetMessage(r.Context(), messageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Error("message not found", "error", err)
			http.Error(w, "message not found", http.StatusNotFound)
			return
		}

		slog.Error("error getting message", "error", err)
		http.Error(w, "error getting message", http.StatusInternalServerError)
		return
	}

	err = h.Queries.MarkMessageAsAnswered(r.Context(), messageID)
	if err != nil {
		slog.Error("error setting message to answered", "error", err)
		http.Error(w, "error setting message to answered", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)

	go h.NotifyRoomClient(Message{
		Kind:   MessageKindMessageAnswered,
		RoomID: rawRoomID,
		Value: MessageAnswered{
			ID: rawMessageID,
		},
	})
}

func (h Handlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	type requestBody struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required,min=12"`
		Name     string `json:"name" validate:"required"`
		Bio      string `json:"bio"`
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

			if err.Tag() == "email" && err.Field() == "Email" {
				http.Error(w, "validation failed: Email must be a valid email address", http.StatusBadRequest)
				return
			}

			if err.Tag() == "min" && err.Field() == "Password" {
				http.Error(w, "validation failed: Password must be at least 12 characters", http.StatusBadRequest)
				return
			}
		}

		http.Error(w, "validation failed, missing required field(s): "+strings.Join(missingFields, ", "), http.StatusBadRequest)
		return
	}

	bytePassword := []byte(body.Password)
	hashedPassword, err := bcrypt.GenerateFromPassword(bytePassword, bcrypt.DefaultCost)
	if err != nil {
		slog.Error("failed to hash password", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	userID, err := h.Queries.CreateUser(r.Context(), pgstore.CreateUserParams{
		Email:        body.Email,
		PasswordHash: string(hashedPassword),
		Name:         body.Name,
		Bio:          body.Bio,
	})
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			slog.Error("user already exists", "error", err)
			http.Error(w, "user already exists", http.StatusBadRequest)
			return
		}
		slog.Error("error creating user", "error", err)
		http.Error(w, "error creating user", http.StatusInternalServerError)
		return
	}

	type response struct {
		ID string `json:"id"`
	}

	w.WriteHeader(http.StatusCreated)
	sendJSON(w, response{ID: userID.String()})
}

func (h Handlers) Login(w http.ResponseWriter, r *http.Request) {
	type requestBody struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required"`
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

			if err.Tag() == "email" && err.Field() == "Email" {
				http.Error(w, "validation failed: Email must be a valid email address", http.StatusBadRequest)
				return
			}
		}

		http.Error(w, "validation failed, missing required field(s): "+strings.Join(missingFields, ", "), http.StatusBadRequest)
		return
	}

	user, err := h.Queries.GetUserByEmail(r.Context(), body.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Error("user not found", "error", err)
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		slog.Error("error getting user", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password))
	if err != nil {
		slog.Error("invalid password", "error", err)
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := auth.GenerateJWT(user.ID, user.Email)
	if err != nil {
		slog.Error("failed to generate token", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	type response struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}

	sendJSON(w, response{
		ID:    user.ID.String(),
		Token: token,
	})
}
