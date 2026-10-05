package mailer

import (
	"context"
	"fmt"
	"log/slog"
)

// Config holds all configuration options needed to initialize the mailer system.
type Config struct {
	SMTPHost       string
	SMTPPort       int
	SMTPUsername   string
	SMTPPassword   string
	SMTPFromEmail  string
	SMTPFromName   string
	SMTPEncryption string
	EmailWorkers   int
	EmailQueueSize int
	Environment    string
}

// Manager implements the Mailer interface coordinating templating, dispatching, and background workers.
type Manager struct {
	cfg            Config
	logger         *slog.Logger
	sender         Sender
	templateEngine *TemplateEngine
	workerPool     *WorkerPool
}

// NewManager creates and initializes the complete Mailer manager.
func NewManager(cfg Config, logger *slog.Logger) (*Manager, error) {
	tmplEngine := NewTemplateEngine()

	var sender Sender
	var err error

	smtpCfg := SMTPConfig{
		Host:        cfg.SMTPHost,
		Port:        cfg.SMTPPort,
		Username:    cfg.SMTPUsername,
		Password:    cfg.SMTPPassword,
		FromEmail:   cfg.SMTPFromEmail,
		FromName:    cfg.SMTPFromName,
		Encryption:  cfg.SMTPEncryption,
		TimeoutSecs: 15,
	}
	sender, err = NewSMTPSender(smtpCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize smtp sender: %w", err)
	}

	workerCfg := WorkerPoolConfig{
		WorkerCount: cfg.EmailWorkers,
		QueueSize:   cfg.EmailQueueSize,
	}
	pool := NewWorkerPool(workerCfg, sender, logger)

	return &Manager{
		cfg:            cfg,
		logger:         logger,
		sender:         sender,
		templateEngine: tmplEngine,
		workerPool:     pool,
	}, nil
}

// Start begins background worker group processing.
func (m *Manager) Start(ctx context.Context) {
	m.workerPool.Start(ctx)
}

// Stop gracefully shuts down the worker group.
func (m *Manager) Stop() {
	m.workerPool.Stop()
	_ = m.sender.Close()
}

// RenderTemplate renders the template and returns HTML and plain text bodies.
func (m *Manager) RenderTemplate(templateName string, data any) (string, string, error) {
	return m.templateEngine.Render(templateName, data)
}

// prepareEmail checks if a template is specified and renders it if needed.
func (m *Manager) prepareEmail(email *Email) error {
	if email.From == "" {
		email.From = m.cfg.SMTPFromEmail
	}
	if email.FromName == "" {
		email.FromName = m.cfg.SMTPFromName
	}

	if email.TemplateName != "" {
		html, text, err := m.templateEngine.Render(email.TemplateName, email.TemplateData)
		if err != nil {
			return fmt.Errorf("failed to render template %q: %w", email.TemplateName, err)
		}
		email.HTMLBody = html
		email.PlainText = text

		if email.Subject == "" {
			if title, err := m.templateEngine.RenderTitle(email.TemplateName, email.TemplateData); err == nil && title != "" {
				email.Subject = title
			}
		}
	}

	return nil
}

// Send sends a single email synchronously.
func (m *Manager) Send(ctx context.Context, email *Email) error {
	if err := m.prepareEmail(email); err != nil {
		return err
	}
	return m.sender.Send(ctx, email)
}

// SendBatch sends multiple emails in a single batch connection synchronously.
func (m *Manager) SendBatch(ctx context.Context, emails []*Email) error {
	for i, email := range emails {
		if err := m.prepareEmail(email); err != nil {
			return fmt.Errorf("failed preparing email #%d: %w", i, err)
		}
	}
	return m.sender.SendBatch(ctx, emails)
}

// Enqueue queues a single email for background asynchronous sending.
func (m *Manager) Enqueue(email *Email) error {
	if err := m.prepareEmail(email); err != nil {
		return err
	}
	return m.workerPool.Enqueue(email)
}

// EnqueueBatch queues a batch of emails for background asynchronous batch sending.
func (m *Manager) EnqueueBatch(emails []*Email) error {
	for i, email := range emails {
		if err := m.prepareEmail(email); err != nil {
			return fmt.Errorf("failed preparing batch email #%d: %w", i, err)
		}
	}
	return m.workerPool.EnqueueBatch(emails)
}

// Convenience helper methods for common application emails:

// SendWelcomeEmail queues a welcome/verification email to a newly registered user.
func (m *Manager) SendWelcomeEmail(to string, name string, verifyURL string) error {
	return m.Enqueue(&Email{
		To:           []string{to},
		Subject:      "Welcome to Benchmarq",
		TemplateName: "welcome.html",
		TemplateData: map[string]any{
			"Name":      name,
			"VerifyURL": verifyURL,
		},
	})
}

// SendPasswordResetEmail queues a password reset email.
func (m *Manager) SendPasswordResetEmail(to string, name string, resetURL string, expiresMinutes int) error {
	if expiresMinutes <= 0 {
		expiresMinutes = 15
	}
	return m.Enqueue(&Email{
		To:           []string{to},
		Subject:      "Reset Your Password - Benchmarq",
		TemplateName: "reset_password.html",
		TemplateData: map[string]any{
			"Name":           name,
			"ResetURL":       resetURL,
			"ExpiresMinutes": expiresMinutes,
		},
	})
}

// SendExamInviteEmail queues an exam invitation email to a candidate.
func (m *Manager) SendExamInviteEmail(to string, studentName string, examTitle string, examURL string, startTime string, duration string, accessCode string) error {
	return m.Enqueue(&Email{
		To:           []string{to},
		Subject:      fmt.Sprintf("Invitation to Exam: %s", examTitle),
		TemplateName: "exam_invite.html",
		TemplateData: map[string]any{
			"StudentName": studentName,
			"ExamTitle":   examTitle,
			"ExamURL":     examURL,
			"StartTime":   startTime,
			"Duration":    duration,
			"AccessCode":  accessCode,
		},
	})
}

// SendExamResultEmail queues an exam completion and results email to a candidate.
func (m *Manager) SendExamResultEmail(to string, studentName string, examTitle string, score float64, totalMarks float64, percentage float64, resultURL string) error {
	return m.Enqueue(&Email{
		To:           []string{to},
		Subject:      fmt.Sprintf("Your Results for %s", examTitle),
		TemplateName: "exam_result.html",
		TemplateData: map[string]any{
			"StudentName": studentName,
			"ExamTitle":   examTitle,
			"Score":       score,
			"TotalMarks":  totalMarks,
			"Percentage":  percentage,
			"ResultURL":   resultURL,
		},
	})
}
