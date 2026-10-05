package auth

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ABUDIYAAAA/benchmarq/internal/config"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/cookies"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/logger"
	"github.com/google/uuid"
)

type mockRepository struct {
	users       map[string]*User
	usersByID   map[uuid.UUID]*User
	sessions    map[string]*UserSession
	resetTokens map[string]*PasswordResetToken
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		users:       make(map[string]*User),
		usersByID:   make(map[uuid.UUID]*User),
		sessions:    make(map[string]*UserSession),
		resetTokens: make(map[string]*PasswordResetToken),
	}
}

func (m *mockRepository) CreateUser(ctx context.Context, user *User) error {
	if _, exists := m.users[user.Email]; exists {
		return ErrUserAlreadyExists
	}
	m.users[user.Email] = user
	m.usersByID[user.ID] = user
	return nil
}

func (m *mockRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	user, exists := m.users[email]
	if !exists {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (m *mockRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	user, exists := m.usersByID[id]
	if !exists {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (m *mockRepository) UpdateUserPassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	user, exists := m.usersByID[userID]
	if !exists {
		return ErrUserNotFound
	}
	user.PasswordHash = &passwordHash
	return nil
}

func (m *mockRepository) CreateSession(ctx context.Context, session *UserSession) error {
	m.sessions[session.RefreshTokenHash] = session
	return nil
}

func (m *mockRepository) GetSessionByRefreshTokenHash(ctx context.Context, tokenHash string) (*UserSession, error) {
	session, exists := m.sessions[tokenHash]
	if !exists {
		return nil, ErrSessionNotFound
	}
	return session, nil
}

func (m *mockRepository) UpdateSessionRefreshToken(ctx context.Context, sessionID uuid.UUID, newRefreshTokenHash string, newExpiresAt time.Time) error {
	for oldHash, sess := range m.sessions {
		if sess.ID == sessionID {
			delete(m.sessions, oldHash)
			sess.RefreshTokenHash = newRefreshTokenHash
			sess.ExpiresAt = newExpiresAt
			m.sessions[newRefreshTokenHash] = sess
			return nil
		}
	}
	return ErrSessionNotFound
}

func (m *mockRepository) RevokeSession(ctx context.Context, sessionID uuid.UUID) error {
	for _, sess := range m.sessions {
		if sess.ID == sessionID {
			sess.IsRevoked = true
			return nil
		}
	}
	return nil
}

func (m *mockRepository) RevokeUserSessions(ctx context.Context, userID uuid.UUID) error {
	for _, sess := range m.sessions {
		if sess.UserID == userID {
			sess.IsRevoked = true
		}
	}
	return nil
}

func (m *mockRepository) RevokeDeviceSessions(ctx context.Context, userID uuid.UUID, deviceID string) error {
	for _, sess := range m.sessions {
		if sess.UserID == userID && sess.DeviceID == deviceID {
			sess.IsRevoked = true
		}
	}
	return nil
}

func (m *mockRepository) CreatePasswordResetToken(ctx context.Context, token *PasswordResetToken) error {
	m.resetTokens[token.TokenHash] = token
	return nil
}

func (m *mockRepository) GetPasswordResetTokenByHash(ctx context.Context, tokenHash string) (*PasswordResetToken, error) {
	t, exists := m.resetTokens[tokenHash]
	if !exists {
		return nil, ErrResetTokenNotFound
	}
	if t.IsUsed {
		return nil, ErrResetTokenAlreadyUsed
	}
	if time.Now().UTC().After(t.ExpiresAt) {
		return nil, ErrResetTokenExpired
	}
	return t, nil
}

func (m *mockRepository) MarkPasswordResetTokenUsed(ctx context.Context, tokenID uuid.UUID) error {
	for _, t := range m.resetTokens {
		if t.ID == tokenID {
			t.IsUsed = true
			return nil
		}
	}
	return nil
}

func (m *mockRepository) ClaimPendingInvitations(ctx context.Context, email string, userID uuid.UUID) error {
	return nil
}

func TestAuthWorkflow(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:                  "test_secret_for_auth_12345",
		JWTAccessExpiryMinutes:     15,
		JWTRefreshExpiryDays:       7,
		JWTIssuer:                  "benchmarq",
		CookieHTTPOnly:             true,
		CookieSecure:               false,
		CookieSameSite:             "lax",
		CookiePath:                 "/",
		ResetPasswordExpiryMinutes: 15,
		FrontendURL:                "http://localhost:3000",
	}
	log := logger.NewInfoLogger()
	repo := newMockRepository()
	service := NewService(repo, cfg, log, nil)

	ctx := context.Background()

	// 1. Test Sign Up
	signUpReq := &SignUpRequest{
		Email:     "developer@benchmarq.io",
		Password:  "SecurePassword123!",
		FirstName: "Jane",
		LastName:  "Doe",
	}
	authResp, tokens, err := service.SignUp(ctx, signUpReq, "127.0.0.1", "TestAgent", "")
	if err != nil {
		t.Fatalf("SignUp failed: %v", err)
	}
	if authResp.User.Email != "developer@benchmarq.io" {
		t.Errorf("expected email developer@benchmarq.io, got %s", authResp.User.Email)
	}
	if authResp.DeviceID == "" {
		t.Errorf("expected generated deviceID, got empty")
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Errorf("expected access and refresh tokens, got empty")
	}

	// 2. Test Login
	loginReq := &LoginRequest{
		Email:    "developer@benchmarq.io",
		Password: "SecurePassword123!",
	}
	loginResp, loginTokens, err := service.Login(ctx, loginReq, "127.0.0.1", "TestAgent", authResp.DeviceID)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if loginResp.DeviceID != authResp.DeviceID {
		t.Errorf("expected device ID %s, got %s", authResp.DeviceID, loginResp.DeviceID)
	}

	// 3. Test Refresh Token Rotation
	rotTokens, devID, err := service.RefreshToken(ctx, loginTokens.RefreshToken, loginResp.DeviceID, "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("RefreshToken failed: %v", err)
	}
	if devID != loginResp.DeviceID {
		t.Errorf("expected device ID %s, got %s", loginResp.DeviceID, devID)
	}
	if rotTokens.AccessToken == "" || rotTokens.RefreshToken == "" {
		t.Errorf("expected rotated tokens, got empty")
	}

	// Old refresh token must now be invalid
	_, _, err = service.RefreshToken(ctx, loginTokens.RefreshToken, loginResp.DeviceID, "127.0.0.1", "TestAgent")
	if err == nil {
		t.Fatalf("expected old refresh token to be invalid, but succeeded")
	}

	// 4. Test Forgot Password
	forgotReq := &ForgotPasswordRequest{Email: "developer@benchmarq.io"}
	err = service.ForgotPassword(ctx, forgotReq)
	if err != nil {
		t.Fatalf("ForgotPassword failed: %v", err)
	}

	// 5. Test Logout
	err = service.Logout(ctx, rotTokens.RefreshToken)
	if err != nil {
		t.Fatalf("Logout failed: %v", err)
	}
}

func TestAuthHandlersHTTP(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:                  "test_secret_for_auth_12345",
		JWTAccessExpiryMinutes:     15,
		JWTRefreshExpiryDays:       7,
		JWTIssuer:                  "benchmarq",
		CookieHTTPOnly:             true,
		CookieSecure:               false,
		CookieSameSite:             "lax",
		CookiePath:                 "/",
		ResetPasswordExpiryMinutes: 15,
		FrontendURL:                "http://localhost:3000",
	}
	log := logger.NewInfoLogger()
	repo := newMockRepository()
	service := NewService(repo, cfg, log, nil)
	handler := NewHandler(service, cfg)

	// Test Signup HTTP
	body := `{"email": "api_user@benchmarq.io", "password": "Password123!", "first_name": "API", "last_name": "User"}`
	req := httptest.NewRequest("POST", "/auth/signup", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.SignUpHandler(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}

	// Check that Cookies were set
	cookiesList := resp.Cookies()
	var hasAccess, hasRefresh, hasDevice bool
	for _, c := range cookiesList {
		if c.Name == cookies.AccessTokenCookieName {
			hasAccess = true
		}
		if c.Name == cookies.RefreshTokenCookieName {
			hasRefresh = true
		}
		if c.Name == cookies.DeviceIDCookieName {
			hasDevice = true
		}
	}
	if !hasAccess || !hasRefresh || !hasDevice {
		t.Errorf("expected auth cookies to be set, got hasAccess=%v, hasRefresh=%v, hasDevice=%v", hasAccess, hasRefresh, hasDevice)
	}
}
