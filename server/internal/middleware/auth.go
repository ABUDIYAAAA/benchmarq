package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/ABUDIYAAAA/benchmarq/internal/config"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/cookies"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/response"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/token"
	"github.com/google/uuid"
)

type contextKey string

const (
	UserIDContextKey   contextKey = "user_id"
	UserEmailContextKey contextKey = "user_email"
	DeviceIDContextKey contextKey = "device_id"
)

// AuthRequired ensures that the request carries a valid access token.
func AuthRequired(appCfg *config.AppConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := cookies.GetAccessTokenFromRequest(r)
			if tokenStr == "" {
				response.Error(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			claims, err := token.ValidateAccessToken(tokenStr, appCfg.Config.JWTSecret)
			if err != nil {
				if errors.Is(err, token.ErrTokenExpired) {
					response.Error(w, http.StatusUnauthorized, "Access token expired")
					return
				}
				response.Error(w, http.StatusUnauthorized, "Invalid authentication token")
				return
			}

			// Store claims in request context
			ctx := context.WithValue(r.Context(), UserIDContextKey, claims.UserID)
			ctx = context.WithValue(ctx, UserEmailContextKey, claims.Email)
			ctx = context.WithValue(ctx, DeviceIDContextKey, claims.DeviceID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID extracts the authenticated user's ID from context.
func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(UserIDContextKey).(uuid.UUID)
	return id, ok
}

// GetUserEmail extracts the authenticated user's email from context.
func GetUserEmail(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(UserEmailContextKey).(string)
	return email, ok
}

// GetDeviceID extracts the active device ID from context.
func GetDeviceID(ctx context.Context) (string, bool) {
	deviceID, ok := ctx.Value(DeviceIDContextKey).(string)
	return deviceID, ok
}
