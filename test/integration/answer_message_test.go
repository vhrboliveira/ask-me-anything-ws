package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

	t.Run("answers a message and set as answered", func(t *testing.T) {
		truncateData(t)

		room := createAndGetRoom(t)
		msgID, _ := createAndGetMessages(t, room.ID)
		answer := "This is the answer to this message"
		payload := strings.NewReader(`{"user_id": "` + room.UserID.String() + `", "answer": "` + answer + `"}`)
		rr := execAuthenticatedRequest(t, method, baseURL+room.ID.String()+"/messages/"+msgID+"/answer", payload)
		response := rr.Result()
		defer response.Body.Close()

		var body struct {
			ID     string `json:"id"`
			Answer string `json:"answer"`
		}
		require.NoError(t, json.NewDecoder(response.Body).Decode(&body))

		assert.Equal(t, msgID, body.ID)
		assert.Equal(t, answer, body.Answer)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})

	t.Run("returns an error if the user tries to answer a message twice or more", func(t *testing.T) {
		truncateData(t)

		room := createAndGetRoom(t)
		msgID, _ := createAndGetMessages(t, room.ID)
		answer := "This is the answer to this message"
		payload := strings.NewReader(`{"user_id": "` + room.UserID.String() + `", "answer": "` + answer + `"}`)
		_ = execAuthenticatedRequest(t, method, baseURL+room.ID.String()+"/messages/"+msgID+"/answer", payload)

		anotherAnswer := "I'm trying to change the previous answer"
		payload = strings.NewReader(`{"user_id": "` + room.UserID.String() + `", "answer": "` + anotherAnswer + `"}`)
		rr := execAuthenticatedRequest(t, method, baseURL+room.ID.String()+"/messages/"+msgID+"/answer", payload)
		response := rr.Result()
		defer response.Body.Close()

		body := parseResponseBody(t, response)
		want := "the message has already been answered\n"

		assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
		assert.Equal(t, want, body)
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

		answer := "This is the answer to this message"
		payload := strings.NewReader(`{"user_id": "` + room.UserID.String() + `", "answer": "` + answer + `"}`)
		rr := execAuthenticatedRequest(t, method, baseURL+room.ID.String()+"/messages/"+msgID+"/answer", payload)
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

		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, receivedMessage.Kind, types.MessageKindMessageAnswered)
		assert.Equal(t, msgID, messageAnswered.ID)
		assert.Equal(t, answer, messageAnswered.Answer)
	})

	truncateData(t)
	fakeID := uuid.New().String()
	room := createAndGetRoom(t)
	gothUser := mockGothUser(nil)
	userID := getUserIDByEmail(t, gothUser.Email)
	answer := "any answer here"
	type constraintFn func(t *testing.T)
	var newURL string
	var newPayload string

	errorTestCases := []struct {
		name               string
		fn                 customFn
		payload            string
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
			payload:            `{"user_id": "` + userID + `", "answer": "` + answer + `"}`,
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
			payload:            `{"user_id": "` + userID + `", "answer": "` + answer + `"}`,
			expectedMessage:    "unauthorized, session not found or invalid\n",
			expectedStatusCode: http.StatusUnauthorized,
			url:                baseURL + fakeID + "/messages/" + fakeID + "/answer",
			setConstraint:      nil,
		},
		{
			name:               "returns an error if request body is invalid",
			fn:                 execAuthenticatedRequest,
			payload:            `{ "invalid": "field" }`,
			expectedMessage:    "validation failed, missing required field(s): UserID, Answer\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL + fakeID + "/messages/" + fakeID + "/answer",
			setConstraint:      nil,
		},
		{
			name:               "returns an error if request body is not a valid JSON",
			fn:                 execAuthenticatedRequest,
			payload:            "aaaaaaa",
			expectedMessage:    "invalid body\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL + fakeID + "/messages/" + fakeID + "/answer",
			setConstraint:      nil,
		},
		{
			name:               "returns an error if room id is not valid",
			fn:                 execAuthenticatedRequest,
			payload:            `{"user_id": "` + userID + `", "answer": "` + answer + `"}`,
			expectedMessage:    "invalid room id\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL + "invalid_room_id/messages/" + fakeID + "/answer",
			setConstraint:      nil,
		},
		{
			name:               "returns an error if user ID is not provided",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "validation failed, missing required field(s): UserID\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL + fakeID + "/messages/" + fakeID + "/answer",
			payload:            `{"answer": "` + answer + `"}`,
			setConstraint:      nil,
		},
		{
			name:               "returns an error if user ID is empty",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "validation failed, missing required field(s): UserID\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL + fakeID + "/messages/" + fakeID + "/answer",
			payload:            `{"answer": "` + answer + `", "user_id": ""}`,
			setConstraint:      nil,
		},
		{
			name:               "returns an error if user ID is not a valid UUID",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "validation failed: UserID must be a valid UUID\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL + fakeID + "/messages/" + fakeID + "/answer",
			payload:            `{"user_id": "invalid-uuid", "answer": "` + answer + `"}`,
			setConstraint:      nil,
		},
		{
			name:               "returns an error if answer is not provided",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "validation failed, missing required field(s): Answer\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL + fakeID + "/messages/" + fakeID + "/answer",
			payload:            `{"user_id": "` + userID + `"}`,
			setConstraint:      nil,
		},
		{
			name:               "returns an error if answer is not a valid string",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "validation failed, missing required field(s): Answer\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL + fakeID + "/messages/" + fakeID + "/answer",
			payload:            `{"user_id": "` + userID + `", "answer": "    "}`,
			setConstraint:      nil,
		},
		{
			name:               "returns an error if room does not exist",
			fn:                 execAuthenticatedRequest,
			payload:            `{"user_id": "` + userID + `", "answer": "` + answer + `"}`,
			expectedMessage:    "room not found\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL + fakeID + "/messages/" + fakeID + "/answer",
			setConstraint:      nil,
		},
		{
			name:               "returns an error if message id is not valid",
			fn:                 execAuthenticatedRequest,
			payload:            `{"user_id": "` + userID + `", "answer": "` + answer + `"}`,
			expectedMessage:    "invalid message id\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL + room.ID.String() + "/messages/invalid_message_id/answer",
			setConstraint:      nil,
		},
		{
			name:               "returns an error if message does not exist",
			fn:                 execAuthenticatedRequest,
			payload:            `{"user_id": "` + userID + `", "answer": "` + answer + `"}`,
			expectedMessage:    "message not found\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL + room.ID.String() + "/messages/" + fakeID + "/answer",
			setConstraint:      nil,
		},
		{
			name:               "returns an error if fails to get message",
			fn:                 execAuthenticatedRequest,
			payload:            `{"user_id": "` + userID + `", "answer": "` + answer + `"}`,
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
			payload:            `{"user_id": "` + userID + `", "answer": "` + answer + `"}`,
			expectedMessage:    "error setting message to answered\n",
			expectedStatusCode: http.StatusInternalServerError,
			url:                baseURL,
			setConstraint: func(t *testing.T) {
				truncateData(t)

				room := createAndGetRoom(t)
				setAnswerMessageConstraintFailure(t, room.ID)
				msgID, _ := createAndGetMessages(t, room.ID)

				newURL = baseURL + room.ID.String() + "/messages/" + msgID + "/answer"
				newPayload = `{"user_id": "` + room.UserID.String() + `", "answer": "` + answer + `"}`
			},
		},
	}

	for _, tc := range errorTestCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setConstraint != nil {
				tc.setConstraint(t)
				if newURL != "" {
					tc.url = newURL
					tc.payload = newPayload
				}
			}

			payload := strings.NewReader(tc.payload)
			rr := tc.fn(t, method, tc.url, payload)
			response := rr.Result()
			defer response.Body.Close()

			body := parseResponseBody(t, response)

			assert.Equal(t, tc.expectedStatusCode, response.StatusCode)
			assert.Equal(t, tc.expectedMessage, body)
		})
	}
}
