package mochi

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/burkel24/go-mochi/internal"
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

type AuthServiceParams struct {
	fx.In

	Logger      internal.LoggerService
	UserService internal.UserService
}

type AuthServiceResult struct {
	fx.Out

	AuthService internal.AuthService
}

type AuthService struct {
	logger        internal.LoggerService
	signingSecret string
	userService   internal.UserService
}

func NewAuthService(params AuthServiceParams) (AuthServiceResult, error) {
	var result AuthServiceResult

	signingSecret := os.Getenv("JWT_SIGNING_SECRET")

	result.AuthService = &AuthService{
		logger:        params.Logger,
		signingSecret: signingSecret,
		userService:   params.UserService,
	}

	return result, nil
}

func (svc *AuthService) AuthRequired() func(http.Handler) http.Handler {
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

func (svc *AuthService) AdminRequired() func(http.Handler) http.Handler {
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

func (svc *AuthService) GetUserFromCtx(ctx context.Context) (internal.User, error) {
	user, ok := ctx.Value(userContextKey).(internal.User)
	if !ok {
		return nil, fmt.Errorf("could not get user from context")
	}

	return user, nil
}

func (svc *AuthService) LoginUser(ctx context.Context, username, password string) (string, error) {
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

func (svc *AuthService) generateUserToken(user internal.User) (string, error) {
	claims := internal.NewClaims(user, "TODO", "TODO")

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(svc.signingSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

func (svc *AuthService) validateUserToken(tokenString string) (*internal.Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&internal.Claims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(svc.signingSecret), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*internal.Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

func (svc *AuthService) getTokenStringFromAuthHeader(r *http.Request) (string, error) {
	authHeader := r.Header.Get(AuthHeaderName)

	if authHeader == "" {
		return "", fmt.Errorf("missing auth header")
	}

	return authHeader[len("Bearer "):], nil
}
