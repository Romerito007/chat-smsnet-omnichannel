// Package email is the real SMTP email transport. A single SMTPSender renders
// HTML templates (embedded from templates/) and delivers them over SMTP. It
// implements both the notifications EmailSender port (the notification.email
// channel) and the auth Mailer port (account-lifecycle emails: verification,
// invitation, password reset and reset confirmation). There is no logging-only
// fallback: when no SMTP host is configured, sends fail loudly rather than
// silently drop.
package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"embed"
	"fmt"
	"html/template"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	authcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/auth/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/notifications/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

//go:embed templates/*.html
var templateFS embed.FS

// templates is parsed once at package init; each file is addressable by its base
// name (e.g. "verification.html").
var templates = template.Must(template.ParseFS(templateFS, "templates/*.html"))

// Config holds the SMTP transport settings the sender needs. It mirrors
// config.EmailConfig without importing the app layer.
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	// TLSMode is "starttls" (opportunistic, 587), "tls" (implicit, 465) or "none".
	TLSMode string
}

// Configured reports whether a host is set.
func (c Config) Configured() bool { return strings.TrimSpace(c.Host) != "" }

// SMTPSender renders templates and delivers email over SMTP.
type SMTPSender struct {
	cfg    Config
	logger shared.Logger
	// transport sends a fully-built message; injectable so tests can capture mail
	// without a live SMTP server.
	transport func(cfg Config, from string, to []string, msg []byte) error
}

// NewSMTPSender builds the sender.
func NewSMTPSender(cfg Config, logger shared.Logger) *SMTPSender {
	return &SMTPSender{cfg: cfg, logger: logger, transport: smtpDeliver}
}

// ── notifications EmailSender ─────────────────────────────────────────────────

// Send delivers a privacy-safe notification email (subject + optional deep link;
// never the notification body). Implements notifications/contracts.EmailSender.
func (s *SMTPSender) Send(_ context.Context, msg contracts.EmailMessage) error {
	body, err := s.render("notification.html", map[string]any{"Subject": msg.Subject, "Link": msg.Link})
	if err != nil {
		return err
	}
	return s.deliver(msg.To, subjectOr(msg.Subject, "Notification"), body)
}

// ── auth Mailer ───────────────────────────────────────────────────────────────

// SendVerification delivers the email-verification message.
func (s *SMTPSender) SendVerification(_ context.Context, msg authcontracts.AccountEmail) error {
	return s.renderAndDeliver("verification.html", "Confirm your email", msg)
}

// SendInvite delivers a teammate invitation.
func (s *SMTPSender) SendInvite(_ context.Context, msg authcontracts.AccountEmail) error {
	return s.renderAndDeliver("invite.html", "You have been invited", msg)
}

// SendPasswordReset delivers the password-reset link.
func (s *SMTPSender) SendPasswordReset(_ context.Context, msg authcontracts.AccountEmail) error {
	return s.renderAndDeliver("password_reset.html", "Reset your password", msg)
}

// SendPasswordResetDone confirms a completed reset (no link).
func (s *SMTPSender) SendPasswordResetDone(_ context.Context, msg authcontracts.AccountEmail) error {
	return s.renderAndDeliver("password_reset_done.html", "Your password was changed", msg)
}

func (s *SMTPSender) renderAndDeliver(tmpl, subject string, msg authcontracts.AccountEmail) error {
	body, err := s.render(tmpl, map[string]any{"Name": msg.Name, "Company": msg.Company, "Link": msg.Link})
	if err != nil {
		return err
	}
	return s.deliver(msg.To, subject, body)
}

// ── rendering + transport ─────────────────────────────────────────────────────

func (s *SMTPSender) render(name string, data any) ([]byte, error) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, fmt.Errorf("render email %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

func (s *SMTPSender) deliver(to, subject string, htmlBody []byte) error {
	to = strings.TrimSpace(to)
	if to == "" {
		return fmt.Errorf("email: empty recipient")
	}
	if !s.cfg.Configured() {
		return fmt.Errorf("email: SMTP host not configured (set SMTP_HOST)")
	}
	msg := buildMessage(s.cfg.From, to, subject, htmlBody)
	if err := s.transport(s.cfg, s.cfg.From, []string{to}, msg); err != nil {
		return fmt.Errorf("email: deliver to %s: %w", to, err)
	}
	if s.logger != nil {
		s.logger.Info("email sent", "to", to, "subject", subject)
	}
	return nil
}

// buildMessage assembles a minimal MIME HTML message with CRLF line endings.
func buildMessage(from, to, subject string, htmlBody []byte) []byte {
	var b bytes.Buffer
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("Date: " + time.Now().UTC().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("\r\n")
	b.Write(htmlBody)
	return b.Bytes()
}

// smtpDeliver is the real transport. It honors the TLS mode and authenticates
// with PLAIN auth when credentials are set.
func smtpDeliver(cfg Config, from string, to []string, msg []byte) error {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}

	var client *smtp.Client
	var err error
	if strings.EqualFold(cfg.TLSMode, "tls") {
		// Implicit TLS (typically port 465).
		conn, derr := tls.Dial("tcp", addr, &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12})
		if derr != nil {
			return derr
		}
		if client, err = smtp.NewClient(conn, cfg.Host); err != nil {
			return err
		}
	} else {
		if client, err = smtp.Dial(addr); err != nil {
			return err
		}
		if strings.EqualFold(cfg.TLSMode, "starttls") {
			if ok, _ := client.Extension("STARTTLS"); ok {
				if err = client.StartTLS(&tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
					_ = client.Close()
					return err
				}
			}
		}
	}
	defer func() { _ = client.Close() }()

	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return err
		}
	}
	if err = client.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err = client.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write(msg); err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func subjectOr(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

var (
	_ contracts.EmailSender = (*SMTPSender)(nil)
	_ authcontracts.Mailer  = (*SMTPSender)(nil)
)
