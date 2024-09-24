package api_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/valkey-io/valkey-go"

	"github.com/vhrboliveira/ama-go/internal/router"
	"github.com/vhrboliveira/ama-go/internal/service"
	"github.com/vhrboliveira/ama-go/internal/store/pgstore"
	"github.com/vhrboliveira/ama-go/internal/web"
)

var DBPool *pgxpool.Pool
var Handler *web.Handlers
var Router *chi.Mux
var ValkeyClient valkey.Client

func TestMain(m *testing.M) {
	setup()

	// Run tests
	code := m.Run()

	// Stop Docker Compose
	teardown()

	os.Exit(code)
}

func setup() {
	var err error

	if err = godotenv.Load(".env.test"); err != nil {
		panic("Error loading .env.test file")
	}

	// Start Docker Compose
	if err = exec.Command(
		"docker",
		"compose",
		"--env-file", "./.env.test",
		"-f", "../../deploy/compose-test.yaml", "up", "-d").Run(); err != nil {
		panic("Could not start Docker Compose:" + err.Error())
	}

	// Set up the database connection
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"))

	DBPool, err = connectWithRetry(dsn, 5, 1*time.Second)
	if err != nil {
		panic("Unable to create connection pool:" + err.Error())
	}

	if err := runMigrations(); err != nil {
		panic("Failed to run migrations:" + err.Error())
	}

	VALKEY_ENDPOINT := os.Getenv("VALKEY_ENDPOINT")
	if VALKEY_ENDPOINT == "" {
		panic("VALKEY_ENDPOINT is not set")
	}
	ValkeyClient, err = valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{VALKEY_ENDPOINT},
	})
	if err != nil {
		slog.Error("Failed to initialize Valkey client", "error", err)
		panic(err)
	}

	q := pgstore.New(DBPool)
	roomService := service.NewRoomService(q)
	messageService := service.NewMessageService(q)
	userService := service.NewUserService(q)
	wsService := service.NewWebSocketService()
	Handler = web.NewHandler(roomService, messageService, userService, wsService)
	Router = router.SetupRouter(Handler, userService, &ValkeyClient)
}

func teardown() {
	DBPool.Close()

	err := exec.Command("docker", "compose", "--env-file", "./.env.test", "-f", "../../deploy/compose-test.yaml", "down").Run()
	if err != nil {
		panic("Could not stop Docker Compose:" + err.Error())
	}
}

func runMigrations() error {
	return exec.Command(
		"tern",
		"migrate",
		"--migrations",
		"../../internal/store/pgstore/migrations",
		"--config",
		"../../internal/store/pgstore/migrations/tern.conf",
	).Run()
}

func connectWithRetry(dsn string, maxRetries int, delay time.Duration) (*pgxpool.Pool, error) {
	var err error

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		panic("Unable to parse config:" + err.Error())
	}

	var errs []string

	for i := range maxRetries {
		DBPool, err = pgxpool.NewWithConfig(context.Background(), config)
		if err == nil {
			err = DBPool.Ping(context.Background())
			if err == nil {
				return DBPool, nil
			}
		}

		errs = append(errs, fmt.Sprintf("Failed to connect to database (attempt: %d/%d). Error: %v\n", i+1, maxRetries, err))
		time.Sleep(delay)
	}

	slog.Error(strings.Join(errs, ""))
	return nil, fmt.Errorf("failed to connect to database after %d attempts. Last error: %v", maxRetries, err)
}
