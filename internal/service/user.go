package service

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vhrboliveira/ama-go/internal/store/pgstore"
)

type UserService struct {
	q *pgstore.Queries
}

func NewUserService(q *pgstore.Queries) *UserService {
	return &UserService{
		q: q,
	}
}

func (u *UserService) GetUserByEmail(ctx context.Context, email string) (pgstore.User, error) {
	user, err := u.q.GetUserByEmail(ctx, email)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Error("user not found, creating user", "error", err)
			return pgstore.User{}, nil
		}

		slog.Error("error getting user", "error", err)
		return pgstore.User{}, errors.New("error getting user")
	}

	return user, nil

}

func (u *UserService) GetUserByID(ctx context.Context, id string) (pgstore.User, error) {
	userId, err := uuid.Parse(id)
	if err != nil {
		slog.Error("failed to parse user ID", "error", err)
		return pgstore.User{}, err
	}

	user, err := u.q.GetUserById(ctx, userId)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Error("user not found, creating user", "error", err)
			return pgstore.User{}, errors.New("no user found")
		}

		slog.Error("error getting user", "error", err)
		return pgstore.User{}, errors.New("error getting user")
	}

	return user, nil
}

func (u *UserService) CreateUser(ctx context.Context, user pgstore.User) (userID uuid.UUID, createdAt pgtype.Timestamp, updatedAt pgtype.Timestamp, err error) {
	dbUser := pgstore.CreateUserParams{
		Email:          user.Email,
		Name:           user.Name,
		Photo:          user.Photo,
		Provider:       user.Provider,
		ProviderUserID: user.ProviderUserID,
	}

	createUserRow, err := u.q.CreateUser(ctx, dbUser)
	if err != nil {
		slog.Error("error creating user", "error", err)
		return uuid.Nil, pgtype.Timestamp{}, pgtype.Timestamp{}, errors.New("error creating user")
	}

	userID = createUserRow.ID
	createdAt = createUserRow.CreatedAt
	updatedAt = createUserRow.UpdatedAt

	return userID, createdAt, updatedAt, err
}

func (u *UserService) UpdateUser(ctx context.Context, userID uuid.UUID, name string, enablePicture bool) (bool, pgtype.Timestamp, error) {
	params := pgstore.UpdateUserParams{
		ID:            userID,
		Name:          name,
		EnablePicture: enablePicture,
	}

	updatedUser, err := u.q.UpdateUser(ctx, params)

	return updatedUser.NewUser, updatedUser.UpdatedAt, err
}

func (u *UserService) DeleteUserInfo(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	return u.q.DeleteUser(ctx, userID)
}
