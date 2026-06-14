package provider

import (
	"strings"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
)

func TestFullSystemPrompt_AppendsPersonaAfterFixedPrompt(t *testing.T) {
	base := systemPrompt(entity.ActionSuggestReply)

	// No persona → exactly the fixed action prompt.
	if got := fullSystemPrompt(contracts.Request{Action: entity.ActionSuggestReply}); got != base {
		t.Errorf("no persona must return the fixed prompt unchanged, got %q", got)
	}

	// With persona → fixed prompt FIRST, then the persona after a blank line.
	persona := "Você é o assistente da loja de sapatos. Tom descontraído."
	got := fullSystemPrompt(contracts.Request{Action: entity.ActionSuggestReply, SystemInstructions: persona})
	if !strings.HasPrefix(got, base) {
		t.Errorf("the fixed action prompt must come first, got %q", got)
	}
	if !strings.HasSuffix(got, persona) || !strings.Contains(got, "\n\n") {
		t.Errorf("the persona must be appended after a blank line, got %q", got)
	}
}
