package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound          = errors.New("user not found")
	ErrUserAlreadyExists     = errors.New("a user with this email already exists")
	ErrSessionNotFound       = errors.New("session not found")
	ErrSessionRotated        = errors.New("session refresh token was already rotated")
	ErrResetTokenNotFound    = errors.New("password reset token not found or invalid")
	ErrResetTokenExpired     = errors.New("password reset token has expired")
	ErrResetTokenAlreadyUsed = errors.New("password reset token has already been used")
	ErrVerificationInvalid   = errors.New("email verification token not found, used or expired")
)

type Repository interface {
	CreateUser(ctx context.Context, user *User) error
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)
	UpdateUserPassword(ctx context.Context, userID uuid.UUID, passwordHash string) error
	CreateSession(ctx context.Context, session *UserSession) error
	// GetSessionByRefreshTokenHash finds the session whose current or
	// previous refresh token has this hash.
	GetSessionByRefreshTokenHash(ctx context.Context, tokenHash string) (*UserSession, error)
	// RotateSessionRefreshToken replaces the session's refresh token only if
	// it is still currentHash, keeping currentHash as the previous token. It
	// returns ErrSessionRotated if another request rotated it first.
	RotateSessionRefreshToken(ctx context.Context, sessionID uuid.UUID, currentHash, newHash string, newExpiresAt time.Time) error
	RevokeSession(ctx context.Context, sessionID uuid.UUID) error
	RevokeUserSessions(ctx context.Context, userID uuid.UUID) error
	RevokeDeviceSessions(ctx context.Context, userID uuid.UUID, deviceID string) error
	CreatePasswordResetToken(ctx context.Context, token *PasswordResetToken) error
	GetPasswordResetTokenByHash(ctx context.Context, tokenHash string) (*PasswordResetToken, error)
	MarkPasswordResetTokenUsed(ctx context.Context, tokenID uuid.UUID) error
	// CreateEmailVerificationToken stores a new token and invalidates every
	// earlier unused token of the same user.
	CreateEmailVerificationToken(ctx context.Context, token *EmailVerificationToken) error
	// LastEmailVerificationSentAt returns when the newest token for the user
	// was issued, or nil if none was.
	LastEmailVerificationSentAt(ctx context.Context, userID uuid.UUID) (*time.Time, error)
	// ConsumeEmailVerificationToken marks a valid token used and the owning
	// user verified in one statement, returning that user's ID.
	ConsumeEmailVerificationToken(ctx context.Context, tokenHash string) (uuid.UUID, error)
	ClaimPendingInvitations(ctx context.Context, email string, userID uuid.UUID) error
}

type postgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) CreateUser(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (
			id, email, password_hash, first_name, last_name, avatar_url, is_active, is_email_verified, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
	`
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	now := time.Now().UTC()
	user.CreatedAt = now
	user.UpdatedAt = now

	_, err := r.db.Exec(ctx, query,
		user.ID,
		user.Email,
		user.PasswordHash,
		user.FirstName,
		user.LastName,
		user.AvatarURL,
		user.IsActive,
		user.IsEmailVerified,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return ErrUserAlreadyExists
		}
		return fmt.Errorf("failed creating user: %w", err)
	}

	return nil
}

func (r *postgresRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, email, password_hash, first_name, last_name, avatar_url, is_active, is_email_verified, created_at, updated_at, deleted_at
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
	`
	var user User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.FirstName,
		&user.LastName,
		&user.AvatarURL,
		&user.IsActive,
		&user.IsEmailVerified,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed fetching user by email: %w", err)
	}

	return &user, nil
}

func (r *postgresRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	query := `
		SELECT id, email, password_hash, first_name, last_name, avatar_url, is_active, is_email_verified, created_at, updated_at, deleted_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`
	var user User
	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.FirstName,
		&user.LastName,
		&user.AvatarURL,
		&user.IsActive,
		&user.IsEmailVerified,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed fetching user by id: %w", err)
	}

	return &user, nil
}

func (r *postgresRepository) UpdateUserPassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	query := `
		UPDATE users
		SET password_hash = $1, updated_at = now()
		WHERE id = $2 AND deleted_at IS NULL
	`
	tag, err := r.db.Exec(ctx, query, passwordHash, userID)
	if err != nil {
		return fmt.Errorf("failed updating user password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *postgresRepository) CreateSession(ctx context.Context, session *UserSession) error {
	query := `
		INSERT INTO user_sessions (
			id, user_id, device_id, refresh_token_hash, user_agent, ip_address, is_revoked, expires_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
	`
	if session.ID == uuid.Nil {
		session.ID = uuid.New()
	}
	now := time.Now().UTC()
	session.CreatedAt = now
	session.UpdatedAt = now

	_, err := r.db.Exec(ctx, query,
		session.ID,
		session.UserID,
		session.DeviceID,
		session.RefreshTokenHash,
		session.UserAgent,
		session.IPAddress,
		session.IsRevoked,
		session.ExpiresAt,
		session.CreatedAt,
		session.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed creating user session: %w", err)
	}

	return nil
}

func (r *postgresRepository) GetSessionByRefreshTokenHash(ctx context.Context, tokenHash string) (*UserSession, error) {
	query := `
		SELECT id, user_id, device_id, refresh_token_hash, user_agent, ip_address, is_revoked, expires_at, created_at, updated_at,
		       previous_refresh_token_hash, rotated_at
		FROM user_sessions
		WHERE refresh_token_hash = $1 OR previous_refresh_token_hash = $1
	`
	var session UserSession
	err := r.db.QueryRow(ctx, query, tokenHash).Scan(
		&session.ID,
		&session.UserID,
		&session.DeviceID,
		&session.RefreshTokenHash,
		&session.UserAgent,
		&session.IPAddress,
		&session.IsRevoked,
		&session.ExpiresAt,
		&session.CreatedAt,
		&session.UpdatedAt,
		&session.PreviousRefreshTokenHash,
		&session.RotatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed querying session: %w", err)
	}

	return &session, nil
}

func (r *postgresRepository) RotateSessionRefreshToken(ctx context.Context, sessionID uuid.UUID, currentHash, newHash string, newExpiresAt time.Time) error {
	query := `
		UPDATE user_sessions
		SET previous_refresh_token_hash = refresh_token_hash, refresh_token_hash = $3,
		    rotated_at = now(), expires_at = $4, updated_at = now()
		WHERE id = $1 AND refresh_token_hash = $2 AND is_revoked = FALSE
	`
	tag, err := r.db.Exec(ctx, query, sessionID, currentHash, newHash, newExpiresAt)
	if err != nil {
		return fmt.Errorf("failed rotating session refresh token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSessionRotated
	}
	return nil
}

func (r *postgresRepository) RevokeSession(ctx context.Context, sessionID uuid.UUID) error {
	query := `
		UPDATE user_sessions
		SET is_revoked = TRUE, updated_at = now()
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, sessionID)
	return err
}

func (r *postgresRepository) RevokeUserSessions(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE user_sessions
		SET is_revoked = TRUE, updated_at = now()
		WHERE user_id = $1
	`
	_, err := r.db.Exec(ctx, query, userID)
	return err
}

func (r *postgresRepository) RevokeDeviceSessions(ctx context.Context, userID uuid.UUID, deviceID string) error {
	query := `
		UPDATE user_sessions
		SET is_revoked = TRUE, updated_at = now()
		WHERE user_id = $1 AND device_id = $2
	`
	_, err := r.db.Exec(ctx, query, userID, deviceID)
	return err
}

func (r *postgresRepository) CreatePasswordResetToken(ctx context.Context, token *PasswordResetToken) error {
	query := `
		INSERT INTO password_reset_tokens (
			id, user_id, token_hash, is_used, expires_at, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6
		)
	`
	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}
	token.CreatedAt = time.Now().UTC()

	_, err := r.db.Exec(ctx, query,
		token.ID,
		token.UserID,
		token.TokenHash,
		token.IsUsed,
		token.ExpiresAt,
		token.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed saving password reset token: %w", err)
	}
	return nil
}

func (r *postgresRepository) GetPasswordResetTokenByHash(ctx context.Context, tokenHash string) (*PasswordResetToken, error) {
	query := `
		SELECT id, user_id, token_hash, is_used, expires_at, created_at
		FROM password_reset_tokens
		WHERE token_hash = $1
	`
	var token PasswordResetToken
	err := r.db.QueryRow(ctx, query, tokenHash).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.IsUsed,
		&token.ExpiresAt,
		&token.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrResetTokenNotFound
		}
		return nil, fmt.Errorf("failed querying password reset token: %w", err)
	}

	if token.IsUsed {
		return nil, ErrResetTokenAlreadyUsed
	}
	if time.Now().UTC().After(token.ExpiresAt) {
		return nil, ErrResetTokenExpired
	}

	return &token, nil
}

func (r *postgresRepository) MarkPasswordResetTokenUsed(ctx context.Context, tokenID uuid.UUID) error {
	query := `
		UPDATE password_reset_tokens
		SET is_used = TRUE
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, tokenID)
	return err
}

func (r *postgresRepository) CreateEmailVerificationToken(ctx context.Context, token *EmailVerificationToken) error {
	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}
	token.CreatedAt = time.Now().UTC()

	query := `
		WITH spent AS (
			UPDATE email_verification_tokens
			SET is_used = TRUE
			WHERE user_id = $2 AND is_used = FALSE
		)
		INSERT INTO email_verification_tokens (id, user_id, token_hash, is_used, expires_at, created_at)
		VALUES ($1, $2, $3, FALSE, $4, $5)
	`
	_, err := r.db.Exec(ctx, query, token.ID, token.UserID, token.TokenHash, token.ExpiresAt, token.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed saving email verification token: %w", err)
	}
	return nil
}

func (r *postgresRepository) LastEmailVerificationSentAt(ctx context.Context, userID uuid.UUID) (*time.Time, error) {
	var sentAt *time.Time
	err := r.db.QueryRow(ctx,
		`SELECT max(created_at) FROM email_verification_tokens WHERE user_id = $1`, userID,
	).Scan(&sentAt)
	if err != nil {
		return nil, fmt.Errorf("failed querying last verification email: %w", err)
	}
	return sentAt, nil
}

func (r *postgresRepository) ConsumeEmailVerificationToken(ctx context.Context, tokenHash string) (uuid.UUID, error) {
	query := `
		WITH consumed AS (
			UPDATE email_verification_tokens
			SET is_used = TRUE
			WHERE token_hash = $1 AND is_used = FALSE AND expires_at > now()
			RETURNING user_id
		)
		UPDATE users u
		SET is_email_verified = TRUE, updated_at = now()
		FROM consumed
		WHERE u.id = consumed.user_id AND u.deleted_at IS NULL
		RETURNING u.id
	`
	var userID uuid.UUID
	if err := r.db.QueryRow(ctx, query, tokenHash).Scan(&userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrVerificationInvalid
		}
		return uuid.Nil, fmt.Errorf("failed consuming email verification token: %w", err)
	}
	return userID, nil
}

func (r *postgresRepository) ClaimPendingInvitations(ctx context.Context, email string, userID uuid.UUID) error {
	query := `
		WITH pending AS (
			UPDATE organization_invitations
			SET status = 'accepted', accepted_at = now(), updated_at = now()
			WHERE email = $1 AND status = 'pending' AND expires_at > now()
			RETURNING org_id, role
		)
		INSERT INTO organization_members (org_id, user_id, role)
		SELECT org_id, $2, role FROM pending
		ON CONFLICT (org_id, user_id) DO NOTHING;
	`
	_, err := r.db.Exec(ctx, query, email, userID)
	if err != nil {
		return fmt.Errorf("failed linking pending organization invitations: %w", err)
	}
	return nil
}

func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "unique constraint")
}
