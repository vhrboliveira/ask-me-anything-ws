package service

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/vhrboliveira/ama-go/internal/store/pgstore"
)

type MessageService struct {
	Queries *pgstore.Queries
}

func NewMessageService(queries *pgstore.Queries) *MessageService {
	return &MessageService{Queries: queries}
}

func (s *MessageService) CreateMessage(ctx context.Context, roomID uuid.UUID, msg string) (pgstore.InsertMessageRow, error) {
	message, err := s.Queries.InsertMessage(ctx, pgstore.InsertMessageParams{
		RoomID:  roomID,
		Message: msg,
	})

	return message, err
}

func (s *MessageService) GetMessages(ctx context.Context, roomID uuid.UUID) ([]pgstore.Message, error) {
	roomMessages, err := s.Queries.GetRoomMessages(ctx, roomID)

	if roomMessages == nil {
		roomMessages = []pgstore.Message{}
	}

	return roomMessages, err
}

func (s *MessageService) GetMessage(ctx context.Context, messageID uuid.UUID) (pgstore.Message, error) {
	message, err := s.Queries.GetMessage(ctx, messageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Error("message not found", "error", err)
			message = pgstore.Message{}
			err = nil
		}
	}

	return message, err
}

func (s *MessageService) CheckMessageExists(ctx context.Context, messageID uuid.UUID) (int, error) {
	_, err := s.Queries.GetMessage(ctx, messageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Error("message not found", "error", err)
			return http.StatusBadRequest, errors.New("message not found")
		}

		slog.Error("error checking if message exists", "error", err)
		return http.StatusInternalServerError, errors.New("error validating message ID")
	}

	return http.StatusOK, nil
}

func (s *MessageService) ReactToMessage(ctx context.Context, messageID uuid.UUID) (int32, error) {
	count, err := s.Queries.ReactToMessage(ctx, messageID)

	return count, err
}

func (s *MessageService) RemoveReactionFromMessage(ctx context.Context, messageID uuid.UUID) (int32, error) {
	count, err := s.Queries.RemoveReactionFromMessage(ctx, messageID)
	if err != nil {
		count = 0

		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("error reacting to message", "error", err)
			return 0, errors.New("error reacting to message")
		}
	}

	return count, nil
}

func (s *MessageService) AnswerMessage(ctx context.Context, messageID uuid.UUID, answer string) error {
	params := pgstore.AnswerMessageParams{
		ID:     messageID,
		Answer: answer,
	}

	_, err := s.Queries.AnswerMessage(ctx, params)

	return err
}
