package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	types "github.com/vhrboliveira/ama-go/internal/utils"
)

func TestAnswerMessage(t *testing.T) {
	type customFn func(t testing.TB, method string, url string, body io.Reader) *httptest.ResponseRecorder

	const (
		baseURL = "/api/rooms/"
		method  = http.MethodPatch
	)

	t.Run("sets message as answered", func(t *testing.T) {
		truncateData(t)

		room := createAndGetRoom(t)
		msgID, _ := createAndGetMessages(t, room.ID)
		// userID := "any" // get the user ID from the room
		// answer := "This is the answer to this message"
		// payload := strings.NewReader(`{"user_id": "` + userID + `", "answer": "` + answer + `"}`)
		rr := execAuthenticatedRequest(t, method, baseURL+room.ID.String()+"/messages/"+msgID+"/answer", nil)
		response := rr.Result()
		defer response.Body.Close()

		assert.Equal(t, response.StatusCode, http.StatusOK)
	})

	t.Run("sends a message to the websocket subscribers when a message is answered", func(t *testing.T) {
		truncateData(t)

		room := createAndGetRoom(t)
		msgID, _ := createAndGetMessages(t, room.ID)

		server := httptest.NewServer(Router)
		defer server.Close()

		wsURL := "ws" + server.URL[4:] + "/subscribe/room/" + room.ID.String()
		ws, err := connectAuthenticatedWS(t, wsURL)
		require.NoError(t, err)
		defer ws.Close()

		rr := execAuthenticatedRequest(t, method, baseURL+room.ID.String()+"/messages/"+msgID+"/answer", nil)
		response := rr.Result()
		defer response.Body.Close()

		// Read the message from WebSocket
		_, p, err := ws.ReadMessage()
		require.NoError(t, err)

		var receivedMessage types.Message
		var messageAnswered types.MessageAnswered

		require.NoError(t, json.Unmarshal(p, &receivedMessage), "failed to unmarshal received message")

		jsonBytes, err := json.Marshal(receivedMessage.Value)
		require.NoError(t, err, "failed to marsha received message value")
		require.NoError(t, json.Unmarshal(jsonBytes, &messageAnswered), "failed to unmarshal MessageAnswered value")

		assert.Equal(t, response.StatusCode, http.StatusOK)
		assert.Equal(t, receivedMessage.Kind, types.MessageKindMessageAnswered)
		assert.Equal(t, msgID, messageAnswered.ID)
	})

	truncateData(t)
	fakeID := uuid.New().String()
	room := createAndGetRoom(t)
	type constraintFn func(t *testing.T)
	var newURL string

	errorTestCases := []struct {
		name               string
		fn                 customFn
		expectedMessage    string
		expectedStatusCode int
		url                string
		setConstraint      constraintFn
	}{
		{
			name: "returns unauthorized error if sessionID is not found",
			fn: func(t testing.TB, method, url string, body io.Reader) *httptest.ResponseRecorder {
				return execRequestWithoutCookie(method, url, body)
			},
			expectedMessage:    "unauthorized, session not found or invalid\n",
			expectedStatusCode: http.StatusUnauthorized,
			url:                baseURL + fakeID + "/messages/" + fakeID + "/answer",
			setConstraint:      nil,
		},
		{
			name: "returns unauthorized error if cookie is different from the session",
			fn: func(t testing.TB, method, url string, body io.Reader) *httptest.ResponseRecorder {
				return execRequestWithInvalidCookie(method, url, body)
			},
			expectedMessage:    "unauthorized, session not found or invalid\n",
			expectedStatusCode: http.StatusUnauthorized,
			url:                baseURL + fakeID + "/messages/" + fakeID + "/answer",
			setConstraint:      nil,
		},
		{
			name:               "returns an error if room id is not valid",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "invalid room id\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL + "invalid_room_id/messages/" + fakeID + "/answer",
			setConstraint:      nil,
		},
		{
			name:               "returns an error if room does not exist",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "room not found\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL + fakeID + "/messages/" + fakeID + "/answer",
			setConstraint:      nil,
		},
		{
			name:               "returns an error if message id is not valid",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "invalid message id\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL + room.ID.String() + "/messages/invalid_message_id/answer",
			setConstraint:      nil,
		},
		{
			name:               "returns an error if message does not exist",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "message not found\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL + room.ID.String() + "/messages/" + fakeID + "/answer",
			setConstraint:      nil,
		},
		{
			name:               "returns an error if fails to get message",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "error validating message ID\n",
			expectedStatusCode: http.StatusInternalServerError,
			url:                baseURL + room.ID.String() + "/messages/" + fakeID + "/answer",
			setConstraint: func(t *testing.T) {
				setMessagesConstraintFailure(t)
			},
		},
		{
			name:               "returns an error if fails to set message as answered",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "error setting message to answered\n",
			expectedStatusCode: http.StatusInternalServerError,
			url:                baseURL,
			setConstraint: func(t *testing.T) {
				truncateData(t)

				room := createAndGetRoom(t)
				setAnswerMessageConstraintFailure(t, room.ID)
				msgID, _ := createAndGetMessages(t, room.ID)

				newURL = baseURL + room.ID.String() + "/messages/" + msgID + "/answer"
			},
		},
	}

	for _, tc := range errorTestCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setConstraint != nil {
				tc.setConstraint(t)
				if newURL != "" {
					tc.url = newURL
				}
			}

			rr := tc.fn(t, method, tc.url, nil)
			response := rr.Result()
			defer response.Body.Close()

			body := parseResponseBody(t, response)

			assert.Equal(t, response.StatusCode, tc.expectedStatusCode)
			assert.Equal(t, tc.expectedMessage, body)
		})
	}
}
