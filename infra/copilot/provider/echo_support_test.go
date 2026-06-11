package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
)

// Echo is a deterministic, dependency-free AIProvider used ONLY by tests (it
// lives in a _test.go file and is never compiled into production). It produces
// shaped output for each action from the already policy-filtered context, so
// tests that need an AIProvider can avoid an HTTP call.
type Echo struct{}

// NewEcho builds the test echo provider.
func NewEcho() *Echo { return &Echo{} }

// Name implements contracts.AIProvider.
func (e *Echo) Name() string { return "echo" }

// Infer implements contracts.AIProvider.
func (e *Echo) Infer(_ context.Context, req contracts.Request) (contracts.Response, error) {
	pc := req.Context
	tokensIn := wordCount(renderContext(pc))

	var text string
	var categories []string

	switch req.Action {
	case entity.ActionSuggestReply:
		text = e.suggestReply(pc)
	case entity.ActionSummarize:
		text = e.summarize(pc)
	case entity.ActionClassify:
		categories = e.classify(pc)
		text = "classified as: " + strings.Join(categories, ", ")
	case entity.ActionNextAction:
		text = e.nextAction(pc)
	default:
		text = "no-op"
	}

	return contracts.Response{
		Text:         text,
		Categories:   categories,
		TokensInput:  tokensIn,
		TokensOutput: wordCount(text),
	}, nil
}

func (e *Echo) suggestReply(pc contracts.PromptContext) string {
	last := lastCustomerTurn(pc)
	var b strings.Builder
	b.WriteString("Hi! ")
	if last != "" {
		b.WriteString("Thanks for your message. ")
		b.WriteString("Regarding \"")
		b.WriteString(truncate(last, 80))
		b.WriteString("\", we're looking into it and will help you right away.")
	} else {
		b.WriteString("How can we help you today?")
	}
	if pc.Monitoring != nil && pc.Monitoring.Summary != "" {
		b.WriteString(" (status: " + pc.Monitoring.Summary + ")")
	}
	if pc.Instruction != "" {
		b.WriteString(" [" + pc.Instruction + "]")
	}
	return b.String()
}

func (e *Echo) summarize(pc contracts.PromptContext) string {
	customer, agent := 0, 0
	for _, t := range pc.Transcript {
		switch t.Role {
		case "customer":
			customer++
		case "agent", "automation":
			agent++
		}
	}
	parts := []string{
		fmt.Sprintf("Conversation on %s with %d customer and %d agent messages.", pc.Channel, customer, agent),
	}
	if pc.Customer != nil && pc.Customer.Name != "" {
		parts = append(parts, "Customer: "+pc.Customer.Name+".")
	}
	if first := firstCustomerTurn(pc); first != "" {
		parts = append(parts, "Opened with: "+truncate(first, 100))
	}
	if pc.Financial != nil && pc.Financial.Summary != "" {
		parts = append(parts, "Financial: "+pc.Financial.Summary)
	}
	return strings.Join(parts, " ")
}

func (e *Echo) classify(pc contracts.PromptContext) []string {
	categories := parseCategories(pc.Instruction)
	if len(categories) == 0 {
		return nil
	}
	// Pick the category whose name best overlaps the transcript, else the first.
	text := strings.ToLower(renderTranscript(pc))
	best, bestScore := categories[0], -1
	for _, c := range categories {
		score := strings.Count(text, strings.ToLower(strings.TrimSpace(c)))
		if score > bestScore {
			best, bestScore = c, score
		}
	}
	return []string{strings.TrimSpace(best)}
}

func (e *Echo) nextAction(pc contracts.PromptContext) string {
	if pc.Monitoring != nil && strings.Contains(strings.ToLower(pc.Monitoring.Summary), "offline") {
		return "Open a technical incident: the customer's connection appears offline."
	}
	if pc.Financial != nil && pc.Financial.Summary != "" {
		return "Review the customer's financial status before proceeding."
	}
	if lastCustomerTurn(pc) != "" {
		return "Reply to the customer's last message and confirm resolution."
	}
	return "Greet the customer and ask how you can help."
}

// wordCount is a rough whitespace token estimate used by the test provider.
func wordCount(s string) int {
	if strings.TrimSpace(s) == "" {
		return 0
	}
	return len(strings.Fields(s))
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

var _ contracts.AIProvider = (*Echo)(nil)
