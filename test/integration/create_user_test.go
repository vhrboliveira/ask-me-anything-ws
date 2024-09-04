package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/vhrboliveira/ama-go/internal/store/pgstore"
)

func TestCreateUser(t *testing.T) {
	const (
		baseURL = "/users"
		method  = http.MethodPost
	)

	t.Run("create user", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		user := pgstore.CreateUserParams{
			Email:        "test@example.com",
			PasswordHash: "password12345678",
			Name:         "Test User",
			Bio:          "Test Bio",
		}

		payload := strings.NewReader(fmt.Sprintf(`{"email": "%s", "password": "%s", "name": "%s", "bio": "%s"}`, user.Email, user.PasswordHash, user.Name, user.Bio))
		rr := execRequest(method, baseURL, payload)

		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusCreated)

		body := parseResponseBody(t, response)
		var result struct {
			ID        string `json:"id"`
			Email     string `json:"email"`
			Name      string `json:"name"`
			Bio       string `json:"bio"`
			CreatedAt string `json:"created_at"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to unmarshal response body: %v", err)
		}

		assertValidUUID(t, result.ID)
		assertValidDate(t, result.CreatedAt)
		assertResponse(t, user.Email, result.Email)
		assertResponse(t, user.Name, result.Name)
		assertResponse(t, user.Bio, result.Bio)
	})

	t.Run("returns an error if request body is invalid", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		payload := strings.NewReader(`invalid`)
		rr := execRequest(method, baseURL, payload)

		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusBadRequest)

		body := parseResponseBody(t, response)

		want := "invalid body\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if required fields are missing", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		payload := strings.NewReader(`{ "invalid": "field" }`)
		rr := execRequest(method, baseURL, payload)

		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusBadRequest)

		body := parseResponseBody(t, response)

		want := "validation failed, missing required field(s): Email, Password, Name\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if email is missing", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		payload := strings.NewReader(`{ "password": "password12345678", "name": "Test User" }`)
		rr := execRequest(method, baseURL, payload)

		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusBadRequest)

		body := parseResponseBody(t, response)

		want := "validation failed, missing required field(s): Email\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if password is missing", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		payload := strings.NewReader(`{ "email": "test@example.com", "name": "Test User" }`)
		rr := execRequest(method, baseURL, payload)

		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusBadRequest)

		body := parseResponseBody(t, response)

		want := "validation failed, missing required field(s): Password\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if name is missing", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		payload := strings.NewReader(`{ "email": "test@example.com", "password": "password12345678" }`)
		rr := execRequest(method, baseURL, payload)

		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusBadRequest)

		body := parseResponseBody(t, response)

		want := "validation failed, missing required field(s): Name\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if email is not valid", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		payload := strings.NewReader(`{ "email": "testexample.com", "password": "password12345678", "name": "Test User" }`)
		rr := execRequest(method, baseURL, payload)

		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusBadRequest)

		body := parseResponseBody(t, response)

		want := "validation failed: Email must be a valid email address\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if password is shorter than 12 characters", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		payload := strings.NewReader(`{ "email": "test@example.com", "password": "short", "name": "Test User" }`)
		rr := execRequest(method, baseURL, payload)

		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusBadRequest)

		body := parseResponseBody(t, response)

		want := "validation failed: Password must be at least 12 characters\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns error when email is already taken", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		user := pgstore.CreateUserParams{
			Email:        "test@example.com",
			PasswordHash: "password12345678",
			Name:         "Test User",
		}

		createUser(t, user.Email, user.PasswordHash, user.Name)

		payload := strings.NewReader(fmt.Sprintf(`{"email": "%s", "password": "%s", "name": "%s"}`, user.Email, user.PasswordHash, user.Name))
		rr := execRequest(method, baseURL, payload)

		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusBadRequest)

		body := parseResponseBody(t, response)

		want := "user already exists\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if fails to create user", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		setCreateUserConstraintError(t)

		payload := strings.NewReader(`{"email": "test@example.com", "password": "password12345678", "name": "Test User"}`)
		rr := execRequest(method, baseURL, payload)

		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusInternalServerError)

		body := parseResponseBody(t, response)

		want := "error creating user\n"
		assertResponse(t, want, string(body))
	})
}
