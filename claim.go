package mochi

import (
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	Sub uint      `json:"sub"`
	Exp time.Time `json:"exp"`
	Iat time.Time `json:"iat"`
	Nbf time.Time `json:"nbf"`
	Aud string    `json:"aud"`
	Iss string    `json:"iss"`
}

const (
	TokenExpirationTime = time.Hour * 24
)

func NewClaims(user User, audience, issuer string) *Claims {
	now := time.Now()

	return &Claims{
		Sub: user.GetID(),
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
