package api_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscribeToRoom(t *testing.T) {
	server := httptest.NewServer(Router)
	defer server.Close()
	baseURL := "ws" + server.URL[4:] + "/subscribe"

	testCases := []struct {
		name    string
		addRoom bool
	}{
		{
			name:    "subscribes to room",
			addRoom: true,
		},
		{
			name:    "subscribes to room list",
			addRoom: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			truncateData(t)
			wsURL := baseURL

			if tc.addRoom {
				room := createAndGetRoom(t)
				wsURL += "/room/" + strconv.Itoa(int(room.ID))
			}

			ws, err := connectAuthenticatedWS(t, wsURL)
			require.NoError(t, err)

			defer ws.Close()
		})

		t.Run("returns unauthorized error if sessionID is not found", func(t *testing.T) {
			wsURL := baseURL

			if tc.addRoom {
				wsURL += "/room/" + uuid.New().String()
			}

			_, res, err := websocket.DefaultDialer.Dial(wsURL, nil)

			body := parseResponseBody(t, res)
			wantRes := "unauthorized, session not found or invalid\n"
			wantErr := "websocket: bad handshake"

			assert.Equal(t, res.StatusCode, http.StatusUnauthorized)
			assert.Equal(t, wantRes, body)
			assert.Equal(t, wantErr, err.Error())
		})

		t.Run("returns unauthorized error if cookie is different from the session", func(t *testing.T) {
			wsURL := baseURL

			if tc.addRoom {
				wsURL += "/room/" + uuid.New().String()
			}

			_, res, err := connectWSWithoutSession(t, wsURL)

			body := parseResponseBody(t, res)
			wantRes := "unauthorized, session not found or invalid\n"
			wantErr := "websocket: bad handshake"

			assert.Equal(t, res.StatusCode, http.StatusUnauthorized)
			assert.Equal(t, wantRes, body)
			assert.Equal(t, wantErr, err.Error())
		})
	}
}
