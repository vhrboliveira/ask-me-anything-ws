package api_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/vhrboliveira/ama-go/internal/store/pgstore"
)

func TestGetRoomMessages(t *testing.T) {
	const (
		baseURL = "/api/rooms/"
		method  = "GET"
	)

	t.Run("returns room messages list", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		room := createAndGetRoom(t)
		msgs := []pgstore.InsertMessageParams{
			{
				RoomID:  room.ID,
				Message: "message 1",
			},
			{
				RoomID:  room.ID,
				Message: "message 2",
			},
		}
		insertMessages(t, msgs)

		newURL := baseURL + room.ID.String() + "/messages"
		rr := execRequest(t, method, newURL, nil)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusOK)

		var results []pgstore.GetMessageRow

		body := parseResponseBody(t, response)

		if err := json.Unmarshal(body, &results); err != nil {
			t.Errorf("failed to unmarshal response body: %v", err)
		}

		if lMsg, lRes := len(msgs), len(results); lMsg != lRes {
			t.Errorf("Expected %d message(s), Got: %d", lMsg, lRes)
		}

		expectedMsgs := make(map[string]bool, len(msgs))
		for _, msg := range msgs {
			expectedMsgs[msg.Message] = true
		}

		for i, result := range results {
			assertValidUUID(t, result.ID.String())
			assertValidDate(t, result.CreatedAt.Time.Format(time.RFC3339))

			if _, ok := expectedMsgs[result.Message]; !ok {
				t.Errorf("Unexpected message at index %d: %q", i, result.Message)
			}

			delete(expectedMsgs, result.Message)
		}
	})

	t.Run("returns token not found error if token is not found", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		fakeID := uuid.New().String()
		newURL := baseURL + fakeID + "/messages"
		rr := execRequestWithoutAuth(method, newURL, nil)
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

		fakeID := uuid.New().String()
		newURL := baseURL + fakeID + "/messages"
		rr := execRequestWithInvalidAuth(method, newURL, nil)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusUnauthorized)

		body := parseResponseBody(t, response)

		want := "token is unauthorized\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if room id is not valid", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		newURL := baseURL + "invalid_id/messages"
		rr := execRequest(t, method, newURL, nil)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusBadRequest)

		body := parseResponseBody(t, response)

		want := "invalid room id\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if room does not exist", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		fakeID := uuid.New().String()
		newURL := baseURL + fakeID + "/messages"
		rr := execRequest(t, method, newURL, nil)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusBadRequest)

		body := parseResponseBody(t, response)

		want := "room not found\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if fails to get room", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})
		setRoomsConstraintFailure(t)

		fakeID := uuid.New().String()
		newURL := baseURL + fakeID + "/messages"
		rr := execRequest(t, method, newURL, nil)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusInternalServerError)

		body := parseResponseBody(t, response)

		want := "error getting room\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns empty room messages list if room has no messages", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		room := createAndGetRoom(t)
		newURL := baseURL + room.ID.String() + "/messages"
		rr := execRequest(t, method, newURL, nil)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusOK)

		body := parseResponseBody(t, response)

		want := "[]"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if fails to get room messages", func(t *testing.T) {
		t.Cleanup(func() {
		})
		setMessagesConstraintFailure(t)

		room := createAndGetRoom(t)
		newURL := baseURL + room.ID.String() + "/messages"
		rr := execRequest(t, method, newURL, nil)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusInternalServerError)

		body := parseResponseBody(t, response)

		want := "error getting room messages\n"
		assertResponse(t, want, string(body))
	})

	t.Run("/api/rooms/{room_id}/messages/{message_id}", func(t *testing.T) {
		t.Run("returns message", func(t *testing.T) {
			t.Cleanup(func() {
				truncateTables(t)
			})

			room := createAndGetRoom(t)
			messageID := createAndGetMessages(t, room.ID)

			newURL := baseURL + room.ID.String() + "/messages/" + messageID
			rr := execRequest(t, method, newURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			assertStatusCode(t, response, http.StatusOK)

			body := parseResponseBody(t, response)

			var result pgstore.GetMessageRow

			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatalf("Error to unmarshal body: %q. Error: %v", body, err)
			}

			assertValidUUID(t, result.ID.String())

			want := "message 1"
			assertResponse(t, want, result.Message)
			assertResponse(t, room.ID.String(), result.RoomID.String())

			if !result.CreatedAt.Valid {
				t.Errorf("Expected created at to be not empty: %v", result.CreatedAt)
			}

			if result.Answered {
				t.Errorf("Expected answered to be false: %v", result.Answered)
			}

			if result.ReactionCount != 0 {
				t.Errorf("Expected reaction count to be 0: %v", result.ReactionCount)
			}
		})

		t.Run("returns an error if room id is not valid", func(t *testing.T) {
			t.Cleanup(func() {
				truncateTables(t)
			})

			fakeID := uuid.New().String()
			newURL := baseURL + "invalid_room_id/messages/" + fakeID
			rr := execRequest(t, method, newURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			assertStatusCode(t, response, http.StatusBadRequest)

			body := parseResponseBody(t, response)

			want := "invalid room id\n"
			assertResponse(t, want, string(body))
		})

		t.Run("returns an error if room does not exist", func(t *testing.T) {
			t.Cleanup(func() {
				truncateTables(t)
			})

			fakeID := uuid.New().String()
			newURL := baseURL + fakeID + "/messages/" + fakeID
			rr := execRequest(t, method, newURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			assertStatusCode(t, response, http.StatusBadRequest)

			body := parseResponseBody(t, response)

			want := "room not found\n"
			assertResponse(t, want, string(body))
		})

		t.Run("returns an error if message id is not valid", func(t *testing.T) {
			t.Cleanup(func() {
				truncateTables(t)
			})

			room := createAndGetRoom(t)
			newURL := baseURL + room.ID.String() + "/messages/invalid_message_id"
			rr := execRequest(t, method, newURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			assertStatusCode(t, response, http.StatusBadRequest)

			body := parseResponseBody(t, response)

			want := "invalid message id\n"
			assertResponse(t, want, string(body))
		})

		t.Run("returns an error if message does not exist", func(t *testing.T) {
			t.Cleanup(func() {
				truncateTables(t)
			})

			room := createAndGetRoom(t)
			fakeID := uuid.New().String()
			newURL := baseURL + room.ID.String() + "/messages/" + fakeID
			rr := execRequest(t, method, newURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			assertStatusCode(t, response, http.StatusNotFound)

			body := parseResponseBody(t, response)

			want := "message not found\n"
			assertResponse(t, want, string(body))
		})

		t.Run("returns an error if fails to get message", func(t *testing.T) {
			t.Cleanup(func() {
				truncateTables(t)
			})
			setMessagesConstraintFailure(t)

			room := createAndGetRoom(t)
			fakeID := uuid.New().String()
			newURL := baseURL + room.ID.String() + "/messages/" + fakeID
			rr := execRequest(t, method, newURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			assertStatusCode(t, response, http.StatusInternalServerError)

			body := parseResponseBody(t, response)

			want := "error getting message\n"
			assertResponse(t, want, string(body))
		})
	})
}
