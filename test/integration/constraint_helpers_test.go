package api_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/vhrboliveira/ama-go/internal/store/pgstore"
)

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

	_, err := DBPool.Exec(ctx, "ALTER TABLE messages RENAME TO old_messages;")
	if err != nil {
		t.Fatalf("Failed to add constraint: %v", err)
	}

	t.Cleanup(func() {
		_, err := DBPool.Exec(ctx, "ALTER TABLE old_messages RENAME TO messages;")
		if err != nil {
			t.Fatalf("Failed to remove constraint: %v", err)
		}
	})
}

func setMessagesReactionsConstraint(t testing.TB) {
	t.Helper()

	ctx := context.Background()

	t.Cleanup(func() {
		_, err := DBPool.Exec(ctx, "ALTER TABLE mr RENAME TO messages_reactions")
		require.NoError(t, err, "failed to remove constraint when setting message reaction constraint")
	})

	_, err := DBPool.Exec(ctx, "ALTER TABLE messages_reactions RENAME TO mr;")
	require.NoError(t, err, "failed to set constraint when setting message reaction constraint")
}

func setAnswerMessageConstraintFailure(t testing.TB, roomID uuid.UUID) {
	t.Helper()

	ctx := context.Background()

	t.Cleanup(func() {
		_, err := DBPool.Exec(ctx, "ALTER TABLE messages DROP CONSTRAINT msg_chk_answer;")
		require.NoError(t, err, "failed to remove constraint when setting answer message constraint")
	})

	_, err := DBPool.Exec(ctx, "ALTER TABLE messages ADD CONSTRAINT msg_chk_answer UNIQUE(answered);")
	require.NoError(t, err, "failed to set constraint when setting answer message constraint")

	messages := []pgstore.InsertMessageParams{
		{
			RoomID:  roomID,
			Message: "message test",
		},
	}

	insertMessages(t, messages)

	_, err = DBPool.Exec(ctx, "UPDATE messages SET answered = true")
	require.NoError(t, err, "failed to update constraint message when setting answer message constraint")
}
