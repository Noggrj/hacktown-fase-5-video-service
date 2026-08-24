// Package jwt issues and verifies the HS256 tokens shared across every
// fiapx-* service. The signing secret is distributed via a Kubernetes
// Secret (JWT_SECRET) identically to every consumer — only auth-service
// holds an Issuer, other services only ever construct a Verifier.
package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid or expired token")

// Claims carried inside the token. UserID mirrors RegisteredClaims.Subject
// for callers that only look at the typed struct.
type Claims struct {
	UserID string `json:"sub"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// Issuer signs new tokens. Only auth-service constructs one.
type Issuer struct {
	secret []byte
}

func NewIssuer(secret string) (*Issuer, error) {
	if len(secret) < 16 {
		return nil, errors.New("jwt secret must be at least 16 bytes")
	}
	return &Issuer{secret: []byte(secret)}, nil
}

func (i *Issuer) Issue(userID, email string, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Issuer:    "fiapx-auth-service",
			Subject:   userID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(i.secret)
}

// Verifier validates tokens issued by Issuer. Every fiapx-* service
// (including auth-service itself, for its own /me endpoint) constructs
// one from the same JWT_SECRET to protect its routes.
type Verifier struct {
	secret []byte
}

func NewVerifier(secret string) (*Verifier, error) {
	if len(secret) < 16 {
		return nil, errors.New("jwt secret must be at least 16 bytes")
	}
	return &Verifier{secret: []byte(secret)}, nil
}

func (v *Verifier) Verify(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return v.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
