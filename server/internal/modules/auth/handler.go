package auth

import (
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/ABUDIYAAAA/benchmarq/internal/config"
	"github.com/ABUDIYAAAA/benchmarq/internal/middleware"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/cookies"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/response"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/token"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/validator"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service Service
	cfg     *config.Config
}

func NewHandler(service Service, cfg *config.Config) *Handler {
	return &Handler{
		service: service,
		cfg:     cfg,
	}
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/auth", func(r chi.Router) {
		r.Post("/signup", h.SignUpHandler)
		r.Post("/login", h.LoginHandler)
		r.Post("/refresh", h.RefreshHandler)
		r.Post("/logout", h.LogoutHandler)
		r.Post("/forgot-password", h.ForgotPasswordHandler)
		r.Post("/reset-password", h.ResetPasswordHandler)

		// Protected endpoints
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)
			r.Get("/me", h.GetMeHandler)
		})
	})
}

// SignUpHandler handles user registration.
func (h *Handler) SignUpHandler(w http.ResponseWriter, r *http.Request) {
	var req SignUpRequest
	if errs, err := validator.DecodeAndValidate(r, &req); err != nil {
		response.ValidationError(w, errs)
		return
	}

	clientIP := extractClientIP(r)
	userAgent := r.UserAgent()
	clientDeviceID := cookies.GetDeviceIDFromRequest(r)

	authResp, tokens, err := h.service.SignUp(r.Context(), &req, clientIP, userAgent, clientDeviceID)
	if err != nil {
		if errors.Is(err, ErrUserAlreadyExists) {
			response.Error(w, http.StatusConflict, err.Error())
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to create account")
		return
	}

	// Set auth cookies (access_token, refresh_token, device_id)
	cookies.SetAuthCookies(w, h.cfg, tokens.AccessToken, tokens.RefreshToken, authResp.DeviceID)

	response.Success(w, http.StatusCreated, "User registered successfully", authResp)
}

// LoginHandler handles user login.
func (h *Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if errs, err := validator.DecodeAndValidate(r, &req); err != nil {
		response.ValidationError(w, errs)
		return
	}

	clientIP := extractClientIP(r)
	userAgent := r.UserAgent()
	clientDeviceID := cookies.GetDeviceIDFromRequest(r)

	authResp, tokens, err := h.service.Login(r.Context(), &req, clientIP, userAgent, clientDeviceID)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) || errors.Is(err, ErrUserNotFound) {
			response.Error(w, http.StatusUnauthorized, "Invalid email or password")
			return
		}
		if errors.Is(err, ErrAccountInactive) {
			response.Error(w, http.StatusForbidden, "Account is deactivated")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to authenticate")
		return
	}

	// Set auth cookies (access_token, refresh_token, device_id)
	cookies.SetAuthCookies(w, h.cfg, tokens.AccessToken, tokens.RefreshToken, authResp.DeviceID)

	response.Success(w, http.StatusOK, "Login successful", authResp)
}

// RefreshHandler handles JWT access and refresh token rotation.
func (h *Handler) RefreshHandler(w http.ResponseWriter, r *http.Request) {
	rawRefreshToken := cookies.GetRefreshTokenFromRequest(r)
	if rawRefreshToken == "" {
		response.Error(w, http.StatusUnauthorized, "Refresh token required")
		return
	}

	clientIP := extractClientIP(r)
	userAgent := r.UserAgent()
	clientDeviceID := cookies.GetDeviceIDFromRequest(r)

	tokens, deviceID, err := h.service.RefreshToken(r.Context(), rawRefreshToken, clientDeviceID, clientIP, userAgent)
	if err != nil {
		cookies.ClearAuthCookies(w, h.cfg)
		if errors.Is(err, token.ErrTokenExpired) || errors.Is(err, token.ErrInvalidToken) || errors.Is(err, ErrInvalidDevice) {
			response.Error(w, http.StatusUnauthorized, "Session expired or invalid. Please log in again.")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to refresh token")
		return
	}

	// Set rotated cookies
	cookies.SetAccessTokenCookie(w, h.cfg, tokens.AccessToken)
	cookies.SetRefreshTokenCookie(w, h.cfg, tokens.RefreshToken)
	cookies.SetDeviceIDCookie(w, h.cfg, deviceID)

	response.Success(w, http.StatusOK, "Token refreshed successfully", map[string]string{"status": "ok"})
}

// LogoutHandler terminates the user's active session.
func (h *Handler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	rawRefreshToken := cookies.GetRefreshTokenFromRequest(r)
	_ = h.service.Logout(r.Context(), rawRefreshToken)

	// Clear cookies from browser
	cookies.ClearAuthCookies(w, h.cfg)

	response.Success(w, http.StatusOK, "Logged out successfully", nil)
}

// ForgotPasswordHandler sends a password reset email.
func (h *Handler) ForgotPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if errs, err := validator.DecodeAndValidate(r, &req); err != nil {
		response.ValidationError(w, errs)
		return
	}

	_ = h.service.ForgotPassword(r.Context(), &req)

	// Always return standard message to prevent email enumeration
	response.Success(w, http.StatusOK, "If an account with that email exists, a password reset link has been sent.", nil)
}

// ResetPasswordHandler sets a new password using a reset token.
func (h *Handler) ResetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req ResetPasswordRequest
	if errs, err := validator.DecodeAndValidate(r, &req); err != nil {
		response.ValidationError(w, errs)
		return
	}

	if err := h.service.ResetPassword(r.Context(), &req); err != nil {
		if errors.Is(err, ErrResetTokenNotFound) || errors.Is(err, ErrResetTokenExpired) || errors.Is(err, ErrResetTokenAlreadyUsed) {
			response.Error(w, http.StatusBadRequest, "Invalid or expired password reset link")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to reset password")
		return
	}

	// Clear any active auth cookies
	cookies.ClearAuthCookies(w, h.cfg)

	response.Success(w, http.StatusOK, "Password has been successfully reset. Please log in with your new password.", nil)
}

// GetMeHandler returns current authenticated user information.
func (h *Handler) GetMeHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	user, err := h.service.GetMe(r.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			response.Error(w, http.StatusNotFound, "User not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to load user profile")
		return
	}

	response.Success(w, http.StatusOK, "User profile loaded", user)
}

func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return strings.TrimSpace(xrip)
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return ip
	}
	return r.RemoteAddr
}
