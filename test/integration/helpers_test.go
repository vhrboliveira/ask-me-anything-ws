package api_test

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vhrboliveira/ama-go/internal/auth"
	"github.com/vhrboliveira/ama-go/internal/store/pgstore"
)

func execAuthenticatedRequest(t testing.TB, method, url string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()

	gothUser := mockGothUser(nil)

	gothic.GetProviderName = func(req *http.Request) (string, error) {
		return "google", nil
	}
	gothic.CompleteUserAuth = func(w http.ResponseWriter, r *http.Request) (goth.User, error) {
		return gothUser, nil
	}

	userID := generateUser(t)

	callbackReq := httptest.NewRequest("GET", "/auth/google/callback", nil)
	callbackRec := httptest.NewRecorder()

	Router.ServeHTTP(callbackRec, callbackReq)

	r := httptest.NewRequest(method, url, body)

	rr := httptest.NewRecorder()

	session, _ := gothic.Store.Get(r, auth.SessionName)
	session.Values["sessionID"] = userID
	session.Save(r, rr)

	Router.ServeHTTP(rr, r)

	return rr
}

func execRequestWithoutCookie(method, url string, body io.Reader) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, url, body)

	rr := httptest.NewRecorder()

	Router.ServeHTTP(rr, r)

	return rr
}

func execRequestWithInvalidCookie(method, url string, body io.Reader) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, url, body)
	rr := httptest.NewRecorder()

	session, _ := gothic.Store.Get(r, "different_cookie")
	session.Values["sessionID"] = "any_id_on_the_session"
	session.Save(r, rr)

	Router.ServeHTTP(rr, r)

	return rr
}

func generateSession(t testing.TB, user *pgstore.User) {
	t.Helper()

	gothUser := mockGothUser(user)

	gothic.GetProviderName = func(req *http.Request) (string, error) {
		return "google", nil
	}
	gothic.CompleteUserAuth = func(w http.ResponseWriter, r *http.Request) (goth.User, error) {
		return gothUser, nil
	}

	callbackReq := httptest.NewRequest("GET", "/auth/google/callback", nil)
	callbackRec := httptest.NewRecorder()

	Router.ServeHTTP(callbackRec, callbackReq)
}

func execRequestGettingSession(t testing.TB, method, url string, body io.Reader, userID string) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(method, url, body)
	rr := httptest.NewRecorder()

	session, _ := gothic.Store.Get(r, auth.SessionName)
	session.Values["sessionID"] = userID
	session.Save(r, rr)

	Router.ServeHTTP(rr, r)

	return rr
}

func execRequestGeneratingSession(t testing.TB, method, url string, body io.Reader, user *pgstore.User) *httptest.ResponseRecorder {
	t.Helper()

	gothUser := mockGothUser(user)

	gothic.GetProviderName = func(req *http.Request) (string, error) {
		return "google", nil
	}
	gothic.CompleteUserAuth = func(w http.ResponseWriter, r *http.Request) (goth.User, error) {
		return gothUser, nil
	}

	callbackReq := httptest.NewRequest("GET", "/auth/google/callback", nil)
	callbackRec := httptest.NewRecorder()

	Router.ServeHTTP(callbackRec, callbackReq)

	r := httptest.NewRequest(method, url, body)
	rr := httptest.NewRecorder()

	session, _ := gothic.Store.Get(r, auth.SessionName)
	session.Values["sessionID"] = gothUser.UserID
	session.Save(r, rr)

	Router.ServeHTTP(rr, r)

	return rr
}

func connectAuthenticatedWS(t testing.TB, wsURL string) (*websocket.Conn, error) {
	t.Helper()

	userID := generateUser(t)
	ws, _, err := connectWSWithUserSession(t, wsURL, &userID)
	return ws, err
}

func connectWSWithUserSession(t testing.TB, wsURL string, userID *string) (*websocket.Conn, *http.Response, error) {
	t.Helper()

	gothUser := mockGothUser(nil)
	gothUser.UserID = *userID

	gothic.GetProviderName = func(req *http.Request) (string, error) {
		return "google", nil
	}
	gothic.CompleteUserAuth = func(w http.ResponseWriter, r *http.Request) (goth.User, error) {
		return gothUser, nil
	}

	r := httptest.NewRequest("GET", "/auth/google/callback", nil)
	rr := httptest.NewRecorder()

	Router.ServeHTTP(rr, r)

	response := rr.Result()
	defer response.Body.Close()
	values := response.Header.Values("Set-Cookie")

	headers := http.Header{}
	headers.Add("Cookie", values[0])

	wsConn, res, err := websocket.DefaultDialer.Dial(wsURL, headers)

	return wsConn, res, err
}

func connectWSWithoutSession(t testing.TB, wsURL string) (*websocket.Conn, *http.Response, error) {
	t.Helper()

	gothic.GetProviderName = func(req *http.Request) (string, error) {
		return "google", nil
	}
	gothic.CompleteUserAuth = func(w http.ResponseWriter, r *http.Request) (goth.User, error) {
		return mockGothUser(nil), nil
	}

	r := httptest.NewRequest("GET", "/auth/google/callback", nil)
	rr := httptest.NewRecorder()

	Router.ServeHTTP(rr, r)

	headers := http.Header{}
	headers.Add("Cookie", "invalidCookie")

	wsConn, res, err := websocket.DefaultDialer.Dial(wsURL, headers)

	return wsConn, res, err
}

func parseResponseBody(t testing.TB, response *http.Response) string {
	t.Helper()

	body, err := io.ReadAll(response.Body)
	require.NoError(t, err, "failed to parse response body")

	return string(body)
}

func assertValidUUID(t testing.TB, id string) {
	t.Helper()

	_, err := uuid.Parse(id)
	assert.NoError(t, err, "Expected valid UUID, got an error parsing it")
}

func assertValidDate(t testing.TB, date string) {
	t.Helper()

	_, err := time.Parse(time.RFC3339, date)
	assert.NoError(t, err, "Date is not a valid date with format YYYY-MM-DDTHH:mm:ssZ")
}

func truncateData(t testing.TB) {
	t.Helper()

	query := `
		TRUNCATE TABLE rooms RESTART IDENTITY CASCADE;
		TRUNCATE TABLE messages RESTART IDENTITY CASCADE;
		TRUNCATE TABLE users RESTART IDENTITY CASCADE;
		TRUNCATE TABLE messages_reactions RESTART IDENTITY CASCADE;
		`
	_, err := DBPool.Exec(context.Background(), query)
	require.NoError(t, err, "failed to truncate tables")

	ValkeyClient.Do(context.Background(), ValkeyClient.B().Flushall().Build())
}

func createRooms(t testing.TB, names []string) {
	t.Helper()
	ctx := context.Background()

	userID := generateUser(t)

	tx, err := DBPool.Begin(ctx)
	require.NoError(t, err, "failed to begin transaction to create rooms")

	defer tx.Rollback(ctx)

	stmt := `INSERT INTO rooms (name, user_id) VALUES ($1, $2)`

	batch := &pgx.Batch{}
	for _, name := range names {
		batch.Queue(stmt, name, userID)
	}

	bx := tx.SendBatch(ctx, batch)
	err = bx.Close()
	require.NoError(t, err, "failed to create rooms")

	err = tx.Commit(ctx)
	require.NoError(t, err, "failed to commit the transactions to create rooms")
}

func getRoomByName(t testing.TB, name string) pgstore.Room {
	t.Helper()
	ctx := context.Background()

	rows, err := DBPool.Query(ctx, "SELECT id, name, user_id FROM rooms WHERE name = $1", name)
	require.NoError(t, err, "failed to get room")
	defer rows.Close()

	if !rows.Next() {
		t.Fatalf("Failed to get room: %v", err)
	}

	var room pgstore.Room
	err = rows.Scan(&room.ID, &room.Name, &room.UserID)
	require.NoError(t, err, "failed to scan room")
	return room
}

func createAndGetRoom(t testing.TB) pgstore.Room {
	t.Helper()

	roomName := []string{"room"}
	createRooms(t, roomName)
	return getRoomByName(t, roomName[0])
}

func insertMessages(t testing.TB, msgs []pgstore.InsertMessageParams) {
	t.Helper()

	ctx := context.Background()

	tx, err := DBPool.Begin(ctx)
	require.NoError(t, err, "error beginning the transaction to insert message")
	defer tx.Rollback(ctx)

	stmt := `INSERT INTO messages (room_id, message) VALUES ($1, $2)`

	batch := &pgx.Batch{}
	for _, elem := range msgs {
		batch.Queue(stmt, elem.RoomID, elem.Message)
	}

	bx := tx.SendBatch(ctx, batch)
	err = bx.Close()
	require.NoError(t, err, "failed to send batch when inserting messages")

	err = tx.Commit(ctx)
	require.NoError(t, err, "failed to commit transaction when inserting message")
}

func answerMessageByID(t testing.TB, messageID, answer string) {
	t.Helper()

	ctx := context.Background()

	_, err := DBPool.Exec(ctx, "UPDATE messages SET answered = true, answer = $1 WHERE id = $2", answer, messageID)
	require.NoError(t, err, "error answering message by id")
}

func answerMessages(t testing.TB, answer string) {
	t.Helper()

	ctx := context.Background()

	_, err := DBPool.Exec(ctx, "UPDATE messages SET answered = true, answer = $1", answer)
	require.NoError(t, err, "error answering all messages")
}

func getMessageIDByMessage(t testing.TB, message string) string {
	t.Helper()

	ctx := context.Background()

	row := DBPool.QueryRow(ctx, "SELECT id, answered FROM messages WHERE message = $1", message)

	var id uuid.UUID
	var ans bool
	err := row.Scan(&id, &ans)
	require.NoError(t, err, "failed to get message ID by message")

	return id.String()
}

func createAndGetMessages(t testing.TB, roomID uuid.UUID) (string, string) {
	t.Helper()

	msgs := []pgstore.InsertMessageParams{
		{
			RoomID:  roomID,
			Message: "message 1",
		},
	}

	insertMessages(t, msgs)
	return getMessageIDByMessage(t, msgs[0].Message), msgs[0].Message
}

func getMessageReactions(t testing.TB, messageID string) int {
	t.Helper()

	ctx := context.Background()

	row := DBPool.QueryRow(ctx, "SELECT count(*) FROM messages_reactions WHERE message_id = $1", messageID)

	var count int
	err := row.Scan(&count)
	require.NoError(t, err, "failed to scan message while getting message reactions")

	return count
}

func setMessageReactionWithUserID(t testing.TB, messageID, userID string) {
	t.Helper()

	ctx := context.Background()

	t.Cleanup(func() {
		_, err := DBPool.Exec(ctx, "DELETE FROM messages_reactions WHERE message_id = $1 and user_id = $2", messageID, userID)
		require.NoError(t, err, "failed to cleanup message while setting message reaction")
	})

	_, err := DBPool.Exec(ctx, "INSERT INTO messages_reactions (message_id, user_id) VALUES ($1, $2)", messageID, userID)
	require.NoError(t, err, "failed to insert into message while setting message reaction")
}

func setMessageReaction(t testing.TB, messageID string, count int) {
	t.Helper()

	ctx := context.Background()

	_, err := DBPool.Exec(ctx, "UPDATE messages SET reaction_count = $1 WHERE id = $2", count, messageID)
	require.NoError(t, err, "failed to update message while setting message reaction")
}

func getUserIDByEmail(t testing.TB, email string) string {
	t.Helper()

	ctx := context.Background()
	user := DBPool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", email)
	var id string
	err := user.Scan(&id)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("Failed to scan user: %v", err)
		}
	}

	return id
}

func generateUser(t testing.TB) string {
	t.Helper()

	gothUser := mockGothUser(nil)
	email := gothUser.Email
	name := gothUser.Name
	gothUser.Provider = "google"
	providerUserID := "1234567890"
	gothUser.AvatarURL = "http://avatar.com/test.jpg"

	id := getUserIDByEmail(t, email)
	if id == "" {
		id = createUser(t, email, name, gothUser.Provider, providerUserID, gothUser.AvatarURL)
	}

	return id
}

func createUser(t testing.TB, email, name, provider, providerUserId, Photo string) string {
	t.Helper()

	ctx := context.Background()

	user := pgstore.CreateUserParams{
		Email:          email,
		Name:           name,
		Provider:       provider,
		ProviderUserID: providerUserId,
		Photo:          Photo,
	}

	insertQuery := "INSERT INTO users (email, name, provider, provider_user_id, photo) VALUES ($1, $2, $3, $4, $5) returning id"
	row := DBPool.QueryRow(
		ctx,
		insertQuery,
		user.Email, user.Name, user.Provider, user.ProviderUserID, user.Photo,
	)

	var id uuid.UUID
	require.NoError(t, row.Scan(&id))

	return id.String()
}

func setNewUserToFalse(t testing.TB, userID string) {
	t.Helper()

	DBPool.Exec(context.Background(), "UPDATE users set new_user=false where id = $1", userID)
}

func getValkeyData(t testing.TB, sessionID string) pgstore.User {
	gob.Register(pgstore.User{})
	ctx := context.Background()
	result := ValkeyClient.Do(ctx, ValkeyClient.B().Get().Key(sessionID).Build())
	require.NoError(t, result.Error(), "failed to get session ID on valkey data session")

	encryptedSession, err := result.ToString()
	require.NoError(t, err, "failed to get string data from valkey data session")

	decryptedSession, err := auth.Decrypt([]byte(encryptedSession))
	require.NoError(t, err, "failed to decrypt valkey data session")

	var userSessionValues pgstore.User
	err = gob.NewDecoder(bytes.NewBuffer(decryptedSession)).Decode(&userSessionValues)
	require.NoError(t, err, "failed to decode gob from decrypted data")

	return userSessionValues
}
