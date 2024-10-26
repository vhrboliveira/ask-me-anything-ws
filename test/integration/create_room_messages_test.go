package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vhrboliveira/ama-go/internal/types"
)

func TestCreateRoomMessages(t *testing.T) {
	type customFn func(t testing.TB, method string, url string, body io.Reader) *httptest.ResponseRecorder

	const (
		baseURL = "/api/rooms/room_id/messages"
		method  = http.MethodPost
	)

	t.Run("create messages for a room", func(t *testing.T) {
		truncateData(t)

		room := createAndGetRoom(t)
		newURL := strings.Replace(baseURL, "room_id", strconv.Itoa(int(room.ID)), 1)
		payload := strings.NewReader(`{"message": "Is Go awesome?"}`)
		rr := execAuthenticatedRequest(t, method, newURL, payload)

		response := rr.Result()
		defer response.Body.Close()

		var result struct {
			ID        string `json:"id"`
			CreatedAt string `json:"created_at"`
		}

		err := json.NewDecoder(response.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, response.StatusCode, http.StatusCreated)
		assertValidUUID(t, result.ID)
		assertValidDate(t, result.CreatedAt)
	})

	t.Run("sends a message to the websocket subscribers when a message is created in a room", func(t *testing.T) {
		truncateData(t)

		room := createAndGetRoom(t)

		server := httptest.NewServer(Router)
		defer server.Close()

		wsURL := "ws" + server.URL[4:] + "/subscribe/room/" + strconv.Itoa(int(room.ID))
		ws, err := connectAuthenticatedWS(t, wsURL)
		require.NoError(t, err)
		defer ws.Close()

		want := "Is Go awesome?"
		newURL := strings.Replace(baseURL, "room_id", strconv.Itoa(int(room.ID)), 1)
		payload := strings.NewReader(`{"message": "` + want + `"}`)
		rr := execAuthenticatedRequest(t, method, newURL, payload)

		response := rr.Result()
		defer response.Body.Close()

		var result struct {
			ID        string `json:"id"`
			CreatedAt string `json:"created_at"`
		}

		require.NoError(t, json.NewDecoder(response.Body).Decode(&result))

		assert.Equal(t, response.StatusCode, http.StatusCreated)
		assertValidUUID(t, result.ID)
		assertValidDate(t, result.CreatedAt)

		_, p, err := ws.ReadMessage()
		require.NoError(t, err)

		var receivedMessage types.Message
		var messageCreated types.MessageCreated

		require.NoError(t, json.Unmarshal(p, &receivedMessage), "failed to unmarshal received message")
		jsonBytes, err := json.Marshal(receivedMessage.Value)
		require.NoError(t, err, "failed to marshal received message value")
		require.NoError(t, json.Unmarshal(jsonBytes, &messageCreated), "failed to unmarshal RoomCreated value")

		assert.Equal(t, receivedMessage.Kind, types.MessageKindMessageCreated)
		assert.Equal(t, messageCreated.ID, result.ID)
		assert.Equal(t, messageCreated.Message, want)
		assertValidDate(t, messageCreated.CreatedAt)
	})

	truncateData(t)
	type constraintFn func(t *testing.T)
	fakeID := uuid.New().String()
	room := createAndGetRoom(t)
	fakeRoomID := strconv.Itoa(int(room.ID + 10))

	errorTestCases := []struct {
		name               string
		fn                 customFn
		expectedMessage    string
		expectedStatusCode int
		url                string
		payload            string
		setConstraint      constraintFn
	}{
		{
			name: "returns unauthorized error if sessionID is not found",
			fn: func(t testing.TB, method, url string, body io.Reader) *httptest.ResponseRecorder {
				return execRequestWithoutCookie(method, url, body)
			},
			expectedMessage:    "unauthorized, session not found or invalid\n",
			expectedStatusCode: http.StatusUnauthorized,
			url:                strings.Replace(baseURL, "room_id", fakeID, 1),
			payload:            "",
			setConstraint:      nil,
		},
		{
			name: "returns unauthorized error if cookie is different from the session",
			fn: func(t testing.TB, method, url string, body io.Reader) *httptest.ResponseRecorder {
				return execRequestWithInvalidCookie(method, url, body)
			},
			expectedMessage:    "unauthorized, session not found or invalid\n",
			expectedStatusCode: http.StatusUnauthorized,
			url:                strings.Replace(baseURL, "room_id", fakeID, 1),
			payload:            "",
			setConstraint:      nil,
		},
		{
			name:               "returns an error if room id is not valid",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "invalid room id\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                baseURL,
			payload:            "",
			setConstraint:      nil,
		},
		{
			name:               "returns an error if room does not exist",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "room not found\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                strings.Replace(baseURL, "room_id", fakeRoomID, 1),
			payload:            `{"message": "Is Go awesome?"}`,
			setConstraint:      nil,
		},
		{
			name:               "returns an error if body is invalid",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "validation failed: missing required field(s): message\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                strings.Replace(baseURL, "room_id", strconv.Itoa(int(room.ID)), 1),
			payload:            `{"invalid": "invalid"}`,
			setConstraint:      nil,
		},
		{
			name:               "returns an error if request body is not a valid json",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "invalid body\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                strings.Replace(baseURL, "room_id", strconv.Itoa(int(room.ID)), 1),
			payload:            "aaaaaa",
			setConstraint:      nil,
		},
		{
			name:               "returns an error if payload provides multiples messages",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "invalid body\n",
			expectedStatusCode: http.StatusBadRequest,
			url:                strings.Replace(baseURL, "room_id", strconv.Itoa(int(room.ID)), 1),
			payload:            `[{"message": "a valid message"}, {"message": "another valid message"}]`,
			setConstraint:      nil,
		},
		{
			name:               "returns an error if fails to get room",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "error validating room ID\n",
			expectedStatusCode: http.StatusInternalServerError,
			url:                strings.Replace(baseURL, "room_id", fakeRoomID, 1),
			payload:            `{"message": "a valid message"}`,
			setConstraint: func(t *testing.T) {
				setRoomsConstraintFailure(t)
			},
		},
		{
			name:               "returns an error if fails to insert message",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "error inserting message\n",
			expectedStatusCode: http.StatusInternalServerError,
			url:                strings.Replace(baseURL, "room_id", strconv.Itoa(int(room.ID)), 1),
			payload:            `{"message": "a valid message"}`,
			setConstraint: func(t *testing.T) {
				setMessagesConstraintFailure(t)
			},
		},
	}

	for _, tc := range errorTestCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setConstraint != nil {
				tc.setConstraint(t)
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
