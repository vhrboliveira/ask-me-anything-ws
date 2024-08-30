package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/vhrboliveira/ama-go/internal/web"
)

func TestCreateRoom(t *testing.T) {
	const (
		url    = "/api/rooms"
		method = http.MethodPost
	)

	t.Run("creates a room", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		payload := strings.NewReader(`{"name": "Learning Go"}`)
		rr := execRequest(method, url, payload)

		response := rr.Result()
		defer response.Body.Close()

		var result struct {
			ID string `json:"id"`
		}

		assertStatusCode(t, response, http.StatusCreated)

		body := parseResponseBody(t, response)

		if err := json.Unmarshal(body, &result); err != nil {
			t.Errorf("Error to unmarshal body: %v", err)
		}

		assertValidUUID(t, result.ID)
	})

	t.Run("sends a message to the websocket subscribers when a room is created", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		server := httptest.NewServer(Router)
		defer server.Close()

		wsURL := "ws" + server.URL[4:] + "/subscribe"
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("failed to connect to websocket: %v", err)
		}
		defer ws.Close()

		want := "Learning Go"
		payload := strings.NewReader(`{"name": "` + want + `"}`)
		rr := execRequest(method, url, payload)

		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusCreated)

		body := parseResponseBody(t, response)

		var result struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			t.Errorf("Error to unmarshal body: %v", err)
		}

		// Read the message from WebSocket
		_, p, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read message from websocket: %v", err)
		}

		var receivedMessage web.Message
		var roomCreated web.RoomCreated

		if err := json.Unmarshal(p, &receivedMessage); err != nil {
			t.Fatalf("failed to unmarshal received message: %v", err)
		}

		jsonBytes, err := json.Marshal(receivedMessage.Value)

		if err != nil {
			t.Fatalf("failed to marshal received message value: %v", err)
		}
		if err := json.Unmarshal(jsonBytes, &roomCreated); err != nil {
			t.Fatalf("failed to unmarshal RoomCreated value: %v", err)
		}

		assertResponse(t, receivedMessage.Kind, web.MessageKindRoomCreated)
		assertResponse(t, roomCreated.ID, result.ID)
		assertResponse(t, roomCreated.Name, want)
	})

	t.Run("returns an error if request body is invalid", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		payload := strings.NewReader(`{ "invalid": "field" }`)
		rr := execRequest(method, url, payload)

		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusBadRequest)

		body := parseResponseBody(t, response)

		want := "validation failed: missing required field(s): name\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if request body is not a valid JSON", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		payload := strings.NewReader("aaaaaaa")
		rr := execRequest(method, url, payload)

		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusBadRequest)

		body := parseResponseBody(t, response)

		want := "invalid body\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns error if payload provides multiples rooms", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTables(t)
		})

		payload := strings.NewReader(`[{"name": "Learning Go"}, {"name": "Learning Rust"}]`)
		rr := execRequest(method, url, payload)

		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusBadRequest)

		body := parseResponseBody(t, response)

		want := "invalid body\n"
		assertResponse(t, want, string(body))
	})

	t.Run("returns an error if fails to insert room", func(t *testing.T) {
		name := "Learning Go"
		truncateTables(t)
		setRoomsConstraintFailure(t)

		payload := strings.NewReader(`{"name": "` + name + `"}`)
		rr := execRequest(method, url, payload)

		response := rr.Result()
		defer response.Body.Close()

		assertStatusCode(t, response, http.StatusInternalServerError)

		body := parseResponseBody(t, response)

		want := "error creating room\n"
		assertResponse(t, want, string(body))

	})
}
