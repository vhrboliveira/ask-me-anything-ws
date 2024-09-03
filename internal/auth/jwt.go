package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/jwtauth/v5"
	"github.com/google/uuid"
)

var TokenAuth *jwtauth.JWTAuth

type contextKey string

const tokenKey contextKey = "token"

func InitJWT(jwtSecret string) {
	TokenAuth = jwtauth.New("HS256", []byte(jwtSecret), nil)
}

func GenerateJWT(userID uuid.UUID, email string) (string, error) {
	claims := map[string]interface{}{
		"user_id": userID.String(),
		"email":   email,
		"exp":     time.Now().Add(24 * time.Hour).Unix(), // Token expires in 24 hours
	}

	_, tokenString, err := TokenAuth.Encode(claims)
	return tokenString, err
}

func Authenticator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, claims, err := jwtauth.FromContext(r.Context())

		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		if token == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if claims["user_id"] == nil || claims["email"] == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func WebsocketAuthenticator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "no token found", http.StatusUnauthorized)
			return
		}

		r.Header.Set("Authorization", "Bearer "+token)
		next.ServeHTTP(w, r)
	})
}
