// Package auth handles JWT token generation/validation, password hashing,
// and service account token generation.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Claims represents JWT claims for a user session.
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// ServiceAccountClaims represents claims for a service account token validation context.
type ServiceAccountClaims struct {
	ServiceAccountID string   `json:"sa_id"`
	ProjectID        string   `json:"project_id"`
	Scopes           []string `json:"scopes"`
}

// Auth handles authentication operations.
type Auth struct {
	jwtSecret     []byte
	tokenDuration time.Duration
}

// New creates a new Auth instance.
func New(jwtSecret string) *Auth {
	return &Auth{
		jwtSecret:     []byte(jwtSecret),
		tokenDuration: 1 * time.Hour,
	}
}

// HashPassword hashes a password using bcrypt.
func (a *Auth) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword verifies a password against a bcrypt hash.
func (a *Auth) CheckPassword(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// GenerateJWT creates a signed JWT token for a user.
func (a *Auth) GenerateJWT(userID, email, role string) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(a.tokenDuration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "teamvault",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.jwtSecret)
}

// ValidateJWT parses and validates a JWT token.
func (a *Auth) ValidateJWT(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.jwtSecret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parsing token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// GenerateServiceAccountToken generates a random token for a service account.
// Returns the raw token (to give to the user) and the bcrypt hash (to store in DB).
func (a *Auth) GenerateServiceAccountToken() (rawToken string, tokenHash string, err error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", "", fmt.Errorf("generating random token: %w", err)
	}

	rawToken = hex.EncodeToString(tokenBytes)

	hash, err := bcrypt.GenerateFromPassword([]byte(rawToken), bcrypt.DefaultCost)
	if err != nil {
		return "", "", fmt.Errorf("hashing token: %w", err)
	}

	return rawToken, string(hash), nil
}

// ValidateServiceAccountToken checks a raw token against a bcrypt hash.
func (a *Auth) ValidateServiceAccountToken(rawToken, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(rawToken))
}
