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
	ErrInvalidCredentials      = errors.New("invalid email or password")
	ErrAccountInactive         = errors.New("account is deactivated")
	ErrInvalidDevice           = errors.New("invalid or mismatched device session")
	ErrEmailAlreadyVerified    = errors.New("email is already verified")
	ErrVerificationRateLimited = errors.New("a verification email was sent recently")
)

// verificationResendCooldown is the minimum gap between two verification
// emails to the same user.
const verificationResendCooldown = time.Minute

type Service interface {
	SignUp(ctx context.Context, req *SignUpRequest, clientIP string, userAgent string, clientDeviceID string) (*AuthResponse, *TokenPair, error)
	Login(ctx context.Context, req *LoginRequest, clientIP string, userAgent string, clientDeviceID string) (*AuthResponse, *TokenPair, error)
	RefreshToken(ctx context.Context, rawRefreshToken string, clientDeviceID string, clientIP string, userAgent string) (*TokenPair, string, error)
	Logout(ctx context.Context, rawRefreshToken string) error
	ForgotPassword(ctx context.Context, req *ForgotPasswordRequest) error
	ResetPassword(ctx context.Context, req *ResetPasswordRequest) error
	VerifyEmail(ctx context.Context, req *VerifyEmailRequest) error
	ResendVerification(ctx context.Context, userID uuid.UUID) error
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

	// Send Welcome / Verification email asynchronously. Pending organization
	// invitations are claimed once the address is verified, not here, so an
	// unverified signup cannot join an organization by naming its email.
	verifyURL, err := s.issueVerificationURL(ctx, user)
	if err != nil {
		s.logger.Warn("Failed issuing email verification token on signup", "user_id", user.ID, "error", err)
	} else if s.mailer != nil {
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
	// Invitations sent to this address while the user already had an account
	// are joined on login. Only verified addresses claim invitations, as on
	// email verification.
	if user.IsEmailVerified {
		if err := s.repo.ClaimPendingInvitations(ctx, user.Email, user.ID); err != nil {
			s.logger.Warn("Failed linking pending invitations on login", "user_id", user.ID, "error", err)
		}
	}

	tokens, err := s.createSessionAndTokens(ctx, user, deviceID, clientIP, userAgent)
	if err != nil {
		return nil, nil, err
	}

	return &AuthResponse{
		User:     ToUserResponse(user),
		DeviceID: deviceID,
	}, tokens, nil
}

// refreshReuseInterval is how long a just-rotated refresh token is still
// honoured. Two tabs refreshing at once, or a retry after a lost response,
// present the previous token moments after rotation; that is a race, not
// theft. Outside the window, a previous token means it was stolen and
// replayed, and the device's sessions are revoked.
const refreshReuseInterval = 30 * time.Second

// maxRotationAttempts bounds retries when concurrent refreshes race on the
// same session.
const maxRotationAttempts = 3

func (s *authService) RefreshToken(ctx context.Context, rawRefreshToken string, clientDeviceID string, clientIP string, userAgent string) (*TokenPair, string, error) {
	if rawRefreshToken == "" {
		return nil, "", token.ErrInvalidToken
	}
	tokenHash := token.HashToken(rawRefreshToken)

	for attempt := 1; ; attempt++ {
		session, err := s.repo.GetSessionByRefreshTokenHash(ctx, tokenHash)
		if err != nil {
			if errors.Is(err, ErrSessionNotFound) {
				return nil, "", token.ErrInvalidToken
			}
			return nil, "", err
		}

		// 1. A previous token is only acceptable inside the reuse window.
		if session.PreviousRefreshTokenHash != nil && *session.PreviousRefreshTokenHash == tokenHash {
			if session.RotatedAt == nil || time.Since(*session.RotatedAt) > refreshReuseInterval {
				s.logger.Warn("Refresh token replayed after rotation; revoking device sessions",
					"session_id", session.ID, "user_id", session.UserID, "device_id", session.DeviceID)
				s.revokeDevice(ctx, session)
				return nil, "", token.ErrInvalidToken
			}
		}

		// 2. Revoked or expired
		if session.IsRevoked || time.Now().UTC().After(session.ExpiresAt) {
			s.revokeDevice(ctx, session)
			return nil, "", token.ErrTokenExpired
		}

		// 3. Device consistency
		if clientDeviceID != "" && session.DeviceID != clientDeviceID {
			_ = s.repo.RevokeSession(ctx, session.ID)
			return nil, "", ErrInvalidDevice
		}

		user, err := s.repo.GetUserByID(ctx, session.UserID)
		if err != nil || !user.IsActive {
			_ = s.repo.RevokeSession(ctx, session.ID)
			return nil, "", ErrAccountInactive
		}

		// 4. Rotate from the session's current token (compare-and-swap). If
		// another request rotated in between, re-read and try again.
		newRawRefresh, newHashedRefresh, err := token.GenerateRefreshToken()
		if err != nil {
			return nil, "", fmt.Errorf("failed generating rotated refresh token: %w", err)
		}
		newExpiresAt := time.Now().UTC().Add(time.Duration(s.cfg.JWTRefreshExpiryDays) * 24 * time.Hour)
		err = s.repo.RotateSessionRefreshToken(ctx, session.ID, session.RefreshTokenHash, newHashedRefresh, newExpiresAt)
		if errors.Is(err, ErrSessionRotated) && attempt < maxRotationAttempts {
			continue
		}
		if err != nil {
			if errors.Is(err, ErrSessionRotated) {
				return nil, "", token.ErrInvalidToken
			}
			return nil, "", fmt.Errorf("failed updating rotated session: %w", err)
		}

		// 5. Issue the new access token
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
}

// revokeDevice ends every session on the session's device (or just this
// session when no device is recorded).
func (s *authService) revokeDevice(ctx context.Context, session *UserSession) {
	if session.DeviceID != "" {
		_ = s.repo.RevokeDeviceSessions(ctx, session.UserID, session.DeviceID)
		return
	}
	_ = s.repo.RevokeSession(ctx, session.ID)
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

func (s *authService) VerifyEmail(ctx context.Context, req *VerifyEmailRequest) error {
	userID, err := s.repo.ConsumeEmailVerificationToken(ctx, token.HashToken(req.Token))
	if err != nil {
		return err
	}

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := s.repo.ClaimPendingInvitations(ctx, user.Email, user.ID); err != nil {
		s.logger.Warn("Failed linking pending invitations on email verification", "user_id", user.ID, "error", err)
	}
	return nil
}

func (s *authService) ResendVerification(ctx context.Context, userID uuid.UUID) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.IsEmailVerified {
		return ErrEmailAlreadyVerified
	}

	lastSent, err := s.repo.LastEmailVerificationSentAt(ctx, user.ID)
	if err != nil {
		return err
	}
	if lastSent != nil && time.Since(*lastSent) < verificationResendCooldown {
		return ErrVerificationRateLimited
	}

	verifyURL, err := s.issueVerificationURL(ctx, user)
	if err != nil {
		return err
	}
	if s.mailer != nil {
		_ = s.mailer.SendVerificationEmail(user.Email, user.FirstName, verifyURL, s.verificationExpiryHours())
	}
	return nil
}

// issueVerificationURL stores a fresh verification token for user, which
// invalidates any earlier one, and returns the link to email.
func (s *authService) issueVerificationURL(ctx context.Context, user *User) (string, error) {
	rawToken, tokenHash, err := token.GenerateOpaqueToken()
	if err != nil {
		return "", fmt.Errorf("failed generating verification token: %w", err)
	}

	record := &EmailVerificationToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().UTC().Add(time.Duration(s.verificationExpiryHours()) * time.Hour),
	}
	if err := s.repo.CreateEmailVerificationToken(ctx, record); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/verify-email?token=%s", s.cfg.FrontendURL, rawToken), nil
}

func (s *authService) verificationExpiryHours() int {
	if s.cfg.EmailVerificationExpiryHours <= 0 {
		return 24
	}
	return s.cfg.EmailVerificationExpiryHours
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
