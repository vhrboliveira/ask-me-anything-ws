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

type RoomService struct {
	Queries *pgstore.Queries
}

func NewRoomService(queries *pgstore.Queries) *RoomService {
	return &RoomService{Queries: queries}
}

func (s *RoomService) CreateRoom(ctx context.Context, name string, userID uuid.UUID, description string) (pgstore.InsertRoomRow, error) {
	room, err := s.Queries.InsertRoom(ctx, pgstore.InsertRoomParams{
		Name:        name,
		UserID:      userID,
		Description: description,
	})

	return room, err
}

func (s *RoomService) GetRooms(ctx context.Context) ([]pgstore.GetRoomsRow, error) {
	rooms, err := s.Queries.GetRooms(ctx)

	if rooms == nil {
		rooms = []pgstore.GetRoomsRow{}
	}

	return rooms, err
}

func (s *RoomService) GetRoom(ctx context.Context, roomID int64) (pgstore.GetRoomWithUserRow, error) {
	room, err := s.Queries.GetRoomWithUser(ctx, roomID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Error("room not found", "error", err)
			room = pgstore.GetRoomWithUserRow{}
			err = nil
		}
	}

	return room, err
}

func (s *RoomService) CheckRoomExists(ctx context.Context, roomID int64) (int, error) {
	_, err := s.Queries.GetRoom(ctx, roomID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Error("room not found", "error", err)
			return http.StatusBadRequest, errors.New("room not found")
		}

		slog.Error("error checking if room exists", "error", err)
		return http.StatusInternalServerError, errors.New("error validating room ID")
	}

	return http.StatusOK, nil
}
