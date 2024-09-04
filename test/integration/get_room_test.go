package api_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestGetRoom(t *testing.T) {
	const (
		url    = "/api/rooms"
		method = http.MethodGet
	)

	t.Run("returns rooms list", func(t *testing.T) {
		roomNames := []string{"learning Go", "learning Rust"}

		truncateTables(t)
		createRooms(t, roomNames)

		rr := execRequest(method, url, nil)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusOK)

		var results []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			CreatedAt string `json:"created_at"`
		}

		body := parseResponseBody(t, response)

		if err := json.Unmarshal(body, &results); err != nil {
			t.Fatalf("Error to unmarshal body: %q. Error: %v", body, err)
		}

		if lRoom, lRes := len(roomNames), len(results); lRoom != lRes {
			t.Errorf("Expected %d room(s), Got: %d", lRoom, lRes)
		}

		expectedNames := make(map[string]bool, len(roomNames))
		for _, name := range roomNames {
			expectedNames[name] = true
		}

		for _, result := range results {
			assertValidUUID(t, result.ID)
			assertValidDate(t, result.CreatedAt)

			if _, ok := expectedNames[result.Name]; !ok {
				t.Errorf("Unexpected room name: %s", result.Name)
			}

			delete(expectedNames, result.Name)
		}
	})

	t.Run("returns empty rooms list if there is no rooms", func(t *testing.T) {
		truncateTables(t)

		rr := execRequest(method, url, nil)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusOK)
		body := parseResponseBody(t, response)

		want := "[]"
		assertResponse(t, want, string(body))
	})

	t.Run("returns token not found error if token is not found", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		rr := execRequestWithoutAuth(method, url, nil)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusUnauthorized)

		body := parseResponseBody(t, response)

		want := "no token found\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns authentication error if token is invalid", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		rr := execRequestWithInvalidAuth(method, url, nil)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusUnauthorized)

		body := parseResponseBody(t, response)

		want := "token is unauthorized\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if database returns an error", func(t *testing.T) {
		truncateTables(t)
		setRoomsConstraintFailure(t)

		rr := execRequest(method, url, nil)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusInternalServerError)
		body := parseResponseBody(t, response)

		want := "error getting rooms list\n"
		assertResponse(t, want, string(body))
	})
}
