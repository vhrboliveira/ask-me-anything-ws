package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/vhrboliveira/ama-go-react/internal/store/pgstore"
)

type apiHandler struct {
	q           *pgstore.Queries
	r           *chi.Mux
	upgrader    websocket.Upgrader
	subscribers map[string]map[*websocket.Conn]context.CancelFunc
	mutex       *sync.RWMutex
}

func (h apiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.r.ServeHTTP(w, r)
}

func NewHandler(q *pgstore.Queries) http.Handler {
	h := apiHandler{
		q:           q,
		r:           chi.NewRouter(),
		upgrader:    websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		subscribers: make(map[string]map[*websocket.Conn]context.CancelFunc),
		mutex:       &sync.RWMutex{},
	}

	h.r.Use(middleware.RequestID, middleware.Recoverer, middleware.Logger)

	h.r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	h.r.Get("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	h.r.Get("/subscribe/{room_id}", h.subscribe)

	h.r.Route("/api", func(r chi.Router) {
		r.Route("/rooms", func(r chi.Router) {
			r.Post("/", h.createRoom)
			r.Get("/", h.getRooms)

			r.Route("/{room_id}/messages", func(r chi.Router) {
				r.Post("/", h.createRoomMessage)
				r.Get("/", h.getRoomMessages)

				r.Route("/{message_id}", func(r chi.Router) {
					r.Get("/", h.getRoomMessage)
					r.Patch("/react", h.reactionToMessage)
					r.Delete("/react", h.removeReactionFromMessage)
				})
			})
		})
	})

	return h
}

func (h apiHandler) subscribe(w http.ResponseWriter, r *http.Request) {
	_, rawRoomID, _, ok := h.readRoom(w, r)
	if !ok {
		return
	}

	c, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("failed to upgrade connection", "error", err)
		http.Error(w, "failed to connect to ws connection", http.StatusBadRequest)
		return
	}

	defer c.Close()

	ctx, cancel := context.WithCancel(r.Context())

	h.mutex.Lock()
	if _, ok := h.subscribers[rawRoomID]; !ok {
		h.subscribers[rawRoomID] = make(map[*websocket.Conn]context.CancelFunc)
	}
	slog.Info("new client connected", "room_id", rawRoomID, "client_IP", r.RemoteAddr)
	h.subscribers[rawRoomID][c] = cancel
	h.mutex.Unlock()

	<-ctx.Done()

	h.mutex.Lock()
	delete(h.subscribers[rawRoomID], c)
	h.mutex.Unlock()
}

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

func (h apiHandler) notifyClient(msg Message) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	subscribers, ok := h.subscribers[msg.RoomID]
	if !ok || len(subscribers) == 0 {
		return
	}

	for conn, cancel := range subscribers {
		if err := conn.WriteJSON(msg); err != nil {
			slog.Error("failed to write message to client", "error", err)
			cancel()
		}
	}
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
