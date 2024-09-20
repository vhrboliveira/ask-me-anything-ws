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

const UserIDKey contextKey = "user_id"

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

type SessionAuthUser struct {
	ID        string `db:"id" json:"id"`
	Email     string `db:"email" json:"email"`
	Name      string `db:"name" json:"name"`
	AvatarUrl string `db:"avatar_url" json:"avatar_url"`
}

func AuthInit(valkeyClient *valkey.Client, userService *service.UserService) {
	gob.Register(SessionAuthUser{})

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

func decrypt(ciphertext []byte) ([]byte, error) {
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

		ctx := context.Background()
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

		decryptedSession, err := decrypt([]byte(encryptedSession))
		if err != nil {
			slog.Error("Failed to decrypt session", "error", err)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var userSessionValues SessionAuthUser
		err = gob.NewDecoder(bytes.NewBuffer(decryptedSession)).Decode(&userSessionValues)
		if err != nil {
			slog.Error("Failed to deserialize user data", "error", err)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if userSessionValues.ID == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Set userID in request context
		ctx = context.WithValue(r.Context(), UserIDKey, userSessionValues.ID)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

func CallbackHandler(w http.ResponseWriter, r *http.Request) {
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

	dbUser, err := UserService.GetUserByEmail(r.Context(), oauthUser.Email)
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
			AvatarUrl:      oauthUser.AvatarURL,
			Provider:       provider,
			ProviderUserID: oauthUser.UserID,
		}

		userID, err := UserService.CreateUser(r.Context(), dbUser)
		if err != nil {
			http.Error(w, "Error authenticating", http.StatusInternalServerError)
			return
		}

		dbUser.ID = userID
	}

	session.Values["sessionID"] = dbUser.ID.String()
	err = session.Save(r, w)
	if err != nil {
		slog.Error("Failed to save session", "error", err)
		http.Error(w, "Error authenticating", http.StatusInternalServerError)
		return
	}

	newUser := SessionAuthUser{
		ID:        dbUser.ID.String(),
		Email:     dbUser.Email,
		Name:      dbUser.Name,
		AvatarUrl: dbUser.AvatarUrl,
	}

	var buf bytes.Buffer
	err = gob.NewEncoder(&buf).Encode(&newUser)
	if err != nil {
		slog.Error("Failed to serialize user data", "error", err)
		http.Error(w, "Error authenticating", http.StatusInternalServerError)
		return
	}

	encryptedSession, err := encrypt(buf.Bytes())
	if err != nil {
		slog.Error("Failed to encrypt session", "error", err)
		http.Error(w, "Error authenticating", http.StatusInternalServerError)
		return
	}

	ctx := context.Background()
	sessionID := session.Values["sessionID"].(string)
	result := cache.Do(ctx, cache.B().Set().Key(sessionID).Value(string(encryptedSession)).Ex(oneDayInDuration).Build())
	if result.Error() != nil {
		slog.Error("Failed to store session", "error", result.Error())
		http.Error(w, "Error authenticating", http.StatusInternalServerError)
		return
	}

	SITE_URL := os.Getenv("SITE_URL")
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
