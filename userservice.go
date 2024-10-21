package mochi

import (
	"context"
)

type User interface {
	Admin() bool
	ID() uint
}

type UserService interface {
	ListUsers(ctx context.Context) ([]User, error)
	CreateUser(ctx context.Context, user User) (User, error)
	GetUserByID(ctx context.Context, userID uint) (User, error)
	GetUserByCredentials(ctx context.Context, username, passwordHash string) (User, error)
	UpdateUserPassword(ctx context.Context, userID uint, password string) error
}
