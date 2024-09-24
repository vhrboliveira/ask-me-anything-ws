package router

import (
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/valkey-io/valkey-go"
	"github.com/vhrboliveira/ama-go/internal/auth"
	"github.com/vhrboliveira/ama-go/internal/service"
	"github.com/vhrboliveira/ama-go/internal/web"
)

func SetupRouter(h *web.Handlers, userService *service.UserService, valkeyClient *valkey.Client) *chi.Mux {
	url := os.Getenv("SITE_URL")
	if url == "" {
		panic("SITE_URL is not set")
	}

	auth.AuthInit(valkeyClient, userService)
	router := chi.NewRouter()

	router.Use(middleware.RequestID, middleware.Recoverer, middleware.Logger)
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{url},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	router.Get("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	router.Route("/auth/{provider}", func(router chi.Router) {
		router.Get("/", auth.LoginHandler)
		router.Get("/callback", auth.CallbackHandler)
	})
	router.Get("/logout", auth.LogoutHandler)

	router.Group(func(router chi.Router) {
		router.Use(auth.AuthMiddleware)

		router.Route("/subscribe", func(router chi.Router) {
			router.Get("/", h.SubscribeToRoomsList)
			router.Get("/room/{room_id}", h.SubscribeToRoom)
		})

		router.Route("/api", func(router chi.Router) {
			router.Get("/user", h.GetUserInfo)

			router.Patch("/profile", h.UpdateProfile)

			router.Route("/rooms", func(router chi.Router) {
				router.Post("/", h.CreateRoom)
				router.Get("/", h.GetRooms)

				router.Route("/{room_id}", func(router chi.Router) {
					router.Get("/", h.GetRoom)
					router.Route("/messages", func(router chi.Router) {
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
		})
	})

	return router
}
