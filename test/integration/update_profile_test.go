package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vhrboliveira/ama-go/internal/store/pgstore"
)

func TestUpdateProfile(t *testing.T) {
	type customFn func(t testing.TB, method string, url string, body io.Reader, user *pgstore.User) *httptest.ResponseRecorder

	const (
		baseURL = "/api/profile"
		method  = http.MethodPatch
	)

	t.Run("update profile with its session", func(t *testing.T) {
		truncateData(t)

		var err error
		user := pgstore.User{
			Provider:       "google",
			ProviderUserID: "1234567890",
			Photo:          "http://avatar.com/test.jpg",
			Email:          "vitor@vhrbo.tech",
			Name:           "",
		}
		userID := createUser(t, user.Email, user.Name, user.Provider, user.ProviderUserID, user.Photo)
		user.ID, err = uuid.Parse(userID)
		require.NoError(t, err)

		expectedName := "vitor o"
		enablePicture := "true"
		expectedNewUser := false
		expectedEnablePicture, _ := strconv.ParseBool(enablePicture)

		payload := strings.NewReader(`{"user_id": "` + userID + `", "name": "` + expectedName + `", "enable_picture": ` + enablePicture + `}`)
		rr := execRequestGeneratingSession(t, method, baseURL, payload, &user)

		response := rr.Result()
		defer response.Body.Close()

		var result struct {
			ID            uuid.UUID `json:"id"`
			Name          string    `json:"name"`
			EnablePicture bool      `json:"enable_picture"`
			NewUser       bool      `json:"new_user"`
			UpdatedAt     string    `json:"updated_at"`
		}
		require.NoError(t, json.NewDecoder(response.Body).Decode(&result))

		valkeyUser := getValkeyData(t, userID)

		assert.Equal(t, result.ID, user.ID)
		assert.Equal(t, result.ID, valkeyUser.ID)
		assert.Equal(t, result.EnablePicture, expectedEnablePicture)
		assert.Equal(t, result.EnablePicture, valkeyUser.EnablePicture)
		assert.Equal(t, result.Name, expectedName)
		assert.Equal(t, result.Name, valkeyUser.Name)
		assert.Equal(t, result.NewUser, expectedNewUser)
		assert.Equal(t, result.NewUser, valkeyUser.NewUser)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, result.UpdatedAt, valkeyUser.UpdatedAt.Time.Format(time.RFC3339))
		assertValidDate(t, result.UpdatedAt)

	})

	truncateData(t)

	invalidUserID := uuid.New().String()

	var err error
	user := pgstore.User{
		Provider:       "google",
		ProviderUserID: "1234567890",
		Photo:          "http://avatar.com/test.jpg",
		Email:          "vitor@vhrbo.tech",
		Name:           "vitor",
	}
	enablePicture := "true"
	userID := createUser(t, user.Email, user.Name, user.Provider, user.ProviderUserID, user.Photo)
	user.ID, err = uuid.Parse(userID)
	require.NoError(t, err)

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
			payload:            `{"user_id": "` + invalidUserID + `", "name": "` + user.Name + `", "enable_picture": ` + enablePicture + `}`,
			setConstraint:      false,
		},
		{
			name: "returns unauthorized error if sessionID is not found",
			fn: func(t testing.TB, method string, url string, body io.Reader, user *pgstore.User) *httptest.ResponseRecorder {
				return execRequestWithoutCookie(method, url, body)
			},
			expectedMessage:    "unauthorized, session not found or invalid\n",
			expectedStatusCode: http.StatusUnauthorized,
			payload:            `{"user_id": "` + invalidUserID + `", "name": "` + user.Name + `", "enable_picture": ` + enablePicture + `}`,
			setConstraint:      false,
		},
		{
			name: "returns unauthorized error if cookie is different from the session",
			fn: func(t testing.TB, method string, url string, body io.Reader, user *pgstore.User) *httptest.ResponseRecorder {
				return execRequestWithInvalidCookie(method, url, body)
			},
			expectedMessage:    "unauthorized, session not found or invalid\n",
			expectedStatusCode: http.StatusUnauthorized,
			payload:            `{"user_id": "` + invalidUserID + `", "name": "` + user.Name + `", "enable_picture": ` + enablePicture + `}`,
			setConstraint:      false,
		},
		{
			name:               "returns an error if request body is invalid",
			fn:                 execRequestGeneratingSession,
			expectedMessage:    "validation failed, missing required field(s): EnablePicture, UserID, Name\n",
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
			payload:            `{"name": "` + user.Name + `", "enable_picture": ` + enablePicture + `}`,
			setConstraint:      false,
		},
		{
			name:               "returns an error if name is not provided",
			fn:                 execRequestGeneratingSession,
			expectedMessage:    "validation failed, missing required field(s): Name\n",
			expectedStatusCode: http.StatusBadRequest,
			payload:            `{"user_id": "` + invalidUserID + `", "enable_picture": ` + enablePicture + `}`,
			setConstraint:      false,
		},
		{
			name:               "returns an error if enable picture is not provided",
			fn:                 execRequestGeneratingSession,
			expectedMessage:    "validation failed, missing required field(s): EnablePicture\n",
			expectedStatusCode: http.StatusBadRequest,
			payload:            `{"user_id": "` + invalidUserID + `", "name": "` + user.Name + `"}`,
			setConstraint:      false,
		},
		{
			name:               "returns an error if enable picture is not a boolean",
			fn:                 execRequestGeneratingSession,
			expectedMessage:    "invalid body\n",
			expectedStatusCode: http.StatusBadRequest,
			payload:            `{"user_id": "` + invalidUserID + `", "name": "` + user.Name + `", "enable_picture": "any value here"}`,
			setConstraint:      false,
		},
		{
			name:               "returns an error if userID is not a valid UUID",
			fn:                 execRequestGeneratingSession,
			expectedMessage:    "validation failed: UserID must be a valid UUID\n",
			expectedStatusCode: http.StatusBadRequest,
			payload:            `{"user_id": "invalid-uuid", "name": "` + user.Name + `", "enable_picture": ` + enablePicture + `}`,
			setConstraint:      false,
		},
		{
			name:               "returns an error if fails to update profile",
			fn:                 execRequestGeneratingSession,
			expectedMessage:    "error updating user\n",
			expectedStatusCode: http.StatusInternalServerError,
			payload:            `{"user_id": "` + userID + `", "name": "` + user.Name + `", "enable_picture": ` + enablePicture + `}`,
			setConstraint:      true,
		},
	}

	for _, tc := range errorTestCases {
		t.Run(tc.name, func(t *testing.T) {
			payload := strings.NewReader(tc.payload)

			if tc.setConstraint {
				setCreateUserConstraintError(t)
			}

			rr := tc.fn(t, method, baseURL, payload, &user)
			response := rr.Result()
			defer response.Body.Close()

			body := parseResponseBody(t, response)

			assert.Equal(t, tc.expectedStatusCode, response.StatusCode)
			assert.Equal(t, tc.expectedMessage, body)
		})
	}

}
