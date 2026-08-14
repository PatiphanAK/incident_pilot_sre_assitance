// Package auth provides stateless JWT authentication (HS256) for the API.
// It has no persistence of its own, so it is a plain leaf package rather than
// a full hexagonal module. The future order module reuses it as-is.
package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenTTL is how long a signed token stays valid.
const TokenTTL = 24 * time.Hour

// Claims is the JWT payload. Subject holds the user id.
type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// TokenService signs and verifies HS256 tokens with a shared secret.
type TokenService struct {
	secret []byte
}

func NewTokenService(secret string) *TokenService {
	return &TokenService{secret: []byte(secret)}
}

// Sign returns a token for the given user, valid for TokenTTL.
func (s *TokenService) Sign(userID, username string) (string, error) {
	now := time.Now()
	claims := Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(TokenTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

// Verify parses and validates a token string, rejecting anything that is not
// HS256-signed by our secret with an unexpired exp claim.
func (s *TokenService) Verify(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		return s.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}
