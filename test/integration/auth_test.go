package api_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/facebook"
	"github.com/markbates/goth/providers/google"
	"github.com/stretchr/testify/assert"
	"github.com/vhrboliveira/ama-go/internal/auth"
	"github.com/vhrboliveira/ama-go/internal/store/pgstore"
)

func mockGothUser(u *pgstore.User) goth.User {
	userID := uuid.New().String()
	name := "Test User"
	email := "test@example.com"
	avatarURL := "http://avatar.com/test.jpg"
	provider := "google"

	if u != nil {
		if u.ID != uuid.Nil {
			userID = u.ID.String()
		}
		if u.Name != "" {
			name = u.Name
		}
		if u.Email != "" {
			email = u.Email
		}
		if u.AvatarUrl != "" {
			avatarURL = u.AvatarUrl
		}
		if u.Provider != "" {
			provider = u.Provider
		}
	}

	return goth.User{
		UserID:    userID,
		Name:      name,
		Email:     email,
		AvatarURL: avatarURL,
		Provider:  provider,
	}
}

func TestCallbackHandler(t *testing.T) {
	testCases := []struct {
		name         string
		gothProvider goth.Provider
		provider     string
	}{
		{
			name:         "test callback handler from Google",
			provider:     "google",
			gothProvider: google.New("mock-client-id", "mock-client-secret", "/auth/google/callback"),
		},
		{
			name:         "test callback handler from Facebook",
			gothProvider: facebook.New("mock-client-id", "mock-client-secret", "/auth/facebook/callback"),
			provider:     "facebook",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			truncateData(t)

			userMock := mockGothUser(nil)
			userMock.Provider = "google"
			providerUserID := "1234567890"
			userMock.AvatarURL = "http://avatar.com/test.jpg"
			createUser(t,
				userMock.Email, userMock.Name, userMock.Provider, providerUserID, userMock.AvatarURL,
			)

			goth.UseProviders(tc.gothProvider)

			gothic.GetProviderName = func(req *http.Request) (string, error) {
				return tc.provider, nil
			}
			gothic.CompleteUserAuth = func(w http.ResponseWriter, r *http.Request) (goth.User, error) {
				return userMock, nil
			}

			rr := execRequestWithoutCookie("GET", "/auth/"+tc.provider+"/callback", nil)

			assert.Equal(t, http.StatusTemporaryRedirect, rr.Code)
			assert.Contains(t, rr.Header().Get("Location"), os.Getenv("SITE_URL"))
			assert.Contains(t, rr.Result().Header.Get("Set-Cookie"), auth.SessionName)
		})
	}
}

func TestLogoutHandler(t *testing.T) {
	truncateData(t)

	userMock := mockGothUser(nil)

	req := httptest.NewRequest("GET", "/logout", nil)
	rr := httptest.NewRecorder()

	session, _ := gothic.Store.Get(req, "ama_session")
	session.Values["sessionID"] = userMock.UserID
	session.Save(req, rr)

	Router.ServeHTTP(rr, req)

	response := rr.Result()
	defer response.Body.Close()
	values := response.Header.Values("Set-Cookie")

	newSession, _ := gothic.Store.Get(req, "ama_session")
	assert.Equal(t, http.StatusTemporaryRedirect, rr.Code)
	assert.Len(t, values, 2)
	assert.Contains(t, values[1], "Thu, 01 Jan 1970 00:00:01 GMT")
	assert.Empty(t, newSession.Values["sessionID"])
}
