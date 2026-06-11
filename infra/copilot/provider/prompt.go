package provider

import (
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
)

// systemPrompt returns the action-specific system instruction shared by every
// real provider. It steers the model toward a concise, support-appropriate
// answer; the (policy-filtered) conversation context is supplied as user content.
func systemPrompt(action entity.Action) string {
	switch action {
	case entity.ActionSuggestReply:
		return "You are a customer-support copilot. Draft a concise, friendly reply to the customer's latest message, in the same language as the customer. Output only the reply text, with no preamble."
	case entity.ActionSummarize:
		return "You are a customer-support copilot. Summarize the following conversation in a few clear sentences. Output only the summary."
	case entity.ActionClassify:
		return "You are a customer-support classifier. Classify the conversation into exactly one of the categories listed in the instruction. Respond with only the chosen category name, nothing else."
	case entity.ActionNextAction:
		return "You are a customer-support copilot. Recommend the single best next action for the agent as one short imperative sentence. Output only that sentence."
	default:
		return "You are a helpful customer-support copilot."
	}
}

// renderContext renders the full policy-filtered context into a single prompt
// string. Real providers (openai/gemini/anthropic) use this to build their
// request body; the echo provider uses it for token estimation. Only sections
// present in the context (i.e. allowed by policy) are rendered.
func renderContext(pc contracts.PromptContext) string {
	var b strings.Builder
	b.WriteString("Channel: ")
	b.WriteString(pc.Channel)
	b.WriteString("\n")
	if pc.Customer != nil {
		b.WriteString("Customer: ")
		b.WriteString(strings.TrimSpace(pc.Customer.Name + " " + pc.Customer.Phone + " " + pc.Customer.Document))
		b.WriteString("\n")
	}
	if pc.Financial != nil {
		b.WriteString("Financial: ")
		b.WriteString(pc.Financial.Summary)
		b.WriteString("\n")
	}
	if pc.Monitoring != nil {
		b.WriteString("Monitoring: ")
		b.WriteString(pc.Monitoring.Summary)
		b.WriteString("\n")
	}
	b.WriteString("Transcript:\n")
	b.WriteString(renderTranscript(pc))
	if pc.Instruction != "" {
		b.WriteString("\nInstruction: ")
		b.WriteString(pc.Instruction)
	}
	return b.String()
}

func renderTranscript(pc contracts.PromptContext) string {
	var b strings.Builder
	for _, t := range pc.Transcript {
		b.WriteString(t.Role)
		b.WriteString(": ")
		b.WriteString(t.Text)
		b.WriteString("\n")
	}
	return b.String()
}

func lastCustomerTurn(pc contracts.PromptContext) string {
	for i := len(pc.Transcript) - 1; i >= 0; i-- {
		if pc.Transcript[i].Role == "customer" {
			return pc.Transcript[i].Text
		}
	}
	return ""
}

func firstCustomerTurn(pc contracts.PromptContext) string {
	for _, t := range pc.Transcript {
		if t.Role == "customer" {
			return t.Text
		}
	}
	return ""
}

func parseCategories(instruction string) []string {
	const prefix = "categories:"
	idx := strings.Index(instruction, prefix)
	if idx < 0 {
		return nil
	}
	raw := strings.Split(instruction[idx+len(prefix):], ",")
	out := make([]string, 0, len(raw))
	for _, c := range raw {
		if c = strings.TrimSpace(c); c != "" {
			out = append(out, c)
		}
	}
	return out
}

// estimateTokens is a rough whitespace-based token estimate (~1 token per word),
// good enough for mock cost/observability.
func estimateTokens(s string) int {
	if strings.TrimSpace(s) == "" {
		return 0
	}
	return len(strings.Fields(s))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
