package email

import (
	"context"
	"strings"
	"testing"

	authcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/auth/contracts"
	ncontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/notifications/contracts"
)

type captured struct {
	from string
	to   []string
	msg  string
}

func newCapturingSender() (*SMTPSender, *captured) {
	cap := &captured{}
	s := NewSMTPSender(Config{Host: "smtp.test", Port: 587, From: "no-reply@test"}, nil)
	s.transport = func(_ Config, from string, to []string, msg []byte) error {
		cap.from, cap.to, cap.msg = from, to, string(msg)
		return nil
	}
	return s, cap
}

func TestSendVerification_RendersAndDelivers(t *testing.T) {
	s, cap := newCapturingSender()
	link := "https://app.test/verify-email?token=abc123"
	if err := s.SendVerification(context.Background(), authcontracts.AccountEmail{
		To: "alice@acme.com", Name: "Alice", Company: "Acme", Link: link,
	}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if cap.from != "no-reply@test" || len(cap.to) != 1 || cap.to[0] != "alice@acme.com" {
		t.Errorf("envelope wrong: from=%q to=%v", cap.from, cap.to)
	}
	if !strings.Contains(cap.msg, "To: alice@acme.com") || !strings.Contains(cap.msg, "Subject: Confirm your email") {
		t.Errorf("headers missing:\n%s", cap.msg)
	}
	if !strings.Contains(cap.msg, "Content-Type: text/html") {
		t.Error("expected an HTML content type")
	}
	if !strings.Contains(cap.msg, link) || !strings.Contains(cap.msg, "Acme") || !strings.Contains(cap.msg, "Alice") {
		t.Errorf("body did not render link/name/company:\n%s", cap.msg)
	}
}

func TestSendPasswordResetDone_NoLink(t *testing.T) {
	s, cap := newCapturingSender()
	if err := s.SendPasswordResetDone(context.Background(), authcontracts.AccountEmail{To: "al@acme.com", Name: "Al"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if !strings.Contains(cap.msg, "Subject: Your password was changed") {
		t.Errorf("subject wrong:\n%s", cap.msg)
	}
}

func TestNotificationEmail_RendersSubjectAndLink(t *testing.T) {
	s, cap := newCapturingSender()
	if err := s.Send(context.Background(), ncontracts.EmailMessage{
		To: "al@acme.com", Subject: "New message", Link: "https://app.test/conversations/1",
	}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if !strings.Contains(cap.msg, "Subject: New message") || !strings.Contains(cap.msg, "https://app.test/conversations/1") {
		t.Errorf("notification email missing subject/link:\n%s", cap.msg)
	}
}

func TestDeliver_FailsWhenNotConfigured(t *testing.T) {
	s := NewSMTPSender(Config{}, nil) // no host
	err := s.SendVerification(context.Background(), authcontracts.AccountEmail{To: "x@y.com", Link: "l"})
	if err == nil {
		t.Fatal("expected an error when SMTP is not configured (no silent drop)")
	}
}
