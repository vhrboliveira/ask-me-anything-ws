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

func TestGetUser(t *testing.T) {
	type customFn func(method string, url string, body io.Reader) *httptest.ResponseRecorder

	const (
		baseURL = "/api/user"
		method  = http.MethodGet
	)

	t.Run("returns the user information", func(t *testing.T) {
		truncateData(t)

		defaultBool := true
		expectedUser := pgstore.User{
			Name:           "vitor o",
			Email:          "vitor@vhrbo.tech",
			Provider:       "google",
			ProviderUserID: "1234567890",
			NewUser:        defaultBool,
			EnablePicture:  defaultBool,
			Photo:          "http://avatar.com/test.jpg",
		}

		userID := createUser(t,
			expectedUser.Email,
			expectedUser.Name,
			expectedUser.Provider,
			expectedUser.ProviderUserID,
			expectedUser.Photo,
		)

		expectedUser.ID, _ = uuid.Parse(userID)

		rr := execRequestGeneratingSession(t, method, baseURL, nil, &expectedUser)
		response := rr.Result()
		defer response.Body.Close()

		var user pgstore.User
		require.NoError(t, json.NewDecoder(response.Body).Decode(&user))

		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, user.ID, expectedUser.ID)
		assert.Equal(t, user.Email, expectedUser.Email)
		assert.Equal(t, user.Name, expectedUser.Name)
		assert.Equal(t, user.Provider, expectedUser.Provider)
		assert.Equal(t, user.ProviderUserID, expectedUser.ProviderUserID)
		assert.Equal(t, user.Photo, expectedUser.Photo)
		assert.Equal(t, user.NewUser, expectedUser.NewUser)
		assert.Equal(t, user.EnablePicture, expectedUser.EnablePicture)
		assert.True(t, user.CreatedAt.Valid)
		assert.True(t, user.UpdatedAt.Valid)
		assertValidDate(t, user.CreatedAt.Time.Format(time.RFC3339))
		assertValidDate(t, user.UpdatedAt.Time.Format(time.RFC3339))
	})

	truncateData(t)

	errorTestCases := []struct {
		name               string
		fn                 customFn
		expectedMessage    string
		expectedStatusCode int
	}{
		{
			name:               "returns unauthorized error if sessionID is not found",
			fn:                 execRequestWithoutCookie,
			expectedMessage:    "unauthorized, session not found or invalid\n",
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "returns unauthorized error if cookie is different from the session",
			fn:                 execRequestWithInvalidCookie,
			expectedMessage:    "unauthorized, session not found or invalid\n",
			expectedStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tc := range errorTestCases {
		t.Run(tc.name, func(t *testing.T) {
			rr := tc.fn(method, baseURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			body := parseResponseBody(t, response)

			assert.Equal(t, tc.expectedStatusCode, response.StatusCode)
			assert.Equal(t, tc.expectedMessage, body)
		})
	}

}
