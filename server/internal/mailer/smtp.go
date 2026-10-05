package mailer

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wneessen/go-mail"
)

// SMTPConfig holds configuration settings for connecting to an SMTP server.
type SMTPConfig struct {
	Host        string
	Port        int
	Username    string
	Password    string
	FromEmail   string
	FromName    string
	Encryption  string // "none", "starttls", "tls", "ssl"
	TimeoutSecs int
}

// SMTPSender implements the Sender interface using github.com/wneessen/go-mail.
type SMTPSender struct {
	cfg       SMTPConfig
	clientOpt []mail.Option
}

// NewSMTPSender creates and returns a configured SMTPSender instance.
func NewSMTPSender(cfg SMTPConfig) (*SMTPSender, error) {
	if cfg.Host == "" {
		cfg.Host = "localhost"
	}
	if cfg.Port <= 0 {
		cfg.Port = 25
	}
	if cfg.TimeoutSecs <= 0 {
		cfg.TimeoutSecs = 15
	}

	sender := &SMTPSender{
		cfg: cfg,
	}

	opts := []mail.Option{
		mail.WithPort(cfg.Port),
		mail.WithTimeout(time.Duration(cfg.TimeoutSecs) * time.Second),
	}

	// Configure Encryption / TLS
	switch strings.ToLower(cfg.Encryption) {
	case "ssl", "tls":
		opts = append(opts, mail.WithSSL())
	case "starttls":
		opts = append(opts, mail.WithTLSPolicy(mail.TLSMandatory))
	case "none":
		opts = append(opts, mail.WithTLSPolicy(mail.NoTLS))
	default:
		opts = append(opts, mail.WithTLSPolicy(mail.TLSOpportunistic))
	}

	// Configure Authentication if credentials provided
	if cfg.Username != "" {
		opts = append(opts, mail.WithUsername(cfg.Username))
	}
	if cfg.Password != "" {
		opts = append(opts, mail.WithPassword(cfg.Password))
		opts = append(opts, mail.WithSMTPAuth(mail.SMTPAuthPlain))
	}

	sender.clientOpt = opts
	return sender, nil
}

// newClient creates a new go-mail Client instance with current options.
func (s *SMTPSender) newClient() (*mail.Client, error) {
	return mail.NewClient(s.cfg.Host, s.clientOpt...)
}

// Send sends a single email message via SMTP.
func (s *SMTPSender) Send(ctx context.Context, email *Email) error {
	msg, err := s.buildMessage(email)
	if err != nil {
		return fmt.Errorf("failed to build email message: %w", err)
	}

	client, err := s.newClient()
	if err != nil {
		return fmt.Errorf("failed to initialize smtp client: %w", err)
	}
	defer client.Close()

	if err := client.DialAndSendWithContext(ctx, msg); err != nil {
		return fmt.Errorf("failed to send email to %v: %w", email.To, err)
	}

	return nil
}

// SendBatch sends multiple email messages sequentially through a single persistent SMTP connection.
func (s *SMTPSender) SendBatch(ctx context.Context, emails []*Email) error {
	if len(emails) == 0 {
		return nil
	}

	msgs := make([]*mail.Msg, 0, len(emails))
	for i, email := range emails {
		msg, err := s.buildMessage(email)
		if err != nil {
			return fmt.Errorf("failed to build batch email message #%d (%v): %w", i, email.To, err)
		}
		msgs = append(msgs, msg)
	}

	client, err := s.newClient()
	if err != nil {
		return fmt.Errorf("failed to initialize smtp client for batch: %w", err)
	}
	defer client.Close()

	if err := client.DialAndSendWithContext(ctx, msgs...); err != nil {
		return fmt.Errorf("failed to send batch of %d emails: %w", len(emails), err)
	}

	return nil
}

// Close satisfies the Sender interface.
func (s *SMTPSender) Close() error {
	return nil
}

// buildMessage translates an internal Email model into a go-mail *mail.Msg.
func (s *SMTPSender) buildMessage(email *Email) (*mail.Msg, error) {
	msg := mail.NewMsg()

	// 1. From
	fromAddr := email.From
	if fromAddr == "" {
		fromAddr = s.cfg.FromEmail
	}
	fromName := email.FromName
	if fromName == "" {
		fromName = s.cfg.FromName
	}

	if fromName != "" {
		if err := msg.FromFormat(fromName, fromAddr); err != nil {
			return nil, fmt.Errorf("invalid from address %s <%s>: %w", fromName, fromAddr, err)
		}
	} else {
		if err := msg.From(fromAddr); err != nil {
			return nil, fmt.Errorf("invalid from address %s: %w", fromAddr, err)
		}
	}

	// 2. Recipients
	if len(email.To) == 0 {
		return nil, fmt.Errorf("email must have at least one recipient (To)")
	}
	if err := msg.To(email.To...); err != nil {
		return nil, fmt.Errorf("invalid to recipients %v: %w", email.To, err)
	}
	if len(email.Cc) > 0 {
		if err := msg.Cc(email.Cc...); err != nil {
			return nil, fmt.Errorf("invalid cc recipients %v: %w", email.Cc, err)
		}
	}
	if len(email.Bcc) > 0 {
		if err := msg.Bcc(email.Bcc...); err != nil {
			return nil, fmt.Errorf("invalid bcc recipients %v: %w", email.Bcc, err)
		}
	}
	if email.ReplyTo != "" {
		if err := msg.ReplyTo(email.ReplyTo); err != nil {
			return nil, fmt.Errorf("invalid reply-to address %s: %w", email.ReplyTo, err)
		}
	}

	// 3. Subject
	msg.Subject(email.Subject)

	// 4. Body (Plain Text & HTML Alternative)
	if email.PlainText != "" && email.HTMLBody != "" {
		msg.SetBodyString(mail.TypeTextPlain, email.PlainText)
		msg.AddAlternativeString(mail.TypeTextHTML, email.HTMLBody)
	} else if email.HTMLBody != "" {
		msg.SetBodyString(mail.TypeTextHTML, email.HTMLBody)
	} else if email.PlainText != "" {
		msg.SetBodyString(mail.TypeTextPlain, email.PlainText)
	}

	// 5. Attachments
	for _, att := range email.Attachments {
		if len(att.Data) > 0 {
			opts := []mail.FileOption{}
			if att.ContentType != "" {
				opts = append(opts, mail.WithFileContentType(mail.ContentType(att.ContentType)))
			}
			reader := bytes.NewReader(att.Data)
			if err := msg.AttachReader(att.Filename, reader, opts...); err != nil {
				return nil, fmt.Errorf("failed to attach file %q: %w", att.Filename, err)
			}
		} else if att.Path != "" {
			msg.AttachFile(att.Path)
		}
	}

	// 6. Custom Headers
	for k, v := range email.Headers {
		msg.SetGenHeader(mail.Header(k), v)
	}

	return msg, nil
}
