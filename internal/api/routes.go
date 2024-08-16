package api

import (
	"context"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/gorilla/websocket"
	"github.com/vhrboliveira/ama-go-react/internal/store/pgstore"
)

type apiHandler struct {
	q                    *pgstore.Queries
	r                    *chi.Mux
	upgrader             websocket.Upgrader
	roomSubscribers      map[string]map[*websocket.Conn]context.CancelFunc
	roomsListSubscribers map[*websocket.Conn]context.CancelFunc
	mutex                *sync.RWMutex
}

func (h apiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.r.ServeHTTP(w, r)
}

func NewHandler(q *pgstore.Queries) http.Handler {
	h := apiHandler{
		q:                    q,
		r:                    chi.NewRouter(),
		upgrader:             websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		roomSubscribers:      make(map[string]map[*websocket.Conn]context.CancelFunc),
		roomsListSubscribers: make(map[*websocket.Conn]context.CancelFunc),
		mutex:                &sync.RWMutex{},
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

	h.r.Route("/subscribe", func(r chi.Router) {
		r.Get("/", h.subscribeToRoomsList)
		r.Get("/room/{room_id}", h.subscribeToRoom)
	})

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
					r.Patch("/answer", h.setMessageToAnswered)
				})
			})
		})
	})

	return h
}
