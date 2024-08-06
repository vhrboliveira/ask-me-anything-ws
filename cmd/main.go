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
	"github.com/vhrboliveira/ama-go-react/internal/api"
	"github.com/vhrboliveira/ama-go-react/internal/store/pgstore"
)

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Error("unable to load .env file")
		panic(err)
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, fmt.Sprintf(
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

	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("error pinging database.")
		panic(err)
	}

	handler := api.NewHandler(pgstore.New(pool))

	go func() {
		slog.Info("server running on port 5001...")
		if err := http.ListenAndServe(":5001", handler); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("error serving handler.")
			panic(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
}
