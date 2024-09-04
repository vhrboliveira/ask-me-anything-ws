package api_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/jwtauth/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/vhrboliveira/ama-go/internal/auth"
	"github.com/vhrboliveira/ama-go/internal/store/pgstore"
)

func getAuthToken() string {
	token, err := auth.GenerateJWT(uuid.New(), "test@test.com")
	if err != nil {
		panic(err)
	}

	return token
}

func execRequest(method, url string, body io.Reader) *httptest.ResponseRecorder {
	token := "Bearer " + getAuthToken()

	r := httptest.NewRequest(method, url, body)
	r.Header.Set("Authorization", token)

	rr := httptest.NewRecorder()

	Router.ServeHTTP(rr, r)

	return rr
}

func execRequestWithoutAuth(method, url string, body io.Reader) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, url, body)

	rr := httptest.NewRecorder()

	Router.ServeHTTP(rr, r)

	return rr
}

func execRequestWithInvalidAuth(method, url string, body io.Reader) *httptest.ResponseRecorder {
	token := "Bearer " + getAuthToken()
	token = token[:len(token)-1]

	r := httptest.NewRequest(method, url, body)
	r.Header.Set("Authorization", token)

	rr := httptest.NewRecorder()

	Router.ServeHTTP(rr, r)

	return rr
}

func assertStatusCode(t testing.TB, response *http.Response, expected int) {
	t.Helper()
	if response.StatusCode != expected {
		t.Errorf("Expected %d, Got: %d", expected, response.StatusCode)
	}
}

func assertResponse(t testing.TB, want, got string) {
	t.Helper()
	if want != got {
		t.Errorf("Expected %q, Got: %q", want, got)
	}
}

func parseResponseBody(t testing.TB, response *http.Response) []byte {
	t.Helper()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("Failed to parse response body: %v", err)
	}

	return body
}

func assertValidUUID(t testing.TB, id string) {
	t.Helper()
	if _, err := uuid.Parse(id); err != nil {
		t.Errorf("ID is not a valid UUID: %q. Error: %v", id, err)
	}
}

func assertValidDate(t testing.TB, date string) {
	t.Helper()
	if _, err := time.Parse(time.RFC3339, date); err != nil {
		t.Errorf("Date is not a valid date with format YYYY-MM-DDTHH:mm:ssZ: %v", err)
	}
}

func assertValidToken(t testing.TB, token string) {
	t.Helper()
	jwtSecret := os.Getenv("JWT_SECRET")
	tokenAuth := jwtauth.New("HS256", []byte(jwtSecret), nil)
	_, err := jwtauth.VerifyToken(tokenAuth, token)
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}
}

func truncateTables(t testing.TB) {
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
}

func createRooms(t testing.TB, names []string) {
	t.Helper()
	ctx := context.Background()

	tx, err := DBPool.Begin(ctx)
	if err != nil {
		t.Fatalf("Failed to create rooms: %v", err)
	}
	defer tx.Rollback(ctx)

	stmt := `INSERT INTO rooms (name) VALUES ($1)`

	batch := &pgx.Batch{}
	for _, name := range names {
		batch.Queue(stmt, name)
	}

	bx := tx.SendBatch(ctx, batch)
	err = bx.Close()
	if err != nil {
		t.Fatalf("Failed to create rooms: %v", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("Failed to create rooms: %v", err)
	}
}

func getRoomByName(t testing.TB, name string) pgstore.GetRoomRow {
	t.Helper()
	ctx := context.Background()

	rows, err := DBPool.Query(ctx, "SELECT id, name FROM rooms WHERE name = $1", name)
	if err != nil {
		t.Fatalf("Failed to get room: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatalf("Failed to get room: %v", err)
	}

	var room pgstore.GetRoomRow
	err = rows.Scan(&room.ID, &room.Name)
	if err != nil {
		t.Fatalf("Failed to scan room: %v", err)
	}

	return room
}

func createAndGetRoom(t testing.TB) pgstore.GetRoomRow {
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

	row := DBPool.QueryRow(ctx, "SELECT id FROM messages WHERE message = $1", message)

	var id uuid.UUID
	err := row.Scan(&id)
	if err != nil {
		t.Fatalf("Failed to scan message: %v", err)
	}

	return id.String()
}

func createAndGetMessages(t testing.TB, roomID uuid.UUID) string {
	t.Helper()

	msgs := []pgstore.InsertMessageParams{
		{
			RoomID:  roomID,
			Message: "message 1",
		},
	}

	insertMessages(t, msgs)
	return getMessageIDByMessage(t, msgs[0].Message)
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

func setRoomsConstraintFailure(t testing.TB) {
	t.Helper()
	ctx := context.Background()

	t.Cleanup(func() {
		_, err := DBPool.Exec(ctx, "ALTER TABLE old_rooms RENAME TO rooms;")
		if err != nil {
			t.Fatalf("Failed to remove constraint: %v", err)
		}
	})

	_, err := DBPool.Exec(ctx, "ALTER TABLE rooms RENAME TO old_rooms;")
	if err != nil {
		t.Fatalf("Failed to add constraint: %v", err)
	}
}

func setMessagesConstraintFailure(t testing.TB) {
	t.Helper()
	ctx := context.Background()

	t.Cleanup(func() {
		_, err := DBPool.Exec(ctx, "ALTER TABLE old_messages RENAME TO messages;")
		if err != nil {
			t.Fatalf("Failed to remove constraint: %v", err)
		}
	})

	_, err := DBPool.Exec(ctx, "ALTER TABLE messages RENAME TO old_messages;")
	if err != nil {
		t.Fatalf("Failed to add constraint: %v", err)
	}
}

func setUpdateMessageReactionConstraintFailure(t testing.TB, roomID uuid.UUID) {
	t.Helper()

	ctx := context.Background()

	t.Cleanup(func() {
		_, err := DBPool.Exec(ctx, "ALTER TABLE messages DROP CONSTRAINT msg_chk_reaction_count;")
		if err != nil {
			t.Fatalf("Failed to remove constraint: %v", err)
		}
	})

	_, err := DBPool.Exec(ctx, "ALTER TABLE messages ADD CONSTRAINT msg_chk_reaction_count UNIQUE(reaction_count);")
	if err != nil {
		t.Fatalf("Failed to set constraint: %v", err)
	}

	messages := []pgstore.InsertMessageParams{
		{
			RoomID:  roomID,
			Message: "message test",
		},
	}

	insertMessages(t, messages)

	_, err = DBPool.Exec(ctx, "UPDATE messages SET reaction_count = 1")
	if err != nil {
		t.Fatalf("Failed to update constraint message: %v", err)
	}
}

func setDeleteMessageReactionConstraintFailure(t testing.TB, roomID uuid.UUID, msgID string) {
	t.Helper()

	ctx := context.Background()

	t.Cleanup(func() {
		_, err := DBPool.Exec(ctx, "ALTER TABLE messages DROP CONSTRAINT msg_chk_reaction_count;")
		if err != nil {
			t.Fatalf("Failed to remove constraint: %v", err)
		}
	})

	messages := []pgstore.InsertMessageParams{
		{
			RoomID:  roomID,
			Message: "message test",
		},
	}
	insertMessages(t, messages)

	_, err := DBPool.Exec(ctx, "UPDATE messages SET reaction_count = 1 WHERE id != $1", msgID)
	if err != nil {
		t.Fatalf("Failed to update constraint message: %v", err)
	}

	setMessageReaction(t, msgID, 2)

	_, err = DBPool.Exec(ctx, "ALTER TABLE messages ADD CONSTRAINT msg_chk_reaction_count UNIQUE(reaction_count);")
	if err != nil {
		t.Fatalf("Failed to set constraint: %v", err)
	}
}

func setAnswerMessageConstraintFailure(t testing.TB, roomID uuid.UUID) {
	t.Helper()

	ctx := context.Background()

	t.Cleanup(func() {
		_, err := DBPool.Exec(ctx, "ALTER TABLE messages DROP CONSTRAINT msg_chk_answer;")
		if err != nil {
			t.Fatalf("Failed to remove constraint: %v", err)
		}
	})

	_, err := DBPool.Exec(ctx, "ALTER TABLE messages ADD CONSTRAINT msg_chk_answer UNIQUE(answered);")
	if err != nil {
		t.Fatalf("Failed to set constraint: %v", err)
	}

	messages := []pgstore.InsertMessageParams{
		{
			RoomID:  roomID,
			Message: "message test",
		},
	}

	insertMessages(t, messages)

	_, err = DBPool.Exec(ctx, "UPDATE messages SET answered = true")
	if err != nil {
		t.Fatalf("Failed to update constraint message: %v", err)
	}
}

func createUser(t testing.TB, email, password, name string) {
	t.Helper()

	ctx := context.Background()

	user := pgstore.CreateUserParams{
		Email:        email,
		PasswordHash: password,
		Name:         name,
	}

	DBPool.Exec(ctx, "INSERT INTO users (email, password_hash, name) VALUES ($1, $2, $3)", user.Email, user.PasswordHash, user.Name)
}

func setCreateUserConstraintError(t testing.TB) {
	t.Helper()

	ctx := context.Background()

	t.Cleanup(func() {
		_, err := DBPool.Exec(ctx, "ALTER TABLE users2 RENAME TO users;")
		if err != nil {
			t.Fatalf("Failed to remove constraint: %v", err)
		}
	})

	_, err := DBPool.Exec(ctx, "ALTER TABLE users RENAME TO users2;")
	if err != nil {
		t.Fatalf("Failed to set constraint: %v", err)
	}
}
