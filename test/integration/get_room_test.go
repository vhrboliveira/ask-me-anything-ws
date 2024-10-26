package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vhrboliveira/ama-go/internal/store/pgstore"
)

func TestGetRoom(t *testing.T) {
	type customFn func(t testing.TB, method string, url string, body io.Reader) *httptest.ResponseRecorder

	const (
		baseURL = "/api/rooms"
		method  = http.MethodGet
	)

	t.Run("returns rooms list", func(t *testing.T) {
		roomNames := []string{"learning Go", "learning Rust"}

		truncateData(t)
		createRooms(t, roomNames)

		rr := execAuthenticatedRequest(t, method, baseURL, nil)
		response := rr.Result()
		defer response.Body.Close()

		var results []pgstore.Room
		require.NoError(t, json.NewDecoder(response.Body).Decode(&results))
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, len(roomNames), len(results))

		expectedNames := map[string]struct{}{
			roomNames[0]: {},
			roomNames[1]: {},
		}

		for _, result := range results {
			assert.NotEmpty(t, result.ID)
			assertValidUUID(t, result.UserID.String())
			assertValidDate(t, result.CreatedAt.Time.Format(time.RFC3339))

			_, ok := expectedNames[result.Name]
			assert.True(t, ok, "room not found")

			delete(expectedNames, result.Name)
		}

		assert.Empty(t, expectedNames, "not all expected rooms were found")
	})

	truncateData(t)
	type constraintFn func(t *testing.T)

	errorTestCases := []struct {
		name               string
		fn                 customFn
		expectedMessage    string
		expectedStatusCode int
		setConstraint      constraintFn
	}{
		{
			name: "returns unauthorized error if sessionID is not found",
			fn: func(t testing.TB, method, url string, body io.Reader) *httptest.ResponseRecorder {
				return execRequestWithoutCookie(method, url, body)
			},
			expectedMessage:    "unauthorized, session not found or invalid\n",
			expectedStatusCode: http.StatusUnauthorized,
			setConstraint:      nil,
		},
		{
			name: "returns unauthorized error if cookie is different from the session",
			fn: func(t testing.TB, method, url string, body io.Reader) *httptest.ResponseRecorder {
				return execRequestWithInvalidCookie(method, url, body)
			},
			expectedMessage:    "unauthorized, session not found or invalid\n",
			expectedStatusCode: http.StatusUnauthorized,
			setConstraint:      nil,
		},
		{
			name:               "returns an error if fails to get the rooms list",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "error getting rooms list\n",
			expectedStatusCode: http.StatusInternalServerError,
			setConstraint: func(t *testing.T) {
				setRoomsConstraintFailure(t)
			},
		},
		{
			name:               "returns empty room messages list if there is no room",
			fn:                 execAuthenticatedRequest,
			expectedMessage:    "[]",
			expectedStatusCode: http.StatusOK,
			setConstraint:      nil,
		},
	}

	for _, tc := range errorTestCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setConstraint != nil {
				tc.setConstraint(t)
			}

			rr := tc.fn(t, method, baseURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			body := parseResponseBody(t, response)

			assert.Equal(t, tc.expectedStatusCode, response.StatusCode)
			assert.Equal(t, tc.expectedMessage, body)
		})
	}

	t.Run("GET /api/rooms/{room_id}", func(t *testing.T) {
		t.Skip()
	})
}
