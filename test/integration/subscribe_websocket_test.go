package api_test

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/vhrboliveira/ama-go/internal/web"
)

func TestSubscribeToRoom(t *testing.T) {
	t.Run("subscribes to room", func(t *testing.T) {
		server := httptest.NewServer(Router)
		defer server.Close()

		room := createAndGetRoom(t)
		wsURL := "ws" + server.URL[4:] + "/subscribe/room/" + room.ID.String()
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("failed to connect to websocket: %v", err)
		}
		defer ws.Close()

		testMessage := web.Message{
			Kind:   "test",
			RoomID: room.ID.String(),
			Value:  "Test message",
		}

		// Notify room clients (simulating a message being sent)
		Handler.NotifyRoomClient(testMessage)

		// Read the message from WebSocket
		_, p, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read message from websocket: %v", err)
		}

		// Parse the received message
		var receivedMessage web.Message
		if err := json.Unmarshal(p, &receivedMessage); err != nil {
			t.Fatalf("failed to unmarshal received message: %v", err)
		}
	})

	t.Run("subscribes to room list", func(t *testing.T) {
		// Create a test server
		server := httptest.NewServer(Router)
		defer server.Close()

		// Connect to WebSocket
		wsURL := "ws" + server.URL[4:] + "/subscribe"
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("failed to connect to websocket: %v", err)
		}
		defer ws.Close()
		testMessage := []web.Message{
			{
				Kind:   "test",
				RoomID: "roomID",
				Value:  "Room 1",
			},
			{
				Kind:   "test",
				RoomID: "roomID2",
				Value:  "Room 2",
			},
		}

		// Notify room clients (simulating a message being sent)
		Handler.NotifyRoomsListClients(testMessage[0])
		Handler.NotifyRoomsListClients(testMessage[1])

		// Read the message from WebSocket
		_, p, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read message from websocket: %v", err)
		}

		// Parse the received message
		var receivedMessage web.Message
		if err := json.Unmarshal(p, &receivedMessage); err != nil {
			t.Fatalf("failed to unmarshal received message: %v", err)
		}

		_, p, err = ws.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read message from websocket: %v", err)
		}

		if err := json.Unmarshal(p, &receivedMessage); err != nil {
			t.Fatalf("failed to unmarshal received message: %v", err)
		}
	})
}
