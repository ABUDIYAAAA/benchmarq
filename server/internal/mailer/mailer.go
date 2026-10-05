package mailer

import (
	"context"
)

// Email represents an email message to be sent.
type Email struct {
	From         string            `json:"from,omitempty"`
	FromName     string            `json:"from_name,omitempty"`
	To           []string          `json:"to"`
	Cc           []string          `json:"cc,omitempty"`
	Bcc          []string          `json:"bcc,omitempty"`
	ReplyTo      string            `json:"reply_to,omitempty"`
	Subject      string            `json:"subject"`
	HTMLBody     string            `json:"html_body,omitempty"`
	PlainText    string            `json:"plain_text,omitempty"`
	TemplateName string            `json:"template_name,omitempty"`
	TemplateData any               `json:"template_data,omitempty"`
	Attachments  []Attachment      `json:"attachments,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
}

// Attachment represents a file attached to an email message.
type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Data        []byte `json:"data,omitempty"`
	Path        string `json:"path,omitempty"`
}

// Sender defines the low-level transport interface for dispatching emails.
// Implementations can include SMTP, SES, Resend, Mock/Logger, etc.
type Sender interface {
	Send(ctx context.Context, email *Email) error
	SendBatch(ctx context.Context, emails []*Email) error
	Close() error
}

// Mailer defines the high-level application mailer service interface.
type Mailer interface {
	// Send sends a single email synchronously.
	Send(ctx context.Context, email *Email) error

	// SendBatch sends multiple emails in a single batch connection synchronously.
	SendBatch(ctx context.Context, emails []*Email) error

	// Enqueue adds an email to the asynchronous worker pool queue.
	Enqueue(email *Email) error

	// EnqueueBatch adds a list of emails to the asynchronous worker pool queue.
	EnqueueBatch(emails []*Email) error

	// RenderTemplate renders the given template with data.
	RenderTemplate(templateName string, data any) (htmlBody, textBody string, err error)

	// Start begins background worker goroutines.
	Start(ctx context.Context)

	// Stop gracefully shuts down workers, flushing remaining queued jobs.
	Stop()

	// Convenience helper methods
	SendWelcomeEmail(to string, name string, verifyURL string) error
	SendPasswordResetEmail(to string, name string, resetURL string, expiresMinutes int) error
	SendExamInviteEmail(to string, studentName string, examTitle string, examURL string, startTime string, duration string, accessCode string) error
	SendExamResultEmail(to string, studentName string, examTitle string, score float64, totalMarks float64, percentage float64, resultURL string) error
}
