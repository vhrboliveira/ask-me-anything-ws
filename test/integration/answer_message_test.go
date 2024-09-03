package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/vhrboliveira/ama-go/internal/web"
)

func TestAnswerMessage(t *testing.T) {
	const baseURL = "/api/rooms/"

	t.Run("sets message as answered", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		room := createAndGetRoom(t)
		msg := createAndGetMessages(t, room.ID)

		rr := execRequest(http.MethodPatch, baseURL+room.ID.String()+"/messages/"+msg+"/answer", nil)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusOK)
	})

	t.Run("sends a message to the websocket subscribers when a message is answered", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		room := createAndGetRoom(t)
		msg := createAndGetMessages(t, room.ID)

		server := httptest.NewServer(Router)
		defer server.Close()

		wsURL := "ws" + server.URL[4:] + "/subscribe/room/" + room.ID.String()
		headers := http.Header{}
		headers.Add("Authorization", "Bearer "+getAuthToken())
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
		if err != nil {
			t.Fatalf("failed to connect to websocket: %v", err)
		}
		defer ws.Close()

		rr := execRequest(http.MethodPatch, baseURL+room.ID.String()+"/messages/"+msg+"/answer", nil)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusOK)

		_, p, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read message from websocket: %v", err)
		}

		var receivedMessage web.Message
		var messageAnswered web.MessageAnswered
		if err := json.Unmarshal(p, &receivedMessage); err != nil {
			t.Fatalf("failed to unmarshal received message: %v", err)
		}

		jsonBytes, err := json.Marshal(receivedMessage.Value)
		if err != nil {
			t.Fatalf("failed to marshal received message value: %v", err)
		}
		if err := json.Unmarshal(jsonBytes, &messageAnswered); err != nil {
			t.Fatalf("failed to unmarshal MessageAnswered value: %v", err)
		}

		assertResponse(t, msg, messageAnswered.ID)
	})

	t.Run("returns token not found error if token is not found", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		fakeID := uuid.New().String()
		rr := execRequestWithoutAuth(http.MethodPatch, baseURL+fakeID+"/messages/"+fakeID+"/answer", nil)
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
		rr := execRequestWithInvalidAuth(http.MethodPatch, baseURL+fakeID+"/messages/"+fakeID+"/answer", nil)
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
		newURL := baseURL + "invalid_room_id/messages/" + fakeID + "/answer"
		rr := execRequest(http.MethodPatch, newURL, nil)
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
		newURL := baseURL + fakeID + "/messages/" + fakeID + "/answer"
		rr := execRequest(http.MethodPatch, newURL, nil)
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
		newURL := baseURL + room.ID.String() + "/messages/invalid_message_id/answer"
		rr := execRequest(http.MethodPatch, newURL, nil)
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
		newURL := baseURL + room.ID.String() + "/messages/" + fakeID + "/answer"
		rr := execRequest(http.MethodPatch, newURL, nil)
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
		newURL := baseURL + room.ID.String() + "/messages/" + fakeID + "/answer"
		rr := execRequest(http.MethodPatch, newURL, nil)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusInternalServerError)

		body := parseResponseBody(t, response)

		want := "error getting message\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if fails to set message as answered", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})
		room := createAndGetRoom(t)
		setAnswerMessageConstraintFailure(t, room.ID)

		msgID := createAndGetMessages(t, room.ID)
		newURL := baseURL + room.ID.String() + "/messages/" + msgID + "/answer"
		rr := execRequest(http.MethodPatch, newURL, nil)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusInternalServerError)

		body := parseResponseBody(t, response)

		want := "error setting message to answered\n"
		assertResponse(t, want, string(body))
	})
}
