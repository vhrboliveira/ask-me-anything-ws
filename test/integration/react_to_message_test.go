package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/vhrboliveira/ama-go/internal/web"
)

func TestMessageReaction(t *testing.T) {
	const (
		baseURL = "/api/rooms/"
	)

	t.Run("PATCH /api/rooms/{room_id}/messages/{message_id}/", func(t *testing.T) {
		t.Run("adds a reaction to a message", func(t *testing.T) {
			t.Cleanup(func() {
				truncateTables(t)
			})

			room := createAndGetRoom(t)
			msgID := createAndGetMessages(t, room.ID)
			newURL := baseURL + room.ID.String() + "/messages/" + msgID + "/react"
			rr := execRequest(t, http.MethodPatch, newURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			assertStatusCode(t, response, http.StatusOK)

			body := parseResponseBody(t, response)

			var result struct {
				Count int `json:"count"`
			}
			if err := json.Unmarshal(body, &result); err != nil {
				t.Errorf("Error to unmarshal body: %q. Error: %v", body, err)
			}

			want := "1"
			assertResponse(t, want, strconv.Itoa(result.Count))
		})

		t.Run("sends a message to the websocket subscribers when a reaction is added to a message", func(t *testing.T) {
			t.Cleanup(func() {
				truncateTables(t)
			})

			room := createAndGetRoom(t)

			server := httptest.NewServer(Router)
			defer server.Close()

			wsURL := "ws" + server.URL[4:] + "/subscribe/room/" + room.ID.String() + "?token=" + generateAuthToken(nil)
			headers := http.Header{}
			headers.Add("Authorization", "Bearer "+generateAuthToken(nil))
			ws, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
			if err != nil {
				t.Fatalf("failed to connect to websocket: %v", err)
			}
			defer ws.Close()

			msgID := createAndGetMessages(t, room.ID)
			newURL := baseURL + room.ID.String() + "/messages/" + msgID + "/react"
			rr := execRequest(t, http.MethodPatch, newURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			assertStatusCode(t, response, http.StatusOK)

			body := parseResponseBody(t, response)

			var result struct {
				Count int `json:"count"`
			}
			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatalf("Error to unmarshal body: %q. Error: %v", body, err)
			}

			want := "1"
			assertResponse(t, want, strconv.Itoa(result.Count))

			_, p, err := ws.ReadMessage()
			if err != nil {
				t.Fatalf("failed to read message from websocket: %v", err)
			}

			var receivedMessage web.Message
			var messageReactionAdded web.MessageReactionAdded
			if err := json.Unmarshal(p, &receivedMessage); err != nil {
				t.Fatalf("failed to unmarshal received message: %v", err)
			}

			jsonBytes, err := json.Marshal(receivedMessage.Value)
			if err != nil {
				t.Fatalf("failed to marshal received message value: %v", err)
			}
			if err := json.Unmarshal(jsonBytes, &messageReactionAdded); err != nil {
				t.Fatalf("failed to unmarshal MessageReactionAdded value: %v", err)
			}

			assertResponse(t, msgID, messageReactionAdded.ID)
			assertResponse(t, "1", strconv.Itoa(int(messageReactionAdded.Count)))
		})

		t.Run("adds simultaneously multiple reactions to a message", func(t *testing.T) {
			t.Cleanup(func() {
				truncateTables(t)
			})

			room := createAndGetRoom(t)
			msgID := createAndGetMessages(t, room.ID)
			newURL := baseURL + room.ID.String() + "/messages/" + msgID + "/react"

			var wg sync.WaitGroup
			const requests = 10

			wg.Add(requests)
			for i := 0; i < requests; i++ {
				go func() {
					defer wg.Done()
					rr := execRequest(t, http.MethodPatch, newURL, nil)
					response := rr.Result()
					defer response.Body.Close()
					assertStatusCode(t, response, http.StatusOK)
				}()
			}
			wg.Wait()

			reactions := getMessageReactions(t, msgID)

			if reactions != requests {
				t.Errorf("Expected %q reactions, got %q", requests, reactions)
			}
		})

		t.Run("returns token not found error if token is not found", func(t *testing.T) {
			t.Cleanup(func() {
				truncateTables(t)
			})

			fakeID := uuid.New().String()
			newURL := baseURL + fakeID + "/messages/" + fakeID + "/react"
			rr := execRequestWithoutAuth(http.MethodPatch, newURL, nil)
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
			newURL := baseURL + fakeID + "/messages/" + fakeID + "/react"
			rr := execRequestWithInvalidAuth(http.MethodPatch, newURL, nil)
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

			fakeID := uuid.New().String()
			newURL := baseURL + "invalid_room_id/messages/" + fakeID + "/react"
			rr := execRequest(t, http.MethodPatch, newURL, nil)
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
			newURL := baseURL + fakeID + "/messages/" + fakeID + "/react"
			rr := execRequest(t, http.MethodPatch, newURL, nil)
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
			newURL := baseURL + room.ID.String() + "/messages/invalid_message_id/react"
			rr := execRequest(t, http.MethodPatch, newURL, nil)
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
			newURL := baseURL + room.ID.String() + "/messages/" + fakeID + "/react"
			rr := execRequest(t, http.MethodPatch, newURL, nil)
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
			newURL := baseURL + room.ID.String() + "/messages/" + fakeID + "/react"
			rr := execRequest(t, http.MethodPatch, newURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			assertStatusCode(t, response, http.StatusInternalServerError)

			body := parseResponseBody(t, response)

			want := "error getting message\n"
			assertResponse(t, want, string(body))
		})

		t.Run("returns an error if fails to update message", func(t *testing.T) {
			t.Cleanup(func() {
				truncateTables(t)
			})
			room := createAndGetRoom(t)
			setUpdateMessageReactionConstraintFailure(t, room.ID)

			msgID := createAndGetMessages(t, room.ID)
			newURL := baseURL + room.ID.String() + "/messages/" + msgID + "/react"
			rr := execRequest(t, http.MethodPatch, newURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			assertStatusCode(t, response, http.StatusInternalServerError)

			body := parseResponseBody(t, response)

			want := "error reacting to message\n"
			assertResponse(t, want, string(body))
		})
	})

	t.Run("DELETE /api/rooms/{room_id}/messages/{message_id}/", func(t *testing.T) {
		t.Run("removes a reaction to a message", func(t *testing.T) {
			t.Cleanup(func() {
				truncateTables(t)
			})

			room := createAndGetRoom(t)
			msgID := createAndGetMessages(t, room.ID)
			setMessageReaction(t, msgID, 1)
			newURL := baseURL + room.ID.String() + "/messages/" + msgID + "/react"
			rr := execRequest(t, http.MethodDelete, newURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			assertStatusCode(t, response, http.StatusOK)

			body := parseResponseBody(t, response)

			var result struct {
				Count int `json:"count"`
			}
			if err := json.Unmarshal(body, &result); err != nil {
				t.Errorf("Error to unmarshal body: %q. Error: %v", body, err)
			}

			want := "0"
			assertResponse(t, want, strconv.Itoa(result.Count))
		})

		t.Run("sends a message to the websocket subscribers when a reaction is removed from a message", func(t *testing.T) {
			t.Cleanup(func() {
				truncateTables(t)
			})

			room := createAndGetRoom(t)

			server := httptest.NewServer(Router)
			defer server.Close()

			wsURL := "ws" + server.URL[4:] + "/subscribe/room/" + room.ID.String() + "?token=" + generateAuthToken(nil)
			headers := http.Header{}
			headers.Add("Authorization", "Bearer "+generateAuthToken(nil))
			ws, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
			if err != nil {
				t.Fatalf("failed to connect to websocket: %v", err)
			}
			defer ws.Close()

			msgID := createAndGetMessages(t, room.ID)
			setMessageReaction(t, msgID, 1)
			newURL := baseURL + room.ID.String() + "/messages/" + msgID + "/react"
			rr := execRequest(t, http.MethodDelete, newURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			assertStatusCode(t, response, http.StatusOK)

			body := parseResponseBody(t, response)

			var result struct {
				Count int `json:"count"`
			}
			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatalf("Error to unmarshal body: %q. Error: %v", body, err)
			}

			want := "0"
			assertResponse(t, want, strconv.Itoa(result.Count))

			_, p, err := ws.ReadMessage()
			if err != nil {
				t.Fatalf("failed to read message from websocket: %v", err)
			}

			var receivedMessage web.Message
			var messageReactionAdded web.MessageReactionAdded
			if err := json.Unmarshal(p, &receivedMessage); err != nil {
				t.Fatalf("failed to unmarshal received message: %v", err)
			}

			jsonBytes, err := json.Marshal(receivedMessage.Value)
			if err != nil {
				t.Fatalf("failed to marshal received message value: %v", err)
			}
			if err := json.Unmarshal(jsonBytes, &messageReactionAdded); err != nil {
				t.Fatalf("failed to unmarshal MessageReactionAdded value: %v", err)
			}

			assertResponse(t, msgID, messageReactionAdded.ID)
			assertResponse(t, "0", strconv.Itoa(int(messageReactionAdded.Count)))
		})

		t.Run("does not remove a reaction from a message if there is no reaction", func(t *testing.T) {
			t.Cleanup(func() {
				truncateTables(t)
			})

			room := createAndGetRoom(t)
			msgID := createAndGetMessages(t, room.ID)
			newURL := baseURL + room.ID.String() + "/messages/" + msgID + "/react"
			rr := execRequest(t, http.MethodDelete, newURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			assertStatusCode(t, response, http.StatusOK)

			body := parseResponseBody(t, response)

			var result struct {
				Count int `json:"count"`
			}
			if err := json.Unmarshal(body, &result); err != nil {
				t.Errorf("Error to unmarshal body: %q. Error: %v", body, err)
			}

			want := "0"
			assertResponse(t, want, strconv.Itoa(result.Count))
		})

		t.Run("returns token not found error if token is not found", func(t *testing.T) {
			t.Cleanup(func() {
				truncateTables(t)
			})

			fakeID := uuid.New().String()
			newURL := baseURL + fakeID + "/messages/" + fakeID + "/react"
			rr := execRequestWithoutAuth(http.MethodDelete, newURL, nil)
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
			newURL := baseURL + fakeID + "/messages/" + fakeID + "/react"
			rr := execRequestWithInvalidAuth(http.MethodDelete, newURL, nil)
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

			fakeID := uuid.New().String()
			newURL := baseURL + "invalid_room_id/messages/" + fakeID + "/react"
			rr := execRequest(t, http.MethodDelete, newURL, nil)
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
			newURL := baseURL + fakeID + "/messages/" + fakeID + "/react"
			rr := execRequest(t, http.MethodDelete, newURL, nil)
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
			newURL := baseURL + room.ID.String() + "/messages/invalid_message_id/react"
			rr := execRequest(t, http.MethodDelete, newURL, nil)
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
			newURL := baseURL + room.ID.String() + "/messages/" + fakeID + "/react"
			rr := execRequest(t, http.MethodDelete, newURL, nil)
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
			newURL := baseURL + room.ID.String() + "/messages/" + fakeID + "/react"
			rr := execRequest(t, http.MethodDelete, newURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			assertStatusCode(t, response, http.StatusInternalServerError)

			body := parseResponseBody(t, response)

			want := "error getting message\n"
			assertResponse(t, want, string(body))
		})

		t.Run("returns an error if fails to remove reaction from message", func(t *testing.T) {
			t.Cleanup(func() {
				truncateTables(t)
			})
			room := createAndGetRoom(t)
			msgID := createAndGetMessages(t, room.ID)
			setDeleteMessageReactionConstraintFailure(t, room.ID, msgID)
			newURL := baseURL + room.ID.String() + "/messages/" + msgID + "/react"
			rr := execRequest(t, http.MethodDelete, newURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			assertStatusCode(t, response, http.StatusInternalServerError)

			body := parseResponseBody(t, response)

			want := "error reacting to message\n"
			assertResponse(t, want, string(body))
		})
	})
}
