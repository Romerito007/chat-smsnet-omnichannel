package provider

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
)

// Gemini is the Google Gemini generateContent adapter. It is stateless; the API
// key and optional base URL arrive per-request from the tenant's AIConfig.
type Gemini struct {
	defaultBaseURL string
	defaultModel   string
}

// NewGemini builds the adapter.
func NewGemini() *Gemini {
	return &Gemini{defaultBaseURL: "https://generativelanguage.googleapis.com", defaultModel: "gemini-1.5-flash"}
}

// Name implements contracts.AIProvider.
func (g *Gemini) Name() string { return string(entity.ProviderGemini) }

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	GenerationConfig  geminiGenConfig `json:"generationConfig"`
}

type geminiGenConfig struct {
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
}

// Infer implements contracts.AIProvider against generateContent.
func (g *Gemini) Infer(ctx context.Context, req contracts.Request) (contracts.Response, error) {
	if req.APIKey == "" {
		return contracts.Response{}, notConfigured(entity.ProviderGemini)
	}
	base := orDefault(req.BaseURL, g.defaultBaseURL)
	model := orDefault(req.Model, g.defaultModel)
	endpoint := base + "/v1beta/models/" + url.PathEscape(model) + ":generateContent?key=" + url.QueryEscape(req.APIKey)

	payload := geminiRequest{
		SystemInstruction: &geminiContent{Parts: []geminiPart{{Text: systemPrompt(req.Action)}}},
		Contents:          []geminiContent{{Role: "user", Parts: []geminiPart{{Text: renderContext(req.Context)}}}},
		GenerationConfig:  geminiGenConfig{Temperature: req.Temperature, MaxOutputTokens: maxTokensOr(req.MaxTokens, 0)},
	}

	body, err := postJSON(ctx, endpoint, nil, payload)
	if err != nil {
		return contracts.Response{}, err
	}

	var parsed geminiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return contracts.Response{}, err
	}
	out := contracts.Response{
		TokensInput:  parsed.UsageMetadata.PromptTokenCount,
		TokensOutput: parsed.UsageMetadata.CandidatesTokenCount,
	}
	if len(parsed.Candidates) > 0 {
		var text strings.Builder
		for _, p := range parsed.Candidates[0].Content.Parts {
			text.WriteString(p.Text)
		}
		out.Text = text.String()
	}
	if req.Action == entity.ActionClassify {
		out.Categories = classifyCategories(out.Text, req.Context.Instruction)
	}
	return out, nil
}

var _ contracts.AIProvider = (*Gemini)(nil)
