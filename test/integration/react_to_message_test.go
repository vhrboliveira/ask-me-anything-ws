package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vhrboliveira/ama-go/internal/store/pgstore"
	types "github.com/vhrboliveira/ama-go/internal/utils"
)

func TestMessageReaction(t *testing.T) {
	type customFn func(t testing.TB, method string, url string, body io.Reader) *httptest.ResponseRecorder

	const (
		baseURL = "/api/rooms/"
	)

	gothUser := mockGothUser(nil)

	t.Run("adds simultaneously multiple reactions to a message", func(t *testing.T) {
		truncateData(t)

		room := createAndGetRoom(t)
		msgID, _ := createAndGetMessages(t, room.ID)
		newURL := baseURL + room.ID.String() + "/messages/" + msgID + "/react"

		var wg sync.WaitGroup
		var userIDs []string
		const requests = 50

		for i := 0; i < requests; i++ {
			user := pgstore.User{
				Email:    strconv.Itoa(i+1) + "vitor@vhrbo.tech",
				Provider: "google",
			}
			userID := createUser(t, user.Email, user.Name, user.Provider, "", "")
			parsedUserID, _ := uuid.Parse(userID)
			user.ID = parsedUserID
			userIDs = append(userIDs, userID)
			generateSession(t, &user)
		}

		wg.Add(requests)
		for i := 0; i < requests; i++ {
			go func() {
				defer wg.Done()

				payload := strings.NewReader(`{"user_id": "` + userIDs[i] + `", "message_id": "` + msgID + `"}`)
				rr := execRequestGettingSession(t, http.MethodPatch, newURL, payload, userIDs[i])
				response := rr.Result()
				defer response.Body.Close()
				assert.Equal(t, http.StatusOK, response.StatusCode)
			}()
		}
		wg.Wait()

		reactions := getMessageReactions(t, msgID)

		assert.Equal(t, requests, reactions)
	})

	t.Run("sends a message to the websocket subscribers when a reaction is added on a message", func(t *testing.T) {
		truncateData(t)

		room := createAndGetRoom(t)

		server := httptest.NewServer(Router)
		defer server.Close()

		wsURL := "ws" + server.URL[4:] + "/subscribe/room/" + room.ID.String()
		ws, err := connectAuthenticatedWS(t, wsURL)
		require.NoError(t, err)
		defer ws.Close()

		msgID, _ := createAndGetMessages(t, room.ID)
		userID := getUserIDByEmail(t, gothUser.Email)

		newURL := baseURL + room.ID.String() + "/messages/" + msgID + "/react"
		payload := strings.NewReader(`{"user_id": "` + userID + `", "message_id": "` + msgID + `"}`)
		rr := execAuthenticatedRequest(t, http.MethodPatch, newURL, payload)
		response := rr.Result()
		defer response.Body.Close()

		var result struct {
			Count int `json:"count"`
		}
		require.NoError(t, json.NewDecoder(response.Body).Decode(&result))

		_, p, err := ws.ReadMessage()
		require.NoError(t, err)

		var receivedMessage types.Message
		var messageReactionAdded types.MessageReactionAdded
		require.NoError(t, json.Unmarshal(p, &receivedMessage))

		jsonBytes, err := json.Marshal(receivedMessage.Value)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(jsonBytes, &messageReactionAdded))

		expectedMessage := 1
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, expectedMessage, result.Count)
		assert.Equal(t, msgID, messageReactionAdded.ID)
		assert.Equal(t, expectedMessage, int(messageReactionAdded.Count))
	})

	t.Run("sends a message to the websocket subscribers when a reaction is removed from a message", func(t *testing.T) {
		truncateData(t)

		room := createAndGetRoom(t)

		server := httptest.NewServer(Router)
		defer server.Close()

		wsURL := "ws" + server.URL[4:] + "/subscribe/room/" + room.ID.String()
		ws, err := connectAuthenticatedWS(t, wsURL)
		require.NoError(t, err)
		defer ws.Close()

		msgID, _ := createAndGetMessages(t, room.ID)
		userID := getUserIDByEmail(t, gothUser.Email)

		setMessageReactionWithUserID(t, msgID, userID)

		newURL := baseURL + room.ID.String() + "/messages/" + msgID + "/react"
		payload := strings.NewReader(`{"user_id": "` + userID + `", "message_id": "` + msgID + `"}`)
		rr := execAuthenticatedRequest(t, http.MethodDelete, newURL, payload)
		response := rr.Result()
		defer response.Body.Close()

		var result struct {
			Count int `json:"count"`
		}
		require.NoError(t, json.NewDecoder(response.Body).Decode(&result))

		_, p, err := ws.ReadMessage()
		require.NoError(t, err)

		var receivedMessage types.Message
		var messageReactionAdded types.MessageReactionAdded
		require.NoError(t, json.Unmarshal(p, &receivedMessage))

		jsonBytes, err := json.Marshal(receivedMessage.Value)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(jsonBytes, &messageReactionAdded))

		expectedMessage := 0
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, expectedMessage, result.Count)
		assert.Equal(t, msgID, messageReactionAdded.ID)
		assert.Equal(t, expectedMessage, int(messageReactionAdded.Count))
	})

	successTestCases := []struct {
		name            string
		method          string
		expectedMessage int
	}{
		{
			name:            "adds a reaction to the message",
			method:          http.MethodPatch,
			expectedMessage: 1,
		},
		{
			name:            "removes a reaction from the message",
			method:          http.MethodDelete,
			expectedMessage: 0,
		},
	}

	for _, tc := range successTestCases {
		t.Run(tc.name, func(t *testing.T) {
			truncateData(t)

			room := createAndGetRoom(t)
			msgID, _ := createAndGetMessages(t, room.ID)
			userID := getUserIDByEmail(t, gothUser.Email)

			if tc.method == http.MethodDelete {
				setMessageReactionWithUserID(t, msgID, userID)
			}

			newURL := baseURL + room.ID.String() + "/messages/" + msgID + "/react"

			payload := strings.NewReader(`{"user_id": "` + userID + `", "message_id": "` + msgID + `"}`)
			rr := execAuthenticatedRequest(t, tc.method, newURL, payload)
			response := rr.Result()
			defer response.Body.Close()

			var result struct {
				Count int `json:"count"`
			}
			require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
			assert.Equal(t, http.StatusOK, response.StatusCode)
			assert.Equal(t, tc.expectedMessage, result.Count)
		})
	}

	truncateData(t)
	room := createAndGetRoom(t)
	userID := getUserIDByEmail(t, gothUser.Email)
	msgID, _ := createAndGetMessages(t, room.ID)
	fakeID := uuid.New().String()

	failTestCases := []struct {
		name               string
		method             string
		url                string
		payload            string
		fn                 customFn
		expectedMessage    string
		expectedStatusCode int
		constraint         func(t testing.TB)
		setReaction        bool
	}{
		{
			name:               "patch - returns an error if request body is invalid",
			method:             http.MethodPatch,
			url:                baseURL + room.ID.String() + "/messages/" + msgID + "/react",
			payload:            `{ "invalid": "field" }`,
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "validation failed, missing required field(s): UserID\n",
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name:               "patch - returns an error if request body is not a valid JSON",
			method:             http.MethodPatch,
			url:                baseURL + room.ID.String() + "/messages/" + msgID + "/react",
			payload:            "aaaaaaaa",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "invalid body\n",
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name:               "patch - returns error if user ID is not a valid UUID",
			method:             http.MethodPatch,
			url:                baseURL + room.ID.String() + "/messages/" + msgID + "/react",
			payload:            `{"user_id": "invalid_uuid"}`,
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "validation failed: UserID must be a valid UUID\n",
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name:               "delete - returns an error if request body is invalid",
			method:             http.MethodDelete,
			url:                baseURL + room.ID.String() + "/messages/" + msgID + "/react",
			payload:            `{ "invalid": "field" }`,
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "validation failed, missing required field(s): UserID\n",
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name:               "delete - returns an error if request body is not a valid JSON",
			method:             http.MethodDelete,
			url:                baseURL + room.ID.String() + "/messages/" + msgID + "/react",
			payload:            "aaaaaaaa",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "invalid body\n",
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name:               "delete - returns error if user ID is not a valid UUID",
			method:             http.MethodDelete,
			url:                baseURL + room.ID.String() + "/messages/" + msgID + "/react",
			payload:            `{"user_id": "invalid_uuid"}`,
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "validation failed: UserID must be a valid UUID\n",
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name:               "returns an error if fails to update message",
			method:             http.MethodPatch,
			url:                baseURL + room.ID.String() + "/messages/" + msgID + "/react",
			payload:            `{"user_id": "` + userID + `", "message_id": "` + msgID + `"}`,
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "error reacting to message\n",
			expectedStatusCode: http.StatusInternalServerError,
			constraint: func(t testing.TB) {
				setMessagesReactionsConstraint(t)
			},
		},
		{
			name:               "returns an error if fails to remove reaction from message",
			method:             http.MethodDelete,
			url:                baseURL + room.ID.String() + "/messages/" + msgID + "/react",
			payload:            `{"user_id": "` + userID + `", "message_id": "` + msgID + `"}`,
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "error removing reaction from message\n",
			expectedStatusCode: http.StatusInternalServerError,
			constraint: func(t testing.TB) {
				setMessagesReactionsConstraint(t)
			},
		},
		{
			name:    "patch - returns unauthorized error if sessionID is not found",
			method:  http.MethodPatch,
			url:     baseURL + room.ID.String() + "/messages/" + msgID + "/react",
			payload: `{"user_id": "` + userID + `", "message_id": "` + msgID + `"}`,
			fn: func(t testing.TB, method, url string, body io.Reader) *httptest.ResponseRecorder {
				return execRequestWithoutCookie(method, url, body)
			},
			expectedMessage:    "unauthorized, session not found or invalid\n",
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:    "delete - returns unauthorized error if sessionID is not found",
			method:  http.MethodDelete,
			url:     baseURL + room.ID.String() + "/messages/" + msgID + "/react",
			payload: `{"user_id": "` + userID + `", "message_id": "` + msgID + `"}`,
			fn: func(t testing.TB, method, url string, body io.Reader) *httptest.ResponseRecorder {
				return execRequestWithoutCookie(method, url, body)
			},
			expectedMessage:    "unauthorized, session not found or invalid\n",
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:    "patch - returns unauthorized error if cookie is different from the session",
			method:  http.MethodPatch,
			url:     baseURL + room.ID.String() + "/messages/" + msgID + "/react",
			payload: `{"user_id": "` + userID + `", "message_id": "` + msgID + `"}`,
			fn: func(t testing.TB, method, url string, body io.Reader) *httptest.ResponseRecorder {
				return execRequestWithInvalidCookie(method, url, body)
			},
			expectedMessage:    "unauthorized, session not found or invalid\n",
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:    "delete - returns unauthorized error if cookie is different from the session",
			method:  http.MethodDelete,
			url:     baseURL + room.ID.String() + "/messages/" + msgID + "/react",
			payload: `{"user_id": "` + userID + `", "message_id": "` + msgID + `"}`,
			fn: func(t testing.TB, method, url string, body io.Reader) *httptest.ResponseRecorder {
				return execRequestWithInvalidCookie(method, url, body)
			},
			expectedMessage:    "unauthorized, session not found or invalid\n",
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "returns an error when trying to react to message that the user already reacted",
			method:             http.MethodPatch,
			url:                baseURL + room.ID.String() + "/messages/" + msgID + "/react",
			payload:            `{"user_id": "` + userID + `", "message_id": "` + msgID + `"}`,
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "user has already reacted to the message\n",
			expectedStatusCode: http.StatusInternalServerError,
			setReaction:        true,
		},
		{
			name:               "returns an error when trying to remove a reaction from a message the user did not react",
			method:             http.MethodDelete,
			url:                baseURL + room.ID.String() + "/messages/" + msgID + "/react",
			payload:            `{"user_id": "` + userID + `", "message_id": "` + msgID + `"}`,
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "message reaction not found\n",
			expectedStatusCode: http.StatusInternalServerError,
		},
		{
			name:               "patch - returns an error if room id is not valid",
			method:             http.MethodPatch,
			url:                baseURL + "invalid_room_id/messages/" + msgID + "/react",
			payload:            `{"user_id": "` + userID + `", "message_id": "` + msgID + `"}`,
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "invalid room id\n",
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name:               "delete - returns an error if room id is not valid",
			method:             http.MethodDelete,
			url:                baseURL + "invalid_room_id/messages/" + msgID + "/react",
			payload:            `{"user_id": "` + userID + `", "message_id": "` + msgID + `"}`,
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "invalid room id\n",
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name:               "patch - returns an error if room does not exist",
			method:             http.MethodPatch,
			url:                baseURL + fakeID + "/messages/" + msgID + "/react",
			payload:            `{"user_id": "` + userID + `", "message_id": "` + msgID + `"}`,
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "room not found\n",
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name:               "delete - returns an error if room does not exist",
			method:             http.MethodDelete,
			url:                baseURL + fakeID + "/messages/" + msgID + "/react",
			payload:            `{"user_id": "` + userID + `", "message_id": "` + msgID + `"}`,
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "room not found\n",
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name:               "patch - returns an error if message id is not valid",
			method:             http.MethodPatch,
			url:                baseURL + room.ID.String() + "/messages/invalid_message_id/react",
			payload:            `{"user_id": "` + userID + `", "message_id": "` + msgID + `"}`,
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "invalid message id\n",
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name:               "delete - returns an error if message id is not valid",
			method:             http.MethodDelete,
			url:                baseURL + room.ID.String() + "/messages/invalid_message_id/react",
			payload:            `{"user_id": "` + userID + `", "message_id": "` + msgID + `"}`,
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "invalid message id\n",
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name:               "patch - returns an error if message does not exist",
			method:             http.MethodPatch,
			url:                baseURL + room.ID.String() + "/messages/" + fakeID + "/react",
			payload:            `{"user_id": "` + userID + `", "message_id": "` + msgID + `"}`,
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "message not found\n",
			expectedStatusCode: http.StatusNotFound,
		},
		{
			name:               "delete - returns an error if message does not exist",
			method:             http.MethodDelete,
			url:                baseURL + room.ID.String() + "/messages/" + fakeID + "/react",
			payload:            `{"user_id": "` + userID + `", "message_id": "` + msgID + `"}`,
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "message not found\n",
			expectedStatusCode: http.StatusNotFound,
		},
		{
			name:               "patch - returns an error if fails to get message",
			method:             http.MethodPatch,
			url:                baseURL + room.ID.String() + "/messages/" + fakeID + "/react",
			payload:            `{"user_id": "` + userID + `", "message_id": "` + msgID + `"}`,
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "error validating message ID\n",
			expectedStatusCode: http.StatusInternalServerError,
			constraint: func(t testing.TB) {
				setMessagesConstraintFailure(t)
			},
		},
		{
			name:               "delete - returns an error if fails to get message",
			method:             http.MethodDelete,
			url:                baseURL + room.ID.String() + "/messages/" + fakeID + "/react",
			payload:            `{"user_id": "` + userID + `", "message_id": "` + msgID + `"}`,
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "error validating message ID\n",
			expectedStatusCode: http.StatusInternalServerError,
			constraint: func(t testing.TB) {
				setMessagesConstraintFailure(t)
			},
		},
	}

	for _, tc := range failTestCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setReaction {
				setMessageReactionWithUserID(t, msgID, userID)
			}

			if tc.constraint != nil {
				tc.constraint(t)
			}

			payload := strings.NewReader(tc.payload)
			rr := tc.fn(t, tc.method, tc.url, payload)
			response := rr.Result()
			defer response.Body.Close()

			body := parseResponseBody(t, response)

			assert.Equal(t, tc.expectedStatusCode, response.StatusCode)
			assert.Equal(t, tc.expectedMessage, body)
		})
	}
}
