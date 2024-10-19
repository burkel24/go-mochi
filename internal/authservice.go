package internal

import (
	"context"
	"net/http"
)

type AuthService interface {
	AuthRequired() func(http.Handler) http.Handler
	AdminRequired() func(http.Handler) http.Handler
	GetUserFromCtx(ctx context.Context) (User, error)
	LoginUser(ctx context.Context, username, password string) (string, error)
}
