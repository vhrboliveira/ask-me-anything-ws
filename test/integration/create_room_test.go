package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vhrboliveira/ama-go/internal/store/pgstore"
	types "github.com/vhrboliveira/ama-go/internal/utils"
)

func TestCreateRoom(t *testing.T) {
	type customFn func(t testing.TB, method string, url string, body io.Reader, user *pgstore.User) *httptest.ResponseRecorder

	const (
		url    = "/api/rooms"
		method = http.MethodPost
	)

	createsRoomTestCases := []struct {
		name        string
		description string
	}{
		{name: "creates a room with all the data", description: "A new space to learn Go"},
		{name: "creates a room without description", description: ""},
	}

	for _, tc := range createsRoomTestCases {
		t.Run(tc.name, func(t *testing.T) {
			truncateData(t)

			roomName := "Learning Go"
			userID := generateUser(t)
			parsedUserID, _ := uuid.Parse(userID)
			user := pgstore.User{ID: parsedUserID}
			payload := strings.NewReader(`{"name": "` + roomName + `", "user_id": "` + userID + `", "description": "` + tc.description + `"}`)
			rr := execRequestGeneratingSession(t, method, url, payload, &user)

			response := rr.Result()
			defer response.Body.Close()

			var result pgstore.Room
			require.NoError(t, json.NewDecoder(response.Body).Decode(&result))

			assert.Equal(t, response.StatusCode, http.StatusCreated)
			assertValidUUID(t, result.ID.String())
			assertValidDate(t, result.CreatedAt.Time.Format(time.RFC3339))
			assert.True(t, result.CreatedAt.Valid)
			assert.Equal(t, result.UserID.String(), userID)
			assert.Equal(t, result.Description, tc.description)
		})
	}

	t.Run("sends a message to the websocket subscribers when a room is created", func(t *testing.T) {
		truncateData(t)

		server := httptest.NewServer(Router)
		defer server.Close()

		userID := generateUser(t)
		parsedUserID, _ := uuid.Parse(userID)
		user := pgstore.User{ID: parsedUserID}

		wsURL := "ws" + server.URL[4:] + "/subscribe"
		ws, _, err := connectWSWithUserSession(t, wsURL, &userID)
		require.NoError(t, err)
		defer ws.Close()

		roomName := "Learning Go"
		description := "A new space to learn Go"
		payload := strings.NewReader(`{"name": "` + roomName + `", "user_id": "` + userID + `", "description": "` + description + `"}`)
		rr := execRequestGeneratingSession(t, method, url, payload, &user)

		response := rr.Result()
		defer response.Body.Close()

		var result struct {
			ID          string `json:"id"`
			CreatedAt   string `json:"created_at"`
			UserID      string `json:"user_id"`
			Description string `json:"description"`
		}

		require.NoError(t, json.NewDecoder(response.Body).Decode(&result))

		// Read the message from WebSocket
		_, p, err := ws.ReadMessage()
		require.NoError(t, err)

		var receivedMessage types.Message
		var roomCreated types.RoomCreated

		require.NoError(t, json.Unmarshal(p, &receivedMessage), "failed to unmarshal received message")
		jsonBytes, err := json.Marshal(receivedMessage.Value)
		require.NoError(t, err, "failed to marshal received message value")
		require.NoError(t, json.Unmarshal(jsonBytes, &roomCreated), "failed to unmarshal RoomCreated value")

		assert.Equal(t, response.StatusCode, http.StatusCreated)
		assert.Equal(t, receivedMessage.Kind, types.MessageKindRoomCreated)
		assert.Equal(t, roomCreated.ID, result.ID)
		assert.Equal(t, roomCreated.Name, roomName)
		assertValidDate(t, roomCreated.CreatedAt)
		assertValidDate(t, result.CreatedAt)
		assert.Equal(t, result.UserID, userID)
		assert.Equal(t, roomCreated.UserID, userID)
		assert.Equal(t, roomCreated.Description, description)
	})

	truncateData(t)
	roomName := "Learning Go"
	roomName2 := "Learning Rust"
	userID := generateUser(t)
	parsedUserID, _ := uuid.Parse(userID)
	user := pgstore.User{ID: parsedUserID}
	invalidUserID := uuid.New().String()

	errorTestCases := []struct {
		name               string
		fn                 customFn
		expectedMessage    string
		expectedStatusCode int
		payload            string
		setConstraint      bool
	}{
		{
			name:               "returns forbidden error if user ID does not match user ID from the session",
			fn:                 execRequestGeneratingSession,
			expectedMessage:    "invalid user id\n",
			expectedStatusCode: http.StatusForbidden,
			payload:            `{"name": "` + roomName + `", "user_id": "` + invalidUserID + `"}`,
			setConstraint:      false,
		},
		{
			name: "returns unauthorized error if sessionID is not found",
			fn: func(t testing.TB, method string, url string, body io.Reader, user *pgstore.User) *httptest.ResponseRecorder {
				return execRequestWithoutCookie(method, url, body)
			},
			expectedMessage:    "unauthorized, session not found or invalid\n",
			expectedStatusCode: http.StatusUnauthorized,
			payload:            "",
			setConstraint:      false,
		},
		{
			name: "returns unauthorized error if cookie is different from the session",
			fn: func(t testing.TB, method string, url string, body io.Reader, user *pgstore.User) *httptest.ResponseRecorder {
				return execRequestWithInvalidCookie(method, url, body)
			},
			expectedMessage:    "unauthorized, session not found or invalid\n",
			expectedStatusCode: http.StatusUnauthorized,
			payload:            "",
			setConstraint:      false,
		},
		{
			name:               "returns an error if request body is invalid",
			fn:                 execRequestGeneratingSession,
			expectedMessage:    "validation failed, missing required field(s): Name, UserID\n",
			expectedStatusCode: http.StatusBadRequest,
			payload:            `{ "invalid": "field" }`,
			setConstraint:      false,
		},
		{
			name:               "returns an error if request body is not a valid JSON",
			fn:                 execRequestGeneratingSession,
			expectedMessage:    "invalid body\n",
			expectedStatusCode: http.StatusBadRequest,
			payload:            "aaaaaaa",
			setConstraint:      false,
		},
		{
			name:               "returns an error if user ID is not provided",
			fn:                 execRequestGeneratingSession,
			expectedMessage:    "validation failed, missing required field(s): UserID\n",
			expectedStatusCode: http.StatusBadRequest,
			payload:            `{"name": "Learning Go"}`,
			setConstraint:      false,
		},
		{
			name:               "returns error if user ID is not a valid UUID",
			fn:                 execRequestGeneratingSession,
			expectedMessage:    "validation failed: UserID must be a valid UUID\n",
			expectedStatusCode: http.StatusBadRequest,
			payload:            `{"name": "Learning Go", "user_id": "invalid-uuid"}`,
			setConstraint:      false,
		},
		{
			name:               "returns error if payload provides multiples rooms",
			fn:                 execRequestGeneratingSession,
			expectedMessage:    "invalid body\n",
			expectedStatusCode: http.StatusBadRequest,
			payload:            `[{"name": "` + roomName + `", "user_id": "` + userID + `"}, {"name": "` + roomName2 + `", "user_id": "` + userID + `"}]`,
			setConstraint:      false,
		},
		{
			name:               "returns error if payload provides invalid description",
			fn:                 execRequestGeneratingSession,
			expectedMessage:    "invalid body\n",
			expectedStatusCode: http.StatusBadRequest,
			payload:            `{"name": "` + roomName + `", "user_id": "` + userID + `", "description": ` + userID + ` }`,
			setConstraint:      false,
		},
		{
			name:               "returns an error if fails to insert room",
			fn:                 execRequestGeneratingSession,
			expectedMessage:    "error creating room\n",
			expectedStatusCode: http.StatusInternalServerError,
			payload:            `{"name": "` + roomName + `", "user_id": "` + userID + `"}`,
			setConstraint:      true,
		},
	}

	for _, tc := range errorTestCases {
		t.Run(tc.name, func(t *testing.T) {
			payload := strings.NewReader(tc.payload)

			if tc.setConstraint {
				setRoomsConstraintFailure(t)
			}

			rr := tc.fn(t, method, url, payload, &user)
			response := rr.Result()
			defer response.Body.Close()

			body := parseResponseBody(t, response)

			assert.Equal(t, tc.expectedStatusCode, response.StatusCode)
			assert.Equal(t, tc.expectedMessage, body)
		})
	}
}
