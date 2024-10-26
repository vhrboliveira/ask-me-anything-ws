package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRoomMessagesReactions(t *testing.T) {
	type customFn func(t testing.TB, method string, url string, body io.Reader) *httptest.ResponseRecorder
	const (
		baseURL = "/api/rooms/"
		method  = http.MethodGet
	)

	gothUser := mockGothUser(nil)

	t.Run("returns the messages IDS the User ID has reacted in a room", func(t *testing.T) {
		truncateData(t)

		room := createAndGetRoom(t)
		msgID, _ := createAndGetMessages(t, room.ID)
		msgID2, _ := createAndGetMessages(t, room.ID)
		userID := getUserIDByEmail(t, gothUser.Email)
		setMessageReactionWithUserID(t, msgID, userID)
		setMessageReactionWithUserID(t, msgID2, userID)

		newURL := baseURL + strconv.Itoa(int(room.ID)) + "/reactions?user_id=" + userID
		rr := execAuthenticatedRequest(t, method, newURL, nil)
		response := rr.Result()
		defer response.Body.Close()

		type responseType struct {
			IDS []string `json:"ids"`
		}
		var result responseType
		expectedResult := responseType{
			IDS: []string{msgID, msgID2},
		}
		require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, expectedResult.IDS, result.IDS)
	})

	t.Run("returns no message ID if the user didn't react to any message in a room", func(t *testing.T) {
		truncateData(t)

		room := createAndGetRoom(t)
		userID := getUserIDByEmail(t, gothUser.Email)

		newURL := baseURL + strconv.Itoa(int(room.ID)) + "/reactions?user_id=" + userID
		rr := execAuthenticatedRequest(t, method, newURL, nil)
		response := rr.Result()
		defer response.Body.Close()

		type responseType struct {
			IDS []string `json:"ids"`
		}
		var result responseType
		expectedResult := responseType{
			IDS: []string{},
		}
		require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, expectedResult.IDS, result.IDS)
	})

	truncateData(t)
	fakeID := uuid.New().String()
	room := createAndGetRoom(t)
	fakeRoomID := strconv.Itoa(int(room.ID + 10))
	userID := getUserIDByEmail(t, gothUser.Email)

	type constraintFn func(t *testing.T)

	errorTestCases := []struct {
		name               string
		fn                 customFn
		expectedMessage    string
		expectedStatusCode int
		url                string
		setConstraint      constraintFn
	}{
		{
			name:               "returns an error if request user ID is missing in the query params",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "validation failed, missing required field(s): UserID\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL + strconv.Itoa(int(room.ID)) + "/reactions",
		},
		{
			name:               "returns error if user ID is not a valid UUID",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "invalid user ID\n",
			expectedStatusCode: http.StatusForbidden,
			url:                baseURL + strconv.Itoa(int(room.ID)) + "/reactions?user_id=invalid-user-id",
		},
		{
			name:               "returns error if user ID is not the same from the session cookie",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "invalid user ID\n",
			expectedStatusCode: http.StatusForbidden,
			url:                baseURL + strconv.Itoa(int(room.ID)) + "/reactions?user_id=" + fakeID,
		},
		{
			name: "returns unauthorized error if sessionID is not found",
			fn: func(t testing.TB, method, url string, body io.Reader) *httptest.ResponseRecorder {
				return execRequestWithoutCookie(method, url, body)
			},
			expectedMessage:    "unauthorized, session not found or invalid\n",
			expectedStatusCode: http.StatusUnauthorized,
			url:                baseURL + strconv.Itoa(int(room.ID)) + "/reactions?user_id=" + userID,
			setConstraint:      nil,
		},
		{
			name: "returns unauthorized error if cookie is different from the session",
			fn: func(t testing.TB, method, url string, body io.Reader) *httptest.ResponseRecorder {
				return execRequestWithInvalidCookie(method, url, body)
			},
			expectedMessage:    "unauthorized, session not found or invalid\n",
			expectedStatusCode: http.StatusUnauthorized,
			url:                baseURL + strconv.Itoa(int(room.ID)) + "/reactions?user_id=" + userID,
			setConstraint:      nil,
		},
		{
			name:               "returns an error if room id is not valid",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "invalid room id\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL + "invalid-room-id/reactions",
			setConstraint:      nil,
		},
		{
			name:               "returns an error if room does not exist",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "room not found\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL + fakeRoomID + "/reactions?user_id=" + userID,
			setConstraint:      nil,
		},
		{
			name:               "returns an error if fails to get the messages reactions IDS",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "error getting messages reactions\n",
			expectedStatusCode: http.StatusInternalServerError,
			url:                baseURL + strconv.Itoa(int(room.ID)) + "/reactions?user_id=" + userID,
			setConstraint: func(t *testing.T) {
				setMessagesReactionsConstraint(t)
			},
		},
	}

	for _, tc := range errorTestCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setConstraint != nil {
				tc.setConstraint(t)
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
