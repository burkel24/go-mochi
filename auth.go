package mochi

import (
	"context"
	"fmt"
	"github.com/burkel24/go-mochi/interfaces"
	"github.com/go-chi/render"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/fx"
	"net/http"
	"os"
	"strconv"
	"time"
)

type authContextkey int

const (
	AuthHeaderName      = "Authorization"
	TokenExpirationTime = time.Hour * 24
)

const (
	userContextKey authContextkey = iota
)

type Claims struct {
	Sub uint      `json:"sub"`
	Exp time.Time `json:"exp"`
	Iat time.Time `json:"iat"`
	Nbf time.Time `json:"nbf"`
	Aud string    `json:"aud"`
	Iss string    `json:"iss"`
}

func NewClaims(user interfaces.User, audience, issuer string) *Claims {
	now := time.Now()

	return &Claims{
		Sub: user.ID(),
		Exp: now.Add(TokenExpirationTime),
		Iat: now,
		Nbf: now,
		Aud: audience,
		Iss: issuer,
	}
}

func (c *Claims) GetExpirationTime() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(c.Exp), nil
}

func (c *Claims) GetIssuedAt() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(c.Iat), nil
}

func (c *Claims) GetNotBefore() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(c.Nbf), nil
}

func (c *Claims) GetIssuer() (string, error) {
	return c.Iss, nil
}

func (c *Claims) GetSubject() (string, error) {
	return strconv.FormatUint(uint64(c.Sub), 10), nil
}

func (c *Claims) GetAudience() (jwt.ClaimStrings, error) {
	return []string{c.Aud}, nil
}

type AuthServiceParams struct {
	fx.In

	Logger      LoggerService
	UserService interfaces.UserService
}

type AuthServiceResult struct {
	fx.Out

	AuthService interfaces.AuthService
}

type AuthService struct {
	logger        LoggerService
	signingSecret string
	userService   interfaces.UserService
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

func (svc *AuthService) GetUserFromCtx(ctx context.Context) (interfaces.User, error) {
	user, ok := ctx.Value(userContextKey).(interfaces.User)
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

func (svc *AuthService) generateUserToken(user interfaces.User) (string, error) {
	claims := NewClaims(user, "TODO", "TODO")

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(svc.signingSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

func (svc *AuthService) validateUserToken(tokenString string) (*Claims, error) {
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

func (svc *AuthService) getTokenStringFromAuthHeader(r *http.Request) (string, error) {
	authHeader := r.Header.Get(AuthHeaderName)

	if authHeader == "" {
		return "", fmt.Errorf("missing auth header")
	}

	return authHeader[len("Bearer "):], nil
}
