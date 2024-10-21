package mochi

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/go-chi/render"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/fx"
)

type authContextkey int

const (
	AuthHeaderName = "Authorization"
)

const (
	userContextKey authContextkey = iota
)

type AuthService interface {
	AuthRequired() func(http.Handler) http.Handler
	AdminRequired() func(http.Handler) http.Handler
	GetUserFromCtx(ctx context.Context) (User, error)
	LoginUser(ctx context.Context, username, password string) (string, error)
}

type AuthServiceParams struct {
	fx.In

	Logger      LoggerService
	UserService UserService
}

type AuthServiceResult struct {
	fx.Out

	AuthService AuthService
}

type authService struct {
	logger        LoggerService
	signingSecret string
	userService   UserService
}

func NewAuthService(params AuthServiceParams) (AuthServiceResult, error) {
	var result AuthServiceResult

	signingSecret := os.Getenv("JWT_SIGNING_SECRET")

	result.AuthService = &authService{
		logger:        params.Logger,
		signingSecret: signingSecret,
		userService:   params.UserService,
	}

	return result, nil
}

func (svc *authService) AuthRequired() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString, err := svc.getTokenStringFromAuthHeader(r)
			if err != nil {
				render.Render(w, r, render.Renderer(ErrUnauthorized(err)))
				return
			}

			claims, err := svc.validateUserToken(tokenString)
			if err != nil {
				render.Render(w, r, render.Renderer(ErrUnauthorized(err)))
				return
			}

			user, err := svc.userService.GetUserByID(r.Context(), claims.Sub)
			if err != nil {
				render.Render(w, r, render.Renderer(ErrUnauthorized(err)))
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (svc *authService) AdminRequired() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			user, err := svc.GetUserFromCtx(ctx)
			if err != nil {
				render.Render(w, r, ErrUnauthorized(err))
				return
			}

			if !user.Admin() {
				render.Render(w, r, ErrUnauthorized(fmt.Errorf("user is not an admin")))
				return
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (svc *authService) GetUserFromCtx(ctx context.Context) (User, error) {
	user, ok := ctx.Value(userContextKey).(User)
	if !ok {
		return nil, fmt.Errorf("could not get user from context")
	}

	return user, nil
}

func (svc *authService) LoginUser(ctx context.Context, username, password string) (string, error) {
	user, err := svc.userService.GetUserByCredentials(ctx, username, password)
	if err != nil {
		return "", fmt.Errorf("failed to get user by credentials: %w", err)
	}

	token, err := svc.generateUserToken(user)
	if err != nil {
		return "", fmt.Errorf("failed to generate user token: %w", err)
	}

	return token, nil
}

func (svc *authService) generateUserToken(user User) (string, error) {
	claims := NewClaims(user, "TODO", "TODO")

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(svc.signingSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

func (svc *authService) validateUserToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(svc.signingSecret), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

func (svc *authService) getTokenStringFromAuthHeader(r *http.Request) (string, error) {
	authHeader := r.Header.Get(AuthHeaderName)

	if authHeader == "" {
		return "", fmt.Errorf("missing auth header")
	}

	return authHeader[len("Bearer "):], nil
}
