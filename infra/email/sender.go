// Package email holds the email sender used by notifications
// (notification.email on the Asynq queue). In the MVP it logs the message; the
// structure is ready for a real SMTP transport. Messages are privacy-safe by
// construction — they carry only a subject and a deep link, never the
// notification body.
package email

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/notifications/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Sender is the MVP email sender: it logs the outbound message. Swap the body of
// Send for an SMTP/provider call without touching the domain.
type Sender struct {
	logger shared.Logger
	from   string
}

// NewSender builds the sender.
func NewSender(logger shared.Logger, from string) *Sender {
	return &Sender{logger: logger, from: from}
}

// Send "delivers" the email. It deliberately logs only non-sensitive fields.
func (s *Sender) Send(_ context.Context, msg contracts.EmailMessage) error {
	if s.logger != nil {
		s.logger.Info("notification email sent",
			"to", msg.To,
			"from", s.from,
			"subject", msg.Subject,
			"link", msg.Link,
			"preview", msg.Preview,
		)
	}
	return nil
}

var _ contracts.EmailSender = (*Sender)(nil)
