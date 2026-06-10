package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
)

// Echo is a deterministic, dependency-free provider. It produces useful,
// shaped output for each action from the (already policy-filtered) context, so
// the copilot is fully functional in the MVP without any external API.
type Echo struct{}

// NewEcho builds the echo provider.
func NewEcho() *Echo { return &Echo{} }

// Name implements AIProvider.
func (e *Echo) Name() string { return string(entity.ProviderEcho) }

// Infer implements AIProvider.
func (e *Echo) Infer(_ context.Context, req contracts.Request) (contracts.Response, error) {
	pc := req.Context
	tokensIn := estimateTokens(renderContext(pc))

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
		TokensOutput: estimateTokens(text),
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
	// Deterministic mock: pick the category whose name best overlaps the
	// transcript, else the first.
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

var _ contracts.AIProvider = (*Echo)(nil)
