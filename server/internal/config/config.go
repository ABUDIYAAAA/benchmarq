package config

import (
	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
)

type Config struct {
	Port           string   `env:"PORT" envDefault:"8000"`
	DatabaseUrl    string   `env:"DATABASE_URL"`
	AllowedOrigins []string `env:"ALLOWED_ORIGINS" envSeparator:"," envDefault:"http://localhost:3000,http://localhost:5173"`
	Environment    string   `env:"ENV" envDefault:"development"`
	FrontendURL    string   `env:"FRONTEND_URL" envDefault:"http://localhost:3000"`
	// TrustedProxies are the reverse proxy ranges whose X-Forwarded-For and
	// X-Real-IP headers are believed. Requests from anywhere else keep their
	// socket address.
	TrustedProxies []string `env:"TRUSTED_PROXIES" envSeparator:"," envDefault:"127.0.0.0/8,::1/128,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,fc00::/7"`
	// DesktopLaunchURL is the deep link that opens the exam desktop app.
	DesktopLaunchURL string `env:"DESKTOP_LAUNCH_URL" envDefault:"benchmarq://launch"`

	// JWT Configuration
	JWTSecret              string `env:"JWT_SECRET" envDefault:"benchmarq_super_secret_jwt_key_change_in_prod_12345"`
	JWTAccessExpiryMinutes int    `env:"JWT_ACCESS_EXPIRY_MINUTES" envDefault:"15"`
	JWTRefreshExpiryDays   int    `env:"JWT_REFRESH_EXPIRY_DAYS" envDefault:"7"`
	JWTIssuer              string `env:"JWT_ISSUER" envDefault:"benchmarq"`

	// Cookie Configuration
	CookieDomain   string `env:"COOKIE_DOMAIN" envDefault:""`
	CookieSecure   bool   `env:"COOKIE_SECURE" envDefault:"false"`
	CookieHTTPOnly bool   `env:"COOKIE_HTTP_ONLY" envDefault:"true"`
	CookieSameSite string `env:"COOKIE_SAME_SITE" envDefault:"lax"` // lax, strict, none
	CookiePath     string `env:"COOKIE_PATH" envDefault:"/"`

	// Password Reset Configuration
	ResetPasswordExpiryMinutes int `env:"RESET_PASSWORD_EXPIRY_MINUTES" envDefault:"15"`

	// Email Verification & Invitation Configuration
	EmailVerificationExpiryHours int `env:"EMAIL_VERIFICATION_EXPIRY_HOURS" envDefault:"24"`
	OrgInviteExpiryDays          int `env:"ORG_INVITE_EXPIRY_DAYS" envDefault:"7"`

	// SMTP & Email configuration
	SMTPHost       string `env:"SMTP_HOST" envDefault:"localhost"`
	SMTPPort       int    `env:"SMTP_PORT" envDefault:"1025"`
	SMTPUsername   string `env:"SMTP_USERNAME" envDefault:""`
	SMTPPassword   string `env:"SMTP_PASSWORD" envDefault:""`
	SMTPFromEmail  string `env:"SMTP_FROM_EMAIL" envDefault:"noreply@benchmarq.io"`
	SMTPFromName   string `env:"SMTP_FROM_NAME" envDefault:"Benchmarq"`
	SMTPEncryption string `env:"SMTP_ENCRYPTION" envDefault:"none"`

	// Email Worker Group configuration for async & batch sending
	EmailWorkers   int `env:"EMAIL_WORKERS" envDefault:"5"`
	EmailQueueSize int `env:"EMAIL_QUEUE_SIZE" envDefault:"100"`
}

func NewConfig() (*Config, error) {
	_ = godotenv.Load()

	var config Config
	err := env.Parse(&config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
