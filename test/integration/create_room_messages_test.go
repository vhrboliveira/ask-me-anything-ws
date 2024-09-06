package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/vhrboliveira/ama-go/internal/web"
)

func TestCreateRoomMessages(t *testing.T) {
	const (
		url    = "/api/rooms/room_id/messages"
		method = http.MethodPost
	)

	t.Run("create messages for a room", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		room := createAndGetRoom(t)
		newURL := strings.Replace(url, "room_id", room.ID.String(), 1)
		payload := strings.NewReader(`{"message": "Is Go awesome?"}`)
		rr := execRequest(t, method, newURL, payload)

		response := rr.Result()
		defer response.Body.Close()

		var result struct {
			ID        string `json:"id"`
			CreatedAt string `json:"created_at"`
		}

		assertStatusCode(t, response, http.StatusCreated)

		body := parseResponseBody(t, response)

		if err := json.Unmarshal(body, &result); err != nil {
			t.Errorf("Error to unmarshal body: %v", err)
		}

		assertValidUUID(t, result.ID)
		assertValidDate(t, result.CreatedAt)
	})

	t.Run("sends a message to the websocket subscribers when a message is created in a room", func(t *testing.T) {
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

		want := "Is Go awesome?"
		newURL := strings.Replace(url, "room_id", room.ID.String(), 1)
		payload := strings.NewReader(`{"message": "` + want + `"}`)
		rr := execRequest(t, method, newURL, payload)

		response := rr.Result()
		defer response.Body.Close()

		var result struct {
			ID        string `json:"id"`
			CreatedAt string `json:"created_at"`
		}

		assertStatusCode(t, response, http.StatusCreated)

		body := parseResponseBody(t, response)

		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("Error to unmarshal body: %v", err)
		}

		assertValidUUID(t, result.ID)
		assertValidDate(t, result.CreatedAt)

		_, p, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read message from websocket: %v", err)
		}

		var receivedMessage web.Message
		var messageCreated web.MessageCreated

		if err := json.Unmarshal(p, &receivedMessage); err != nil {
			t.Fatalf("failed to unmarshal received message: %v", err)
		}

		jsonBytes, err := json.Marshal(receivedMessage.Value)
		if err != nil {
			t.Fatalf("failed to marshal received message value: %v", err)
		}

		if err := json.Unmarshal(jsonBytes, &messageCreated); err != nil {
			t.Fatalf("failed to unmarshal RoomCreated value: %v", err)
		}

		assertResponse(t, receivedMessage.Kind, web.MessageKindMessageCreated)
		assertResponse(t, messageCreated.ID, result.ID)
		assertResponse(t, messageCreated.Message, want)
		assertValidDate(t, messageCreated.CreatedAt)
	})

	t.Run("returns token not found error if token is not found", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		fakeID := uuid.New().String()
		newURL := strings.Replace(url, "room_id", fakeID, 1)
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
		newURL := strings.Replace(url, "room_id", fakeID, 1)
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

		rr := execRequest(t, method, url, nil)
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
		newURL := strings.Replace(url, "room_id", fakeID, 1)
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
		newURL := strings.Replace(url, "room_id", fakeID, 1)
		rr := execRequest(t, method, newURL, nil)
		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusInternalServerError)

		body := parseResponseBody(t, response)

		want := "error getting room\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if body is invalid", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		room := createAndGetRoom(t)
		newURL := strings.Replace(url, "room_id", room.ID.String(), 1)
		payload := strings.NewReader(`{"invalid": "invalid"}`)
		rr := execRequest(t, method, newURL, payload)

		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusBadRequest)

		body := parseResponseBody(t, response)

		want := "validation failed: missing required field(s): message\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if request body is not a valid json", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		room := createAndGetRoom(t)
		newURL := strings.Replace(url, "room_id", room.ID.String(), 1)
		payload := strings.NewReader(`aaaaaa`)
		rr := execRequest(t, method, newURL, payload)

		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusBadRequest)

		body := parseResponseBody(t, response)

		want := "invalid body\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if payload provides multiples messages", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		room := createAndGetRoom(t)
		newURL := strings.Replace(url, "room_id", room.ID.String(), 1)
		payload := strings.NewReader(`[{"message": "a valid message"}, {"message": "another valid message"}]`)
		rr := execRequest(t, method, newURL, payload)

		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusBadRequest)

		body := parseResponseBody(t, response)

		want := "invalid body\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if fails to insert message", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})
		setMessagesConstraintFailure(t)

		room := createAndGetRoom(t)
		newURL := strings.Replace(url, "room_id", room.ID.String(), 1)
		payload := strings.NewReader(`{"message": "a valid message"}`)
		rr := execRequest(t, method, newURL, payload)

		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusInternalServerError)

		body := parseResponseBody(t, response)

		want := "error inserting message\n"
		assertResponse(t, want, string(body))
	})
}
