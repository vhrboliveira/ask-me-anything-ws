package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vhrboliveira/ama-go/internal/store/pgstore"
)

func TestGetRoomMessages(t *testing.T) {
	type customFn func(t testing.TB, method string, url string, body io.Reader) *httptest.ResponseRecorder

	const (
		baseURL = "/api/rooms/"
		method  = http.MethodGet
	)

	t.Run("returns room messages list", func(t *testing.T) {
		truncateData(t)

		room := createAndGetRoom(t)
		msgs := []pgstore.InsertMessageParams{
			{RoomID: room.ID, Message: "message 1"},
			{RoomID: room.ID, Message: "message 2"},
		}
		answer := "It's done!"
		insertMessages(t, msgs)
		answerMessages(t, answer)

		newURL := baseURL + room.ID.String() + "/messages"
		rr := execAuthenticatedRequest(t, method, newURL, nil)
		response := rr.Result()
		defer response.Body.Close()

		var results []pgstore.Message
		require.NoError(t, json.NewDecoder(response.Body).Decode(&results))
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, len(msgs), len(results))

		expectedMsgs := map[string]struct{}{
			msgs[0].Message: {},
			msgs[1].Message: {},
		}

		for _, result := range results {
			assertValidUUID(t, result.ID.String())
			assertValidDate(t, result.CreatedAt.Time.Format(time.RFC3339))
			assert.True(t, result.Answered, "expected the answer to be true")
			assert.Equal(t, answer, result.Answer)

			_, ok := expectedMsgs[result.Message]
			assert.True(t, ok, "message not found")

			delete(expectedMsgs, result.Message)
		}

		assert.Empty(t, expectedMsgs, "not all expected messages were found")
	})

	t.Run("returns message for a given message ID", func(t *testing.T) {
		truncateData(t)

		room := createAndGetRoom(t)
		messageID, messageTxt := createAndGetMessages(t, room.ID)
		answer := "This is answered!"
		answerMessageByID(t, messageID, answer)

		newURL := baseURL + room.ID.String() + "/messages/" + messageID
		rr := execAuthenticatedRequest(t, method, newURL, nil)
		response := rr.Result()
		defer response.Body.Close()

		var result pgstore.GetRoomMessagesRow
		require.NoError(t, json.NewDecoder(response.Body).Decode(&result))

		assertValidUUID(t, result.ID.String())
		assert.Equal(t, messageTxt, result.Message)
		assert.Equal(t, room.ID.String(), result.RoomID.String())
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.True(t, result.CreatedAt.Valid, "expected created at to be not empty")
		assert.True(t, result.Answered, "expected answered to be true")
		assert.Equal(t, int(result.ReactionCount), 0, "expected reaction count to be 0")
		assert.Equal(t, answer, result.Answer)
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
			name: "returns unauthorized error if sessionID is not found getting room messages list",
			fn: func(t testing.TB, method, url string, body io.Reader) *httptest.ResponseRecorder {
				return execRequestWithoutCookie(method, url, body)
			},
			expectedMessage:    "unauthorized, session not found or invalid\n",
			expectedStatusCode: http.StatusUnauthorized,
			url:                baseURL + fakeID + "/messages",
			setConstraint:      nil,
		},
		{
			name: "returns unauthorized error if sessionID is not found getting a message",
			fn: func(t testing.TB, method, url string, body io.Reader) *httptest.ResponseRecorder {
				return execRequestWithoutCookie(method, url, body)
			},
			expectedMessage:    "unauthorized, session not found or invalid\n",
			expectedStatusCode: http.StatusUnauthorized,
			url:                baseURL + fakeID + "/messages/" + fakeID,
			setConstraint:      nil,
		},
		{
			name: "returns unauthorized error if cookie is different from the session getting room messages list",
			fn: func(t testing.TB, method, url string, body io.Reader) *httptest.ResponseRecorder {
				return execRequestWithInvalidCookie(method, url, body)
			},
			expectedMessage:    "unauthorized, session not found or invalid\n",
			expectedStatusCode: http.StatusUnauthorized,
			url:                baseURL + fakeID + "/messages",
			setConstraint:      nil,
		},
		{
			name: "returns unauthorized error if cookie is different from the session getting a message",
			fn: func(t testing.TB, method, url string, body io.Reader) *httptest.ResponseRecorder {
				return execRequestWithInvalidCookie(method, url, body)
			},
			expectedMessage:    "unauthorized, session not found or invalid\n",
			expectedStatusCode: http.StatusUnauthorized,
			url:                baseURL + fakeID + "/messages/" + fakeID,
			setConstraint:      nil,
		},
		{
			name:               "returns an error if room id is not valid when getting room messages list",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "invalid room id\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL + "invalid_id/messages",
			setConstraint:      nil,
		},
		{
			name:               "returns an error if room id is not valid when getting a message",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "invalid room id\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL + "invalid_id/messages/" + fakeID,
			setConstraint:      nil,
		},
		{
			name:               "returns an error if room does not exist when getting room messages list",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "room not found\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL + fakeID + "/messages",
			setConstraint:      nil,
		},
		{
			name:               "returns an error if room does not exist when getting a message",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "room not found\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL + fakeID + "/messages/" + fakeID,
			setConstraint:      nil,
		},
		{
			name:               "returns an error if message id is not valid when getting a message",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "invalid message id\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL + room.ID.String() + "/messages/invalid_message_id",
			setConstraint:      nil,
		},
		{
			name:               "returns an error if message does not exist when getting a message",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "message not found\n",
			expectedStatusCode: http.StatusNotFound,
			url:                baseURL + room.ID.String() + "/messages/" + fakeID,
			setConstraint:      nil,
		},
		{
			name:               "returns empty room messages list if room has no messages",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "[]",
			expectedStatusCode: http.StatusOK,
			url:                baseURL + room.ID.String() + "/messages",
			setConstraint:      nil,
		},
		{
			name:               "returns an error if fails to get a message",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "error getting message\n",
			expectedStatusCode: http.StatusInternalServerError,
			url:                baseURL + room.ID.String() + "/messages/" + fakeID,
			setConstraint: func(t *testing.T) {
				setMessagesConstraintFailure(t)
			},
		},
		{
			name:               "returns an error if fails to get room when getting room messages list",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "error validating room ID\n",
			expectedStatusCode: http.StatusInternalServerError,
			url:                baseURL + fakeID + "/messages",
			setConstraint: func(t *testing.T) {
				setRoomsConstraintFailure(t)
			},
		},
		{
			name:               "returns an error if fails to get room when getting a message",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "error validating room ID\n",
			expectedStatusCode: http.StatusInternalServerError,
			url:                baseURL + fakeID + "/messages/" + fakeID,
			setConstraint: func(t *testing.T) {
				setRoomsConstraintFailure(t)
			},
		},
		{
			name:               "returns an error if fails to get room messages",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "error getting room messages\n",
			expectedStatusCode: http.StatusInternalServerError,
			url:                baseURL + room.ID.String() + "/messages",
			setConstraint: func(t *testing.T) {
				setMessagesConstraintFailure(t)
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

			assert.Equal(t, tc.expectedStatusCode, response.StatusCode)
			assert.Equal(t, tc.expectedMessage, body)
		})
	}
}
