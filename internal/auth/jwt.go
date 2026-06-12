package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   int64  `json:"uid"`
	Username string `json:"un"`
	jwt.RegisteredClaims
}

func IssueToken(userID int64, username, secret string, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID: userID, Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func ParseToken(tokenStr, secret string) (int64, string, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("bad alg: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil { return 0, "", err }
	c, ok := tok.Claims.(*Claims)
	if !ok || !tok.Valid { return 0, "", errors.New("invalid claims") }
	return c.UserID, c.Username, nil
}
