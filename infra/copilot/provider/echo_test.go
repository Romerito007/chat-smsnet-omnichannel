package provider

import (
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
)

// The echo provider is retained for tests only (it is never wired into the
// production registry). These checks keep it functional as a deterministic
// stand-in for tests that need an AIProvider without an HTTP call.
func TestEcho_DeterministicSuggestAndClassify(t *testing.T) {
	e := NewEcho()
	if e.Name() != "echo" {
		t.Fatalf("name = %q", e.Name())
	}
	got, err := e.Infer(t.Context(), contracts.Request{Action: entity.ActionSuggestReply, Context: samplePC()})
	if err != nil || got.Text == "" {
		t.Fatalf("suggest: %v %+v", err, got)
	}

	cl, err := e.Infer(t.Context(), contracts.Request{
		Action:  entity.ActionClassify,
		Context: contracts.PromptContext{Instruction: "categories: billing, technical", Transcript: samplePC().Transcript},
	})
	if err != nil || len(cl.Categories) == 0 {
		t.Fatalf("classify: %v %+v", err, cl)
	}
}
