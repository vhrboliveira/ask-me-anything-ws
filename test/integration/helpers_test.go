package api_test

import (
	"context"
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

	gothUser := mockGothUser()

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

func execRequestGeneratingSession(t testing.TB, method, url string, body io.Reader, userID *string) *httptest.ResponseRecorder {
	t.Helper()

	gothUser := mockGothUser()
	gothUser.UserID = *userID

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
	session.Values["sessionID"] = *userID
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

	gothUser := mockGothUser()
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
		return mockGothUser(), nil
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
		`
	_, err := DBPool.Exec(context.Background(), query)

	if err != nil {
		t.Fatalf("Failed to truncate tables: %v", err)
	}

	ValkeyClient.Do(context.Background(), ValkeyClient.B().Flushall().Build())
}

func createRooms(t testing.TB, names []string) {
	t.Helper()
	ctx := context.Background()

	userID := generateUser(t)

	tx, err := DBPool.Begin(ctx)
	if err != nil {
		t.Fatalf("Failed to begin transaction to create rooms: %v", err)
	}
	defer tx.Rollback(ctx)

	stmt := `INSERT INTO rooms (name, user_id) VALUES ($1, $2)`

	batch := &pgx.Batch{}
	for _, name := range names {
		batch.Queue(stmt, name, userID)
	}

	bx := tx.SendBatch(ctx, batch)
	err = bx.Close()
	if err != nil {
		t.Fatalf("Failed to create rooms: %v", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("Failed to commit the transactions to create rooms: %v", err)
	}
}

func getRoomByName(t testing.TB, name string) pgstore.Room {
	t.Helper()
	ctx := context.Background()

	rows, err := DBPool.Query(ctx, "SELECT id, name, user_id FROM rooms WHERE name = $1", name)
	if err != nil {
		t.Fatalf("Failed to get room: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatalf("Failed to get room: %v", err)
	}

	var room pgstore.Room
	err = rows.Scan(&room.ID, &room.Name, &room.UserID)
	if err != nil {
		t.Fatalf("Failed to scan room: %v", err)
	}

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
	if err != nil {
		t.Fatalf("Failed to begin messages transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	stmt := `INSERT INTO messages (room_id, message) VALUES ($1, $2)`

	batch := &pgx.Batch{}
	for _, elem := range msgs {
		batch.Queue(stmt, elem.RoomID, elem.Message)
	}

	bx := tx.SendBatch(ctx, batch)
	err = bx.Close()
	if err != nil {
		t.Fatalf("Failed to send batch when inserting messages: %v", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("Failed to insert messages: %v", err)
	}
}

func getMessageIDByMessage(t testing.TB, message string) string {
	t.Helper()

	ctx := context.Background()

	row := DBPool.QueryRow(ctx, "SELECT id, answered FROM messages WHERE message = $1", message)

	var id uuid.UUID
	var ans bool
	err := row.Scan(&id, &ans)
	if err != nil {
		t.Fatalf("Failed to scan message: %v", err)
	}

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

	row := DBPool.QueryRow(ctx, "SELECT reaction_count FROM messages WHERE id = $1", messageID)

	var count int
	err := row.Scan(&count)
	if err != nil {
		t.Fatalf("Failed to scan message: %v", err)
	}

	return count
}

func setMessageReaction(t testing.TB, messageID string, count int) {
	t.Helper()

	ctx := context.Background()

	_, err := DBPool.Exec(ctx, "UPDATE messages SET reaction_count = $1 WHERE id = $2", count, messageID)
	if err != nil {
		t.Fatalf("Failed to update message: %v", err)
	}
}

func getUserIDByEmail(t testing.TB, email string) string {
	t.Helper()

	ctx := context.Background()
	user := DBPool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", email)
	var id string
	err := user.Scan(&id)
	if err != nil {
		t.Fatalf("Failed to scan user: %v", err)
	}

	return id
}

func generateUser(t testing.TB) string {
	t.Helper()

	gothUser := mockGothUser()
	email := gothUser.Email
	name := gothUser.Name

	createUser(t, email, name)

	return getUserIDByEmail(t, email)
}

func createUser(t testing.TB, email, name string) {
	t.Helper()

	ctx := context.Background()

	user := pgstore.CreateUserParams{
		Email: email,
		Name:  name,
	}

	DBPool.Exec(ctx, "INSERT INTO users (email, name) VALUES ($1, $2)", user.Email, user.Name)
}
