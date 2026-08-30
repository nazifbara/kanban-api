package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func MakeRefreshToken() string {
	token := make([]byte, 32)
	rand.Read(token)
	return hex.EncodeToString(token)
}

func GetAPIKey(header http.Header) (string, error) {
	authorization := header.Get("Authorization")
	if authorization == "" {
		return "", fmt.Errorf("authorization header not set")
	}
	items := strings.Split(authorization, " ")
	if len(items) != 2 || items[0] != "ApiKey" {
		return "", fmt.Errorf("invalid ApiKey format")
	}
	return items[1], nil
}

func GetBearerToken(header http.Header) (string, error) {
	authorization := header.Get("Authorization")
	if authorization == "" {
		return "", fmt.Errorf("authorization header not set")
	}
	items := strings.Split(authorization, " ")
	if len(items) != 2 || items[0] != "Bearer" {
		return "", fmt.Errorf("invalid Bearer format")
	}
	return items[1], nil
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expireIn time.Duration) (string, error) {
	return jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		jwt.RegisteredClaims{
			Issuer:    "chirpy-access",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expireIn)),
			Subject:   userID.String(),
		},
	).SignedString([]byte(tokenSecret))
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != "HS256" {
			return uuid.Nil, fmt.Errorf("unexpected signing method: %v", t.Method.Alg())
		}
		return []byte(tokenSecret), nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok {
		return uuid.Nil, fmt.Errorf("invalid token")
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}

func HashPassword(password string) (string, error) {
	return argon2id.CreateHash(password, argon2id.DefaultParams)
}

func CheckPasswordHash(password, hash string) (bool, error) {
	return argon2id.ComparePasswordAndHash(password, hash)
}
