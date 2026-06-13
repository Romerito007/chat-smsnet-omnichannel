package service

import (
	"fmt"
	"math/rand"
	"net/mail"
	"strings"

	"github.com/nyaruka/phonenumbers"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
)

// defaultPhoneRegion is the region assumed when a phone has no country code
// (DDI). Brazilian numbers without "+55" are parsed as BR.
const defaultPhoneRegion = "BR"

// NormalizePhoneE164 parses a phone (with libphonenumber, default region BR) and
// returns it formatted as E.164 (e.g. +5544941049474). ok is false when the
// number is empty or not a valid number. Exported so the seed writes exactly the
// same shape the create/update validation accepts (one source of truth).
func NormalizePhoneE164(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	num, err := phonenumbers.Parse(raw, defaultPhoneRegion)
	if err != nil {
		return "", false
	}
	if !phonenumbers.IsValidNumber(num) {
		return "", false
	}
	return phonenumbers.Format(num, phonenumbers.E164), true
}

// digitsOnly keeps only the decimal digits of s.
func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// NormalizeDocument validates a CPF (11 digits) or CNPJ (14 digits) by its check
// digits and returns the digits-only form (no mask). An empty document is valid
// (optional field) and returns ("", true). ok is false for an invalid document.
// Exported so the seed can validate the documents it generates with the very same
// rule the create/update path enforces.
func NormalizeDocument(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", true
	}
	d := digitsOnly(raw)
	switch len(d) {
	case 11:
		if isValidCPF(d) {
			return d, true
		}
	case 14:
		if isValidCNPJ(d) {
			return d, true
		}
	}
	return "", false
}

// normalizeEmailStrict trims+lowercases and validates the email format. An empty
// email is valid (optional) and returns ("", true).
func normalizeEmailStrict(raw string) (string, bool) {
	e := strings.ToLower(strings.TrimSpace(raw))
	if e == "" {
		return "", true
	}
	addr, err := mail.ParseAddress(e)
	if err != nil || addr.Address != e {
		return "", false
	}
	return e, true
}

// isValidCPF validates the two CPF check digits and rejects all-equal sequences.
func isValidCPF(d string) bool {
	if len(d) != 11 || allEqual(d) {
		return false
	}
	return cpfDigit(d, 9, 10) == int(d[9]-'0') && cpfDigit(d, 10, 11) == int(d[10]-'0')
}

// GenerateValidCPF returns an 11-digit CPF (digits only) with correct check
// digits, drawn from rng. It computes the DVs with cpfDigit — the same function
// isValidCPF/NormalizeDocument use — so a generated CPF is valid by construction.
// Used by the demo seed so seeded contacts pass the document validation.
func GenerateValidCPF(rng *rand.Rand) string {
	d := make([]byte, 11)
	for {
		for i := 0; i < 9; i++ {
			d[i] = byte('0' + rng.Intn(10))
		}
		base := string(d[:9])
		d[9] = byte('0' + cpfDigit(base, 9, 10))
		d[10] = byte('0' + cpfDigit(string(d[:10]), 10, 11))
		out := string(d)
		if !allEqual(out) { // reject the degenerate all-equal CPFs
			return out
		}
	}
}

// cpfDigit computes a CPF check digit over the first n digits with the standard
// descending weights.
func cpfDigit(d string, n, weight int) int {
	sum := 0
	for i := 0; i < n; i++ {
		sum += int(d[i]-'0') * (weight - i)
	}
	r := (sum * 10) % 11
	if r == 10 {
		r = 0
	}
	return r
}

// isValidCNPJ validates the two CNPJ check digits and rejects all-equal sequences.
func isValidCNPJ(d string) bool {
	if len(d) != 14 || allEqual(d) {
		return false
	}
	return cnpjDigit(d, 12) == int(d[12]-'0') && cnpjDigit(d, 13) == int(d[13]-'0')
}

// cnpjDigit computes a CNPJ check digit over the first n digits with the standard
// cyclic weights.
func cnpjDigit(d string, n int) int {
	weights := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	if n == 13 {
		weights = []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	}
	sum := 0
	for i := 0; i < n; i++ {
		sum += int(d[i]-'0') * weights[i]
	}
	r := sum % 11
	if r < 2 {
		return 0
	}
	return 11 - r
}

// allEqual reports whether every byte of s is the same (e.g. "00000000000"),
// which passes the check-digit math but is never a real document.
func allEqual(s string) bool {
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			return false
		}
	}
	return len(s) > 0
}

// validIdentityChannel reports whether ch is a supported contact identity channel.
func validIdentityChannel(ch string) bool {
	return entity.IsSupportedIdentityChannel(strings.ToLower(strings.TrimSpace(ch)))
}

// normalizePhonesValidated normalizes every phone to E.164 (default region BR),
// dropping empties and duplicates while preserving order. Any invalid number is
// reported in the returned details map keyed by its position (phones[i]); when
// details is non-empty the caller must reject the write with a validation error.
func normalizePhonesValidated(phones []string) ([]string, map[string]any) {
	out := make([]string, 0, len(phones))
	seen := make(map[string]struct{}, len(phones))
	details := map[string]any{}
	for i, p := range phones {
		if strings.TrimSpace(p) == "" {
			continue
		}
		e164, ok := NormalizePhoneE164(p)
		if !ok {
			details[fmt.Sprintf("phones[%d]", i)] = "is not a valid phone number"
			continue
		}
		if _, dup := seen[e164]; dup {
			continue
		}
		seen[e164] = struct{}{}
		out = append(out, e164)
	}
	return out, details
}

// normalizeIdentitiesValidated lowercases the channel, trims the external id and
// validates the channel against the supported set, dropping incomplete pairs and
// duplicates. Unsupported channels are reported by position; non-empty details
// means the caller must reject the write.
func normalizeIdentitiesValidated(ids []contracts.ExternalIdentity) ([]entity.ChannelIdentity, map[string]any) {
	out := make([]entity.ChannelIdentity, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	details := map[string]any{}
	for i, id := range ids {
		ch := strings.ToLower(strings.TrimSpace(id.Channel))
		ex := strings.TrimSpace(id.ExternalID)
		if ch != "" && !validIdentityChannel(ch) {
			details[fmt.Sprintf("external_ids[%d].channel", i)] = "is not a supported channel"
			continue
		}
		if ch == "" || ex == "" {
			continue // incomplete pair: dropped, mirroring the prior behavior
		}
		key := ch + "\x00" + ex
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, entity.ChannelIdentity{Channel: ch, ExternalID: ex})
	}
	return out, details
}
