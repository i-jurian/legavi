package auth

import (
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	jose "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWT struct {
	signKey []byte
	ttl     time.Duration
}

func NewJWT(signKey string, ttl time.Duration) (*JWT, error) {
	decoded, err := base64.StdEncoding.DecodeString(signKey)
	if err != nil {
		return nil, fmt.Errorf("signing key decoding error: %w", err)
	}
	if len(decoded) < 32 {
		return nil, fmt.Errorf("signing key must decode to at least 32 bytes")
	}
	return &JWT{signKey: decoded, ttl: ttl}, nil
}

func (j *JWT) Issue(userID uuid.UUID) (string, error) {
	claims := jose.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jose.NewNumericDate(time.Now()),
		ExpiresAt: jose.NewNumericDate(time.Now().Add(j.ttl)),
	}
	token := jose.NewWithClaims(jose.SigningMethodHS256, claims)
	return token.SignedString(j.signKey)
}

func (j *JWT) Parse(tokenString string) (uuid.UUID, error) {
	var claims jose.RegisteredClaims
	keyFunc := func(token *jose.Token) (any, error) {
		if token.Method != jose.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return j.signKey, nil
	}
	if _, err := jose.ParseWithClaims(tokenString, &claims, keyFunc); err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(claims.Subject)
}
