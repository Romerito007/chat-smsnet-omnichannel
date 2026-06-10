// Package provider holds the AI copilot adapters (echo/mock, openai, gemini,
// anthropic) behind a common port. The copilot domain calls the port; async
// inference is driven by the Asynq `ai` queue (ai.suggest / ai.summarize /
// ai.classify) when latency allows.
//
// Placeholder in the foundation; the echo (mock) adapter is the reference
// implementation added first.
package provider
