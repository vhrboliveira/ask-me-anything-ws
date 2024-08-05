package main

import (
	"context"
	"errors"
	"fmt"
	"log"
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
		log.Default().Panic("error loading .env file: ", err)
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
		log.Default().Panic("error connecting to database: ", err)
	}

	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Default().Panic("error pinging database: ", err)
	}

	handler := api.NewHandler(pgstore.New(pool))

	go func() {
		log.Default().Println("server running on port 5001")
		if err := http.ListenAndServe(":5001", handler); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Default().Panic("error serving handler: ", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
}
