package provider

import (
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
)

// fullSystemPrompt is the system message a provider sends: the fixed action prompt
// FIRST (guarantees the base behavior — same language, concise, output shape), then
// the assistant's free-text persona/conduct APPENDED after a blank line. The
// persona adds segment context; it never replaces the base.
func fullSystemPrompt(req contracts.Request) string {
	base := systemPrompt(req.Action)
	if extra := strings.TrimSpace(req.SystemInstructions); extra != "" {
		return base + "\n\n" + extra
	}
	return base
}

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
// string that the providers send as the user message. Only sections present in
// the context (i.e. allowed by the tenant's privacy policy) are rendered.
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
