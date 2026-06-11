package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	infraemail "github.com/romerito007/chat-smsnet-omnichannel/infra/email"
)

// EmailSender builds the real SMTP sender shared by the notifications email
// channel and the auth account mailer (verification, invite, password reset).
func EmailSender(c *container.Container) *infraemail.SMTPSender {
	return infraemail.NewSMTPSender(infraemail.Config{
		Host:     c.Config.Email.Host,
		Port:     c.Config.Email.Port,
		Username: c.Config.Email.Username,
		Password: c.Config.Email.Password,
		From:     c.Config.Email.From,
		TLSMode:  c.Config.Email.TLSMode,
	}, c.Logger)
}
