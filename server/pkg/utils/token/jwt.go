package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
	ErrTokenExpired = errors.New("token has expired")
)

// Claims represents JWT payload claims.
type Claims struct {
	UserID   uuid.UUID `json:"user_id"`
	Email    string    `json:"email"`
	DeviceID string    `json:"device_id"`
	TokenID  uuid.UUID `json:"token_id"`
	jwt.RegisteredClaims
}

// GenerateAccessToken signs a short-lived access JWT.
func GenerateAccessToken(userID uuid.UUID, email string, deviceID string, secret string, expiryMinutes int, issuer string) (string, error) {
	tokenID := uuid.New()
	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(expiryMinutes) * time.Minute)

	claims := Claims{
		UserID:   userID,
		Email:    email,
		DeviceID: deviceID,
		TokenID:  tokenID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        tokenID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign access token: %w", err)
	}

	return tokenStr, nil
}

// ValidateAccessToken parses and validates a signed access JWT string.
func ValidateAccessToken(tokenStr string, secret string) (*Claims, error) {
	parsedToken, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	claims, ok := parsedToken.Claims.(*Claims)
	if !ok || !parsedToken.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// GenerateRandomHexToken generates a cryptographically secure random hex string of given byte length.
func GenerateRandomHexToken(byteLength int) (string, error) {
	bytes := make([]byte, byteLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed generating random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// HashToken computes a SHA-256 hash of a raw token string (hex format) for secure database storage.
func HashToken(rawToken string) string {
	hash := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(hash[:])
}

// GenerateRefreshToken returns (rawRefreshToken, hashedRefreshToken, error).
func GenerateRefreshToken() (string, string, error) {
	raw, err := GenerateRandomHexToken(32) // 64 hex characters (256 bits entropy)
	if err != nil {
		return "", "", err
	}
	hashed := HashToken(raw)
	return raw, hashed, nil
}

// GeneratePasswordResetToken returns (rawResetToken, hashedResetToken, error).
func GeneratePasswordResetToken() (string, string, error) {
	raw, err := GenerateRandomHexToken(32)
	if err != nil {
		return "", "", err
	}
	hashed := HashToken(raw)
	return raw, hashed, nil
}
