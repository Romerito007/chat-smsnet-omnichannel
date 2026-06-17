package provider

import (
	"strings"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
)

// agent_chat uses an AGENT-facing system prompt (helps the agent, does not write to
// the customer).
func TestSystemPrompt_AgentChat_IsAgentFacing(t *testing.T) {
	sp := systemPrompt(entity.ActionAgentChat)
	if !strings.Contains(sp, "ATENDENTE") || strings.Contains(strings.ToLower(sp), "draft a") {
		t.Errorf("agent_chat prompt must be agent-facing, got: %q", sp)
	}
	// suggest_reply stays customer-facing (unchanged).
	if !strings.Contains(systemPrompt(entity.ActionSuggestReply), "reply to the customer") {
		t.Error("suggest_reply must remain customer-facing")
	}
}

// The customer block is rendered with LABELED fields (so the model can extract the
// CPF), and the agent↔assistant side chat is rendered as its own block.
func TestRenderContext_LabelsAndAgentChat(t *testing.T) {
	pc := contracts.PromptContext{
		Channel:     "whatsapp",
		Customer:    &contracts.CustomerInfo{Name: "Pedro", Phone: "5544999", Document: "37779130819"},
		Transcript:  []contracts.Turn{{Role: "customer", Text: "oi"}},
		AgentChat:   []contracts.Turn{{Role: "agent", Text: "como está a fatura?"}, {Role: "assistant", Text: "vencida há 3 dias"}},
		Instruction: "e o plano dele?",
	}
	out := renderContext(pc)
	for _, want := range []string{"CPF/CNPJ: 37779130819", "Cliente: Pedro", "Telefone: 5544999",
		"Conversa atendente↔assistente", "agent: como está a fatura?", "assistant: vencida há 3 dias",
		"Instruction: e o plano dele?"} {
		if !strings.Contains(out, want) {
			t.Errorf("renderContext missing %q in:\n%s", want, out)
		}
	}
}

// An empty customer document is omitted (no bare "CPF/CNPJ:" line).
func TestRenderContext_OmitsEmptyFields(t *testing.T) {
	out := renderContext(contracts.PromptContext{Customer: &contracts.CustomerInfo{Name: "Ana"}})
	if strings.Contains(out, "CPF/CNPJ:") || strings.Contains(out, "Telefone:") {
		t.Errorf("empty fields must be omitted, got:\n%s", out)
	}
	if !strings.Contains(out, "Cliente: Ana") {
		t.Error("present field must be rendered")
	}
}
