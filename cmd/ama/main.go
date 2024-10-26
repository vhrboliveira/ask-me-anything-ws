package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/valkey-io/valkey-go"
	"github.com/vhrboliveira/ama-go/internal/router"
	"github.com/vhrboliveira/ama-go/internal/service"
	"github.com/vhrboliveira/ama-go/internal/store/pgstore"
	"github.com/vhrboliveira/ama-go/internal/web"
)

var pool *pgxpool.Pool

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Error("unable to load .env file")
		panic(err)
	}

	connectToDB()
	defer pool.Close()

	// Initialize Valkey cache
	VALKEY_ENDPOINT := os.Getenv("VALKEY_ENDPOINT")
	if VALKEY_ENDPOINT == "" {
		panic("VALKEY_ENDPOINT is not set")
	}

	valkeyClient, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{VALKEY_ENDPOINT},
	})
	if err != nil {
		slog.Error("Failed to initialize Valkey client", "error", err)
		panic(err)
	}

	q := pgstore.New(pool)
	roomService := service.NewRoomService(q)
	messageService := service.NewMessageService(q)
	userService := service.NewUserService(q)
	wsService := service.NewWebSocketService()
	h := web.NewHandler(roomService, messageService, userService, wsService)

	router := router.SetupRouter(h, userService, &valkeyClient)

	port := os.Getenv("PORT")
	if port == "" {
		port = "5001"
	}

	go func() {
		slog.Info("server running on port " + port + "...")
		if err := http.ListenAndServe(":"+port, router); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("error serving handler.")
			panic(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
}

func connectToDB() {
	ctx := context.Background()
	var err error

	pool, err = pgxpool.New(ctx, fmt.Sprintf(
		"user=%s password=%s host=%s port=%s dbname=%s sslmode=disable",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME")),
	)

	if err != nil {
		slog.Error("error connecting to database.")
		panic(err)
	}

	if err := pool.Ping(ctx); err != nil {
		slog.Error("error pinging database.")
		panic(err)
	}
}
