package service

import (
	"strings"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/repository"
)

func TestMaskPII(t *testing.T) {
	pii := piiOf(repository.ContactData{Name: "Maria Souza", Phone: "11988887777", Document: "529.982.247-25"})
	cases := []struct {
		name    string
		in      string
		mustGo  []string // substrings that must be removed
		keepLen bool
	}{
		{"literal name", "Falei com Maria Souza ontem", []string{"Maria Souza"}, false},
		{"literal phone", "ligue 11988887777", []string{"11988887777"}, false},
		{"document", "CPF 529.982.247-25", []string{"529.982.247-25"}, false},
		{"email", "envie para joao@exemplo.com.br", []string{"joao@exemplo.com.br"}, false},
		{"foreign phone pattern", "número +55 (21) 3333-4444", []string{"3333-4444"}, false},
		{"clean text unchanged", "obrigado pelo contato", nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := maskPII(c.in, pii)
			for _, s := range c.mustGo {
				if strings.Contains(out, s) {
					t.Errorf("PII %q still present in %q", s, out)
				}
			}
			if c.keepLen && out != c.in {
				t.Errorf("clean text was altered: %q -> %q", c.in, out)
			}
		})
	}
}
