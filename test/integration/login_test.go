package api_test

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/jwtauth/v5"
	"github.com/vhrboliveira/ama-go/internal/store/pgstore"
)

func TestLogin(t *testing.T) {
	const (
		url    = "/login"
		method = http.MethodPost
	)

	insertUser := func(t testing.TB, user pgstore.CreateUserParams) string {
		userPayload := strings.NewReader(`{ "email": "` + user.Email + `", "name": "` + user.Name + `", "password": "` + user.PasswordHash + `" }`)
		rr := execRequest(method, "/users", userPayload)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusCreated)

		body := parseResponseBody(t, response)

		type responseType struct {
			ID string `json:"id"`
		}

		var got responseType
		err := json.Unmarshal(body, &got)
		if err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		return got.ID
	}

	assertValidToken := func(t testing.TB, token string) {
		jwtSecret := os.Getenv("JWT_SECRET")
		tokenAuth := jwtauth.New("HS256", []byte(jwtSecret), nil)
		_, err := jwtauth.VerifyToken(tokenAuth, token)
		if err != nil {
			t.Fatalf("failed to parse token: %v", err)
		}
	}

	t.Run("returns a token and user id if login is successful", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		user := pgstore.CreateUserParams{
			Email:        "test@example.com",
			PasswordHash: "password123456789!",
			Name:         "Test User",
		}

		// Insert user with hashed password
		userID := insertUser(t, user)

		payload := strings.NewReader(`{"email": "` + user.Email + `", "password": "` + user.PasswordHash + `"}`)
		rr := execRequest(method, url, payload)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusOK)

		body := parseResponseBody(t, response)

		type responseType struct {
			ID    string `json:"id"`
			Token string `json:"token"`
		}

		var got responseType
		err := json.Unmarshal(body, &got)
		if err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		assertResponse(t, userID, got.ID)
		assertValidToken(t, got.Token)
	})

	t.Run("returns an error if body is invalid", func(t *testing.T) {
		payload := strings.NewReader(`invalid json`)
		rr := execRequest(method, url, payload)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusBadRequest)

		body := parseResponseBody(t, response)

		want := "invalid body\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if password is missing", func(t *testing.T) {
		payload := strings.NewReader(`{"email": "test@example.com"}`)
		rr := execRequest(method, url, payload)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusBadRequest)

		body := parseResponseBody(t, response)

		want := "validation failed, missing required field(s): Password\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if email is missing", func(t *testing.T) {
		payload := strings.NewReader(`{"password": "password"}`)
		rr := execRequest(method, url, payload)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusBadRequest)

		body := parseResponseBody(t, response)

		want := "validation failed, missing required field(s): Email\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if email is invalid", func(t *testing.T) {
		payload := strings.NewReader(`{"email": "test.com", "password": "password"}`)
		rr := execRequest(method, url, payload)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusBadRequest)

		body := parseResponseBody(t, response)

		want := "validation failed: Email must be a valid email address\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns invalid credentials error if user does not exist", func(t *testing.T) {
		payload := strings.NewReader(`{"email": "test@example.com", "password": "password"}`)
		rr := execRequest(method, url, payload)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusUnauthorized)

		body := parseResponseBody(t, response)

		want := "invalid credentials\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if password is incorrect", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		user := pgstore.CreateUserParams{
			Email:        "test@example.com",
			PasswordHash: "password123456789!",
			Name:         "Test User",
		}

		// Insert user with hashed password
		insertUser(t, user)

		payload := strings.NewReader(`{"email": "` + user.Email + `", "password": "incorrect"}`)
		rr := execRequest(method, url, payload)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusUnauthorized)

		body := parseResponseBody(t, response)

		want := "invalid credentials\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if something goes wrong in the server", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})
		setCreateUserConstraintError(t)

		payload := strings.NewReader(`{"email": "test@example.com", "password": "password"}`)
		rr := execRequest(method, url, payload)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusInternalServerError)

		body := parseResponseBody(t, response)

		want := "internal server error\n"
		assertResponse(t, want, string(body))
	})
}
