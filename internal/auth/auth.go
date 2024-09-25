package auth

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/facebook"
	"github.com/markbates/goth/providers/google"
	"github.com/valkey-io/valkey-go"
	"github.com/vhrboliveira/ama-go/internal/service"
	"github.com/vhrboliveira/ama-go/internal/store/pgstore"
)

type contextKey string

const UserKey contextKey = "user"

const (
	SessionName      = "ama_session"
	thirtyDaysInSec  = 30 * 24 * 60 * 60
	oneDayInDuration = time.Hour * 24
)

var (
	store       *sessions.CookieStore
	cache       valkey.Client
	encryptKey  []byte
	UserService *service.UserService
)

func AuthInit(valkeyClient *valkey.Client, userService *service.UserService) {
	gob.Register(pgstore.User{})

	store = sessions.NewCookieStore([]byte(os.Getenv("COOKIE_SECRET")))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   thirtyDaysInSec,
		HttpOnly: true,
		Secure:   os.Getenv("GO_ENV") == "production",
	}

	cache = *valkeyClient

	UserService = userService

	gothic.Store = store

	goth.UseProviders(
		google.New(os.Getenv("GOOGLE_CLIENT_ID"), os.Getenv("GOOGLE_CLIENT_SECRET"), os.Getenv("API_URL")+"/auth/google/callback"),
		facebook.New(os.Getenv("FACEBOOK_CLIENT_ID"), os.Getenv("FACEBOOK_CLIENT_SECRET"), os.Getenv("API_URL")+"/auth/facebook/callback"),
	)

	var err error
	encryptKey, err = hex.DecodeString(os.Getenv("ENCRYPT_KEY"))
	if err != nil {
		log.Fatalf("Failed to decode encrypt key: %v", err)
	}
}

func encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(encryptKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

func Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(encryptKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

func SetSessionData(ctx context.Context, dbUser pgstore.User) error {
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(&dbUser)
	if err != nil {
		slog.Error("failed to serialize user data", "error", err)
		return err
	}

	encryptedSession, err := encrypt(buf.Bytes())
	if err != nil {
		slog.Error("failed to encrypt session", "error", err)
		return err
	}

	result := cache.Do(ctx, cache.B().Set().Key(dbUser.ID.String()).Value(string(encryptedSession)).Ex(oneDayInDuration).Build())
	if result.Error() != nil {
		slog.Error("failed to store session", "error", result.Error())
		return result.Error()
	}

	return nil
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := store.Get(r, SessionName)
		if err != nil {
			slog.Error("error getting session", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		sessionID, ok := session.Values["sessionID"].(string)
		if !ok {
			slog.Error("unauthorized", "error", "sessionID not found in session")
			http.Error(w, "unauthorized, session not found or invalid", http.StatusUnauthorized)
			return
		}

		ctx := r.Context()
		result := cache.Do(ctx, cache.B().Get().Key(sessionID).Build())
		if result.Error() != nil {
			slog.Error("unauthorized", "error", result.Error())
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		encryptedSession, err := result.ToString()
		if err != nil {
			slog.Error("Failed to convert result to string", "error", err)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		decryptedSession, err := Decrypt([]byte(encryptedSession))
		if err != nil {
			slog.Error("Failed to decrypt session", "error", err)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var userSessionValues pgstore.User
		err = gob.NewDecoder(bytes.NewBuffer(decryptedSession)).Decode(&userSessionValues)
		if err != nil {
			slog.Error("failed to deserialize user data", "error", err)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if userSessionValues == (pgstore.User{}) || userSessionValues.ID == uuid.Nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Set user in request context
		ctx = context.WithValue(r.Context(), UserKey, userSessionValues)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

func CallbackHandler(w http.ResponseWriter, r *http.Request) {
	SITE_URL := os.Getenv("SITE_URL")
	provider := chi.URLParam(r, "provider")
	r = r.WithContext(context.WithValue(r.Context(), gothic.ProviderParamKey, provider))

	oauthUser, err := gothic.CompleteUserAuth(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	session, err := store.Get(r, SessionName)
	if err != nil {
		slog.Error("Failed to get session", "error", err)
		http.Error(w, "Error authenticating", http.StatusInternalServerError)
		return
	}

	if oauthUser.Email == "" {
		slog.Error("User email not found", "user", oauthUser)
		http.Error(w, "Error authenticating", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	dbUser, err := UserService.GetUserByEmail(ctx, oauthUser.Email)
	if err != nil {
		slog.Error("Failed to get user", "error", err)
		http.Error(w, "Error authenticating", http.StatusInternalServerError)
		return
	}

	if dbUser == (pgstore.User{}) {
		var name string
		if oauthUser.Name != "" {
			name = oauthUser.Name
		} else {
			name = oauthUser.FirstName + " " + oauthUser.LastName
		}

		dbUser = pgstore.User{
			Email:          oauthUser.Email,
			Name:           strings.TrimSpace(name),
			Photo:          oauthUser.AvatarURL,
			Provider:       provider,
			ProviderUserID: oauthUser.UserID,
			EnablePicture:  true,
			NewUser:        true,
		}

		userID, createdAt, updatedAt, err := UserService.CreateUser(ctx, dbUser)
		if err != nil {
			http.Error(w, "Error authenticating", http.StatusInternalServerError)
			return
		}

		dbUser.ID = userID
		dbUser.CreatedAt = createdAt
		dbUser.UpdatedAt = updatedAt
	}

	if dbUser.Provider != provider {
		slog.Error("user email already exists with a different provider", "expected", dbUser.Provider, "provided", provider)
		http.Redirect(w, r, SITE_URL+"/profile/error", http.StatusTemporaryRedirect)
		return
	}

	session.Values["sessionID"] = dbUser.ID.String()
	err = session.Save(r, w)
	if err != nil {
		slog.Error("failed to save session", "error", err)
		http.Error(w, "error authenticating", http.StatusInternalServerError)
		return
	}

	err = SetSessionData(ctx, dbUser)
	if err != nil {
		http.Error(w, "error authenticating", http.StatusInternalServerError)
		return
	}

	if dbUser.NewUser {
		SITE_URL += "/profile"
	}

	http.Redirect(w, r, SITE_URL, http.StatusTemporaryRedirect)
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	r = r.WithContext(context.WithValue(r.Context(), gothic.ProviderParamKey, provider))

	gothic.BeginAuthHandler(w, r)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	SITE_URL := os.Getenv("SITE_URL")

	session, err := store.Get(r, SessionName)
	if err != nil {
		slog.Error("unauthorized", "error", err)
		http.SetCookie(w, generateExpiredCookie())
		http.Redirect(w, r, SITE_URL, http.StatusTemporaryRedirect)
		return
	}

	sessionID, ok := session.Values["sessionID"].(string)
	if !ok {
		slog.Error("sessionID not found in session")
		http.SetCookie(w, generateExpiredCookie())
		http.Redirect(w, r, SITE_URL, http.StatusTemporaryRedirect)
		return
	}

	ctx := context.Background()
	result := cache.Do(ctx, cache.B().Del().Key(sessionID).Build())
	if result.Error() != nil {
		slog.Error("Failed to delete session", "error", result.Error())
		http.SetCookie(w, generateExpiredCookie())
		http.Error(w, "Failed to delete session", http.StatusInternalServerError)
		return
	}

	session.Options.MaxAge = -1
	session.Values = nil
	session.ID = ""
	err = session.Save(r, w)
	if err != nil {
		slog.Error("Failed to save session", "error", err)
		http.SetCookie(w, generateExpiredCookie())
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, SITE_URL, http.StatusTemporaryRedirect)
}

func generateExpiredCookie() *http.Cookie {
	return &http.Cookie{
		Name:     SessionName,
		Value:    "",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		Path:     "/",
		HttpOnly: true,
	}
}
