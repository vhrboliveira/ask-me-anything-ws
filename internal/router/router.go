package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/vhrboliveira/ama-go/internal/web"
)

func SetupRouter(h *web.Handlers) *chi.Mux {
	router := chi.NewRouter()

	auth.InitJWT(os.Getenv("JWT_SECRET"))

	router.Use(middleware.RequestID, middleware.Recoverer, middleware.Logger)
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	router.Get("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	router.Post("/login", h.Login)
	router.Post("/user", h.CreateUser)

	router.Route("/subscribe", func(router chi.Router) {
		router.Get("/", h.SubscribeToRoomsList)
		router.Get("/room/{room_id}", h.SubscribeToRoom)
	})

	router.Route("/api", func(router chi.Router) {
		router.Route("/rooms", func(router chi.Router) {
			router.Post("/", h.CreateRoom)
			router.Get("/", h.GetRooms)

			router.Route("/{room_id}/messages", func(router chi.Router) {
				router.Post("/", h.CreateRoomMessage)
				router.Get("/", h.GetRoomMessages)

				router.Route("/{message_id}", func(router chi.Router) {
					router.Get("/", h.GetRoomMessage)
					router.Patch("/react", h.ReactionToMessage)
					router.Delete("/react", h.RemoveReactionFromMessage)
					router.Patch("/answer", h.SetMessageToAnswered)
				})
			})
		})
	})

	return router
}
