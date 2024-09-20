package api_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
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

	t.Cleanup(func() {
		_, err := DBPool.Exec(ctx, "ALTER TABLE messages DROP CONSTRAINT msg_chk_answer;")
		if err != nil {
			t.Fatalf("Failed to remove constraint: %v", err)
		}
	})
}
