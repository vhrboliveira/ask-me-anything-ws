package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func TestSubscribeToRoom(t *testing.T) {
	t.Run("subscribes to room", func(t *testing.T) {
		server := httptest.NewServer(Router)
		defer server.Close()

		room := createAndGetRoom(t)
		wsURL := "ws" + server.URL[4:] + "/subscribe/room/" + room.ID.String()
		headers := http.Header{}
		headers.Add("Authorization", "Bearer "+getAuthToken())
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
		if err != nil {
			t.Fatalf("failed to connect to websocket: %v", err)
		}
		defer ws.Close()
	})

	t.Run("returns token not found error if token is not found when subscribing to room", func(t *testing.T) {
		server := httptest.NewServer(Router)
		defer server.Close()

		fakeID := uuid.New().String()
		wsURL := "ws" + server.URL[4:] + "/subscribe/room/" + fakeID
		_, res, err := websocket.DefaultDialer.Dial(wsURL, nil)

		assertStatusCode(t, res, http.StatusUnauthorized)

		body := parseResponseBody(t, res)
		wantRes := "no token found\n"
		assertResponse(t, wantRes, string(body))

		wantErr := "websocket: bad handshake"
		assertResponse(t, wantErr, err.Error())
	})

	t.Run("returns authentication error if token is invalid when subscribing to room", func(t *testing.T) {
		server := httptest.NewServer(Router)
		defer server.Close()

		fakeID := uuid.New().String()
		wsURL := "ws" + server.URL[4:] + "/subscribe/room/" + fakeID
		token := getAuthToken()
		invalidToken := token[:len(token)-1]
		headers := http.Header{"Authorization": {"Bearer " + invalidToken}}
		_, res, err := websocket.DefaultDialer.Dial(wsURL, headers)

		assertStatusCode(t, res, http.StatusUnauthorized)

		body := parseResponseBody(t, res)
		wantRes := "token is unauthorized\n"
		assertResponse(t, wantRes, string(body))

		wantErr := "websocket: bad handshake"
		assertResponse(t, wantErr, err.Error())
	})

	t.Run("subscribes to room list", func(t *testing.T) {
		// Create a test server
		server := httptest.NewServer(Router)
		defer server.Close()

		// Connect to WebSocket
		wsURL := "ws" + server.URL[4:] + "/subscribe"
		headers := http.Header{}
		headers.Add("Authorization", "Bearer "+getAuthToken())
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
		if err != nil {
			t.Fatalf("failed to connect to websocket: %v", err)
		}
		defer ws.Close()
	})

	t.Run("returns token not found error if token is not found when subscribing to room list", func(t *testing.T) {
		server := httptest.NewServer(Router)
		defer server.Close()

		wsURL := "ws" + server.URL[4:] + "/subscribe"
		_, res, err := websocket.DefaultDialer.Dial(wsURL, nil)

		assertStatusCode(t, res, http.StatusUnauthorized)

		body := parseResponseBody(t, res)
		wantRes := "no token found\n"
		assertResponse(t, wantRes, string(body))

		wantErr := "websocket: bad handshake"
		assertResponse(t, wantErr, err.Error())
	})

	t.Run("returns authentication error if token is invalid when subscribing to room list", func(t *testing.T) {
		server := httptest.NewServer(Router)
		defer server.Close()

		wsURL := "ws" + server.URL[4:] + "/subscribe"
		token := getAuthToken()
		invalidToken := token[:len(token)-1]
		headers := http.Header{"Authorization": {"Bearer " + invalidToken}}
		_, res, err := websocket.DefaultDialer.Dial(wsURL, headers)

		assertStatusCode(t, res, http.StatusUnauthorized)

		body := parseResponseBody(t, res)
		wantRes := "token is unauthorized\n"
		assertResponse(t, wantRes, string(body))

		wantErr := "websocket: bad handshake"
		assertResponse(t, wantErr, err.Error())
	})

}
