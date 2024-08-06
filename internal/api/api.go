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
		upgrader:    websocket.Upgrader{},
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

	return h
}

func (h apiHandler) subscribe(w http.ResponseWriter, r *http.Request) {
	rawRoomID := chi.URLParam(r, "room_id")
	roomID, err := uuid.Parse(rawRoomID)

	if err != nil {
		http.Error(w, "invalid room id", http.StatusBadRequest)
		return
	}

	_, err = h.q.GetRoom(r.Context(), roomID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "room not found", http.StatusBadRequest)
			return
		}

		slog.Error("error getting room", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
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
