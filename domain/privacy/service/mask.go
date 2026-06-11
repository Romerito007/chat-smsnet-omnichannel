package service

import (
	"regexp"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/repository"
)

// redacted is the placeholder substituted for masked PII.
const redacted = "[REDACTED]"

// anonymizedName is the replacement display name written to an anonymized
// contact. The row and its id are kept so conversations/metrics stay linked.
const anonymizedName = "Contato Anonimizado"

var (
	// emailRe matches e-mail addresses.
	emailRe = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	// numberRe matches phone/CPF/CNPJ-like runs: an optional '+', then a digit,
	// 7+ digit-or-separator characters, ending in a digit. It errs toward
	// over-redaction, which is the safe direction for PII masking.
	numberRe = regexp.MustCompile(`\+?\d[\d\s().\-]{7,}\d`)
)

// contactPII holds the literal values to scrub from message text, taken from the
// contact before it is anonymized.
type contactPII struct {
	Name     string
	Phone    string
	Document string
}

func piiOf(c repository.ContactData) contactPII {
	return contactPII{Name: c.Name, Phone: c.Phone, Document: c.Document}
}

// maskPII removes PII from a single message body per the policy: the contact's
// own name/phone/document literals first, then any e-mail address and any
// phone/document-like number pattern. It is deterministic and side-effect free.
func maskPII(text string, c contactPII) string {
	out := text
	// Literal values specific to this contact (case-insensitive). Skip very short
	// values to avoid masking common substrings.
	for _, lit := range []string{c.Phone, c.Document, c.Name} {
		lit = strings.TrimSpace(lit)
		if len(lit) < 3 {
			continue
		}
		out = replaceFold(out, lit, redacted)
	}
	out = emailRe.ReplaceAllString(out, redacted)
	out = numberRe.ReplaceAllString(out, redacted)
	return out
}

// replaceFold replaces every case-insensitive occurrence of old in s with new.
func replaceFold(s, old, new string) string {
	if old == "" {
		return s
	}
	var b strings.Builder
	lowerS := strings.ToLower(s)
	lowerOld := strings.ToLower(old)
	for {
		i := strings.Index(lowerS, lowerOld)
		if i < 0 {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:i])
		b.WriteString(new)
		s = s[i+len(old):]
		lowerS = lowerS[i+len(lowerOld):]
	}
	return b.String()
}
