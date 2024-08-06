package main

import (
	"log/slog"
	"os/exec"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Error("error loading .env file.")
		panic(err)
	}

	cmd := exec.Command(
		"tern",
		"migrate",
		"--migrations",
		"./internal/store/pgstore/migrations",
		"--config",
		"./internal/store/pgstore/migrations/tern.conf",
	)

	if err := cmd.Run(); err != nil {
		slog.Error("error running migrations")
		panic(err)
	}
}
