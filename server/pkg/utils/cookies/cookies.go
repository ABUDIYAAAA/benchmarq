package cookies

import (
	"net/http"
	"strings"
	"time"

	"github.com/ABUDIYAAAA/benchmarq/internal/config"
)

const (
	AccessTokenCookieName  = "access_token"
	RefreshTokenCookieName = "refresh_token"
	DeviceIDCookieName     = "device_id"
)

// ParseSameSite converts config string to http.SameSite enum.
func ParseSameSite(sameSite string) http.SameSite {
	switch strings.ToLower(sameSite) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

// SetAuthCookies sets access_token, refresh_token, and device_id cookies on the response.
func SetAuthCookies(w http.ResponseWriter, cfg *config.Config, accessToken string, refreshToken string, deviceID string) {
	sameSite := ParseSameSite(cfg.CookieSameSite)

	// Access Token Cookie
	accessMaxAge := cfg.JWTAccessExpiryMinutes * 60
	http.SetCookie(w, &http.Cookie{
		Name:     AccessTokenCookieName,
		Value:    accessToken,
		Path:     cfg.CookiePath,
		Domain:   cfg.CookieDomain,
		MaxAge:   accessMaxAge,
		Expires:  time.Now().Add(time.Duration(accessMaxAge) * time.Second),
		HttpOnly: cfg.CookieHTTPOnly,
		Secure:   cfg.CookieSecure,
		SameSite: sameSite,
	})

	// Refresh Token Cookie (Scraped only on auth refresh/logout endpoints)
	refreshMaxAge := cfg.JWTRefreshExpiryDays * 24 * 60 * 60
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshTokenCookieName,
		Value:    refreshToken,
		Path:     "/api/v1/auth",
		Domain:   cfg.CookieDomain,
		MaxAge:   refreshMaxAge,
		Expires:  time.Now().Add(time.Duration(refreshMaxAge) * time.Second),
		HttpOnly: true, // Always HttpOnly for refresh token
		Secure:   cfg.CookieSecure,
		SameSite: sameSite,
	})

	// Device ID Cookie (Persistent across logins on this browser)
	deviceMaxAge := 365 * 24 * 60 * 60 // 1 year
	http.SetCookie(w, &http.Cookie{
		Name:     DeviceIDCookieName,
		Value:    deviceID,
		Path:     cfg.CookiePath,
		Domain:   cfg.CookieDomain,
		MaxAge:   deviceMaxAge,
		Expires:  time.Now().Add(time.Duration(deviceMaxAge) * time.Second),
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		SameSite: sameSite,
	})
}

// SetAccessTokenCookie updates just the access token cookie (e.g. after refresh).
func SetAccessTokenCookie(w http.ResponseWriter, cfg *config.Config, accessToken string) {
	sameSite := ParseSameSite(cfg.CookieSameSite)
	accessMaxAge := cfg.JWTAccessExpiryMinutes * 60

	http.SetCookie(w, &http.Cookie{
		Name:     AccessTokenCookieName,
		Value:    accessToken,
		Path:     cfg.CookiePath,
		Domain:   cfg.CookieDomain,
		MaxAge:   accessMaxAge,
		Expires:  time.Now().Add(time.Duration(accessMaxAge) * time.Second),
		HttpOnly: cfg.CookieHTTPOnly,
		Secure:   cfg.CookieSecure,
		SameSite: sameSite,
	})
}

// SetRefreshTokenCookie updates the refresh token cookie (after rotation).
func SetRefreshTokenCookie(w http.ResponseWriter, cfg *config.Config, refreshToken string) {
	sameSite := ParseSameSite(cfg.CookieSameSite)
	refreshMaxAge := cfg.JWTRefreshExpiryDays * 24 * 60 * 60

	http.SetCookie(w, &http.Cookie{
		Name:     RefreshTokenCookieName,
		Value:    refreshToken,
		Path:     "/api/v1/auth",
		Domain:   cfg.CookieDomain,
		MaxAge:   refreshMaxAge,
		Expires:  time.Now().Add(time.Duration(refreshMaxAge) * time.Second),
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		SameSite: sameSite,
	})
}

// SetDeviceIDCookie updates or persists the device ID cookie.
func SetDeviceIDCookie(w http.ResponseWriter, cfg *config.Config, deviceID string) {
	sameSite := ParseSameSite(cfg.CookieSameSite)
	deviceMaxAge := 365 * 24 * 60 * 60 // 1 year

	http.SetCookie(w, &http.Cookie{
		Name:     DeviceIDCookieName,
		Value:    deviceID,
		Path:     cfg.CookiePath,
		Domain:   cfg.CookieDomain,
		MaxAge:   deviceMaxAge,
		Expires:  time.Now().Add(time.Duration(deviceMaxAge) * time.Second),
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		SameSite: sameSite,
	})
}
func ClearAuthCookies(w http.ResponseWriter, cfg *config.Config) {
	sameSite := ParseSameSite(cfg.CookieSameSite)

	http.SetCookie(w, &http.Cookie{
		Name:     AccessTokenCookieName,
		Value:    "",
		Path:     cfg.CookiePath,
		Domain:   cfg.CookieDomain,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: cfg.CookieHTTPOnly,
		Secure:   cfg.CookieSecure,
		SameSite: sameSite,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     RefreshTokenCookieName,
		Value:    "",
		Path:     "/api/v1/auth",
		Domain:   cfg.CookieDomain,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		SameSite: sameSite,
	})
}

// GetAccessTokenFromRequest extracts access token from Cookie or Authorization header fallback.
func GetAccessTokenFromRequest(r *http.Request) string {
	if cookie, err := r.Cookie(AccessTokenCookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	return ""
}

// GetRefreshTokenFromRequest extracts refresh token from cookie.
func GetRefreshTokenFromRequest(r *http.Request) string {
	if cookie, err := r.Cookie(RefreshTokenCookieName); err == nil {
		return cookie.Value
	}
	return ""
}

// GetDeviceIDFromRequest extracts device ID from cookie.
func GetDeviceIDFromRequest(r *http.Request) string {
	if cookie, err := r.Cookie(DeviceIDCookieName); err == nil {
		return cookie.Value
	}
	return ""
}
