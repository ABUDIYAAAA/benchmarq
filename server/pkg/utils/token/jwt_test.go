package token

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGenerateAndValidateAccessToken(t *testing.T) {
	secret := "test_secret_key_1234567890"
	userID := uuid.New()
	email := "test@benchmarq.io"
	deviceID := "device-uuid-1234"

	tokenStr, err := GenerateAccessToken(userID, email, deviceID, secret, 15, "benchmarq")
	if err != nil {
		t.Fatalf("failed generating token: %v", err)
	}

	claims, err := ValidateAccessToken(tokenStr, secret)
	if err != nil {
		t.Fatalf("failed validating token: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("expected UserID %v, got %v", userID, claims.UserID)
	}
	if claims.Email != email {
		t.Errorf("expected email %v, got %v", email, claims.Email)
	}
	if claims.DeviceID != deviceID {
		t.Errorf("expected deviceID %v, got %v", deviceID, claims.DeviceID)
	}
}

func TestRefreshTokenGenerationAndHashing(t *testing.T) {
	raw, hashed, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("failed generating refresh token: %v", err)
	}

	if len(raw) != 64 {
		t.Errorf("expected raw token length of 64, got %d", len(raw))
	}

	expectedHash := HashToken(raw)
	if hashed != expectedHash {
		t.Errorf("expected hash %s, got %s", expectedHash, hashed)
	}
}

func TestExpiredTokenValidation(t *testing.T) {
	secret := "test_secret_key_1234567890"
	userID := uuid.New()

	// 0 minute / expired token
	tokenStr, err := GenerateAccessToken(userID, "expired@test.com", "dev1", secret, -1, "benchmarq")
	if err != nil {
		t.Fatalf("failed generating token: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	_, err = ValidateAccessToken(tokenStr, secret)
	if err == nil {
		t.Fatalf("expected error for expired token, got nil")
	}
}
