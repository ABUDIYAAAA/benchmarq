package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/ABUDIYAAAA/benchmarq/internal/config"
	"github.com/ABUDIYAAAA/benchmarq/internal/mailer"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/token"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrAccountInactive    = errors.New("account is deactivated")
	ErrInvalidDevice      = errors.New("invalid or mismatched device session")
)

type Service interface {
	SignUp(ctx context.Context, req *SignUpRequest, clientIP string, userAgent string, clientDeviceID string) (*AuthResponse, *TokenPair, error)
	Login(ctx context.Context, req *LoginRequest, clientIP string, userAgent string, clientDeviceID string) (*AuthResponse, *TokenPair, error)
	RefreshToken(ctx context.Context, rawRefreshToken string, clientDeviceID string, clientIP string, userAgent string) (*TokenPair, string, error)
	Logout(ctx context.Context, rawRefreshToken string) error
	ForgotPassword(ctx context.Context, req *ForgotPasswordRequest) error
	ResetPassword(ctx context.Context, req *ResetPasswordRequest) error
	GetMe(ctx context.Context, userID uuid.UUID) (*UserResponse, error)
}

type authService struct {
	repo   Repository
	cfg    *config.Config
	logger *slog.Logger
	mailer mailer.Mailer
}

func NewService(repo Repository, cfg *config.Config, logger *slog.Logger, m mailer.Mailer) Service {
	return &authService{
		repo:   repo,
		cfg:    cfg,
		logger: logger,
		mailer: m,
	}
}

func (s *authService) SignUp(ctx context.Context, req *SignUpRequest, clientIP string, userAgent string, clientDeviceID string) (*AuthResponse, *TokenPair, error) {
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to hash password: %w", err)
	}
	passStr := string(hashedPassword)

	user := &User{
		ID:              uuid.New(),
		Email:           req.Email,
		PasswordHash:    &passStr,
		FirstName:       req.FirstName,
		LastName:        &req.LastName,
		IsActive:        true,
		IsEmailVerified: false,
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, nil, err
	}

	// Claim any pending organization invitations for this email
	if err := s.repo.ClaimPendingInvitations(ctx, user.Email, user.ID); err != nil {
		s.logger.Warn("Failed linking pending invitations on signup", "email", user.Email, "error", err)
	}

	// Device ID resolution
	deviceID := clientDeviceID
	if deviceID == "" {
		deviceID = uuid.New().String()
	}

	// Issue Tokens and Session
	tokens, err := s.createSessionAndTokens(ctx, user, deviceID, clientIP, userAgent)
	if err != nil {
		return nil, nil, err
	}

	// Send Welcome / Verification email asynchronously
	if s.mailer != nil {
		verifyURL := fmt.Sprintf("%s/verify-email?email=%s", s.cfg.FrontendURL, user.Email)
		_ = s.mailer.SendWelcomeEmail(user.Email, user.FirstName, verifyURL)
	}

	return &AuthResponse{
		User:     ToUserResponse(user),
		DeviceID: deviceID,
	}, tokens, nil
}

func (s *authService) Login(ctx context.Context, req *LoginRequest, clientIP string, userAgent string, clientDeviceID string) (*AuthResponse, *TokenPair, error) {
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, nil, ErrInvalidCredentials
		}
		return nil, nil, err
	}

	if !user.IsActive {
		return nil, nil, ErrAccountInactive
	}

	if user.PasswordHash == nil || *user.PasswordHash == "" {
		return nil, nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	// Device ID resolution
	deviceID := clientDeviceID
	if deviceID == "" {
		deviceID = uuid.New().String()
	}

	// Issue Tokens and Session
	tokens, err := s.createSessionAndTokens(ctx, user, deviceID, clientIP, userAgent)
	if err != nil {
		return nil, nil, err
	}

	return &AuthResponse{
		User:     ToUserResponse(user),
		DeviceID: deviceID,
	}, tokens, nil
}

func (s *authService) RefreshToken(ctx context.Context, rawRefreshToken string, clientDeviceID string, clientIP string, userAgent string) (*TokenPair, string, error) {
	if rawRefreshToken == "" {
		return nil, "", token.ErrInvalidToken
	}

	tokenHash := token.HashToken(rawRefreshToken)
	session, err := s.repo.GetSessionByRefreshTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil, "", token.ErrInvalidToken
		}
		return nil, "", err
	}

	if session.IsRevoked || time.Now().UTC().After(session.ExpiresAt) {
		// Possible token reuse attack; revoke all sessions for this device
		if session.DeviceID != "" {
			_ = s.repo.RevokeDeviceSessions(ctx, session.UserID, session.DeviceID)
		}
		return nil, "", token.ErrTokenExpired
	}

	// Check device consistency if device ID provided
	if clientDeviceID != "" && session.DeviceID != clientDeviceID {
		_ = s.repo.RevokeSession(ctx, session.ID)
		return nil, "", ErrInvalidDevice
	}

	user, err := s.repo.GetUserByID(ctx, session.UserID)
	if err != nil || !user.IsActive {
		_ = s.repo.RevokeSession(ctx, session.ID)
		return nil, "", ErrAccountInactive
	}

	// Rotate refresh token
	newRawRefresh, newHashedRefresh, err := token.GenerateRefreshToken()
	if err != nil {
		return nil, "", fmt.Errorf("failed generating rotated refresh token: %w", err)
	}

	newExpiresAt := time.Now().UTC().Add(time.Duration(s.cfg.JWTRefreshExpiryDays) * 24 * time.Hour)
	if err := s.repo.UpdateSessionRefreshToken(ctx, session.ID, newHashedRefresh, newExpiresAt); err != nil {
		return nil, "", fmt.Errorf("failed updating rotated session: %w", err)
	}

	// Issue new Access Token
	newAccess, err := token.GenerateAccessToken(
		user.ID,
		user.Email,
		session.DeviceID,
		s.cfg.JWTSecret,
		s.cfg.JWTAccessExpiryMinutes,
		s.cfg.JWTIssuer,
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed generating access token: %w", err)
	}

	return &TokenPair{
		AccessToken:  newAccess,
		RefreshToken: newRawRefresh,
	}, session.DeviceID, nil
}

func (s *authService) Logout(ctx context.Context, rawRefreshToken string) error {
	if rawRefreshToken == "" {
		return nil
	}

	tokenHash := token.HashToken(rawRefreshToken)
	session, err := s.repo.GetSessionByRefreshTokenHash(ctx, tokenHash)
	if err != nil {
		return nil
	}

	return s.repo.RevokeSession(ctx, session.ID)
}

func (s *authService) ForgotPassword(ctx context.Context, req *ForgotPasswordRequest) error {
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		// Do not leak email existence; return success silently
		s.logger.Info("Password reset requested for non-existing or inactive email", "email", req.Email)
		return nil
	}

	rawToken, tokenHash, err := token.GeneratePasswordResetToken()
	if err != nil {
		return fmt.Errorf("failed generating reset token: %w", err)
	}

	expiryMinutes := s.cfg.ResetPasswordExpiryMinutes
	if expiryMinutes <= 0 {
		expiryMinutes = 15
	}

	resetRecord := &PasswordResetToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: tokenHash,
		IsUsed:    false,
		ExpiresAt: time.Now().UTC().Add(time.Duration(expiryMinutes) * time.Minute),
	}

	if err := s.repo.CreatePasswordResetToken(ctx, resetRecord); err != nil {
		return err
	}

	if s.mailer != nil {
		resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.cfg.FrontendURL, rawToken)
		_ = s.mailer.SendPasswordResetEmail(user.Email, user.FirstName, resetURL, expiryMinutes)
	}

	return nil
}

func (s *authService) ResetPassword(ctx context.Context, req *ResetPasswordRequest) error {
	tokenHash := token.HashToken(req.Token)
	resetRecord, err := s.repo.GetPasswordResetTokenByHash(ctx, tokenHash)
	if err != nil {
		return err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed hashing password: %w", err)
	}

	if err := s.repo.UpdateUserPassword(ctx, resetRecord.UserID, string(hashedPassword)); err != nil {
		return err
	}

	if err := s.repo.MarkPasswordResetTokenUsed(ctx, resetRecord.ID); err != nil {
		s.logger.Warn("Failed marking reset token as used", "token_id", resetRecord.ID, "error", err)
	}

	// Revoke all existing sessions for this user on all devices to prevent unauthorized sessions
	_ = s.repo.RevokeUserSessions(ctx, resetRecord.UserID)

	return nil
}

func (s *authService) GetMe(ctx context.Context, userID uuid.UUID) (*UserResponse, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return ToUserResponse(user), nil
}

func (s *authService) createSessionAndTokens(ctx context.Context, user *User, deviceID string, clientIP string, userAgent string) (*TokenPair, error) {
	// Generate Refresh Token (raw for cookie, hashed for DB)
	rawRefresh, hashedRefresh, err := token.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed generating refresh token: %w", err)
	}

	// Generate Access Token (JWT)
	accessToken, err := token.GenerateAccessToken(
		user.ID,
		user.Email,
		deviceID,
		s.cfg.JWTSecret,
		s.cfg.JWTAccessExpiryMinutes,
		s.cfg.JWTIssuer,
	)
	if err != nil {
		return nil, fmt.Errorf("failed generating access token: %w", err)
	}

	// Store Session in Database
	var ipPtr *string
	if clientIP != "" {
		ipPtr = &clientIP
	}
	var agentPtr *string
	if userAgent != "" {
		agentPtr = &userAgent
	}

	expiresAt := time.Now().UTC().Add(time.Duration(s.cfg.JWTRefreshExpiryDays) * 24 * time.Hour)
	session := &UserSession{
		ID:               uuid.New(),
		UserID:           user.ID,
		DeviceID:         deviceID,
		RefreshTokenHash: hashedRefresh,
		UserAgent:        agentPtr,
		IPAddress:        ipPtr,
		IsRevoked:        false,
		ExpiresAt:        expiresAt,
	}

	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed creating session record: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
	}, nil
}
