package service

import (
	"context"
	"testing"
	"time"

	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// scriptedProvider returns a fixed sequence of responses, one per Infer call.
type scriptedProvider struct {
	responses []contracts.Response
	calls     int
}

func (p *scriptedProvider) Name() string { return "scripted" }
func (p *scriptedProvider) Infer(_ context.Context, _ contracts.Request) (contracts.Response, error) {
	i := p.calls
	if i >= len(p.responses) {
		i = len(p.responses) - 1
	}
	p.calls++
	return p.responses[i], nil
}

// fakeSession is a copilot ToolSession with scripted read/write tools.
type fakeSession struct {
	tools        []contracts.ToolDefinition
	writes       map[string]bool
	actions      map[string]string // tool name → ISP action slug (for write-mode resolution)
	executed     []string
	autoExecuted []string
	proposed     []string
}

func (s *fakeSession) Tools() []contracts.ToolDefinition { return s.tools }
func (s *fakeSession) IsWrite(name string) bool          { return s.writes[name] }
func (s *fakeSession) WriteAction(name string) string    { return s.actions[name] }
func (s *fakeSession) ExecuteRead(_ context.Context, name, _ string) (string, error) {
	s.executed = append(s.executed, name)
	return "RESULT for " + name, nil
}
func (s *fakeSession) ExecuteWrite(_ context.Context, name, _ string) (string, error) {
	s.autoExecuted = append(s.autoExecuted, name)
	return "AUTO RESULT for " + name, nil
}
func (s *fakeSession) ProposeWrite(_ context.Context, name, _ string) (contracts.ProposedAction, error) {
	s.proposed = append(s.proposed, name)
	return contracts.ProposedAction{ApprovalID: "appr-1", Server: "ops", Tool: name, Args: map[string]any{}}, nil
}

type fakeBroker struct{ session *fakeSession }

func (b *fakeBroker) OpenToolSession(context.Context, string) (contracts.ToolSession, error) {
	return b.session, nil
}

func agenticService(prov *scriptedProvider, session *fakeSession) (*Service, *fakeLogs) {
	logs := &fakeLogs{}
	clock := fixedClock{t: time.Unix(1700000000, 0).UTC()}
	cfg := &entity.AIConfig{ID: "cfg1", TenantID: "t1", Provider: entity.ProviderOpenAI, Model: "m", APIKey: "test-key", Enabled: true}
	cfgSvc := NewConfigService(&fakeConfigRepo{cfg: cfg}, clock)
	convs := &fakeConvRepo{items: map[string]*conventity.Conversation{"conv1": conv()}}
	svc := NewService(cfgSvc, logs, convs, builderWithAllSources(sampleMessages()), spyResolver{p: prov}, shared.NoopPublisher{}, clock)
	svc.SetToolBroker(&fakeBroker{session: session})
	return svc, logs
}

func TestAgentic_ReadToolLoop(t *testing.T) {
	prov := &scriptedProvider{responses: []contracts.Response{
		{ToolCalls: []contracts.ToolCall{{ID: "c1", Name: "consultar_cliente", Arguments: `{"cpf":"1"}`}}, TokensInput: 3, TokensOutput: 1},
		{Text: "The customer is active.", TokensInput: 5, TokensOutput: 4},
	}}
	session := &fakeSession{
		tools:  []contracts.ToolDefinition{{Name: "consultar_cliente", ReadOnly: true}},
		writes: map[string]bool{},
	}
	svc, _ := agenticService(prov, session)

	res, err := svc.SuggestReply(allCtx(), contracts.SuggestReplyInput{ConversationID: "conv1"})
	if err != nil {
		t.Fatalf("suggest: %v", err)
	}
	if res.Text != "The customer is active." {
		t.Errorf("expected final answer from the loop, got %q", res.Text)
	}
	if len(session.executed) != 1 || session.executed[0] != "consultar_cliente" {
		t.Errorf("read tool should have executed in the loop: %v", session.executed)
	}
	if len(res.ProposedActions) != 0 {
		t.Errorf("a read-only loop must propose nothing: %+v", res.ProposedActions)
	}
	if prov.calls != 2 {
		t.Errorf("expected 2 model turns (call + final), got %d", prov.calls)
	}
}

// When the assistant set the operation to "automatico", the write executes inline
// (no proposal, no approval) and the loop continues to a final answer.
func TestAgentic_WriteAutomaticExecutesInline(t *testing.T) {
	prov := &scriptedProvider{responses: []contracts.Response{
		{ToolCalls: []contracts.ToolCall{{ID: "c1", Name: "smsnet_liberacao", Arguments: `{"id_cliente":"c1"}`}}},
		{Text: "Acesso liberado."},
	}}
	session := &fakeSession{
		tools:   []contracts.ToolDefinition{{Name: "smsnet_liberacao", ReadOnly: false}},
		writes:  map[string]bool{"smsnet_liberacao": true},
		actions: map[string]string{"smsnet_liberacao": "liberacao"},
	}
	svc, _ := agenticService(prov, session)
	behavior := entity.Behavior{WriteModes: map[string]string{"liberacao": entity.WriteModeAuto}}

	resp, proposed, err := svc.agenticLoop(allCtx(), "conv1", prov, contracts.Request{}, behavior)
	if err != nil {
		t.Fatalf("loop: %v", err)
	}
	if resp.Text != "Acesso liberado." {
		t.Errorf("expected final answer after auto write, got %q", resp.Text)
	}
	if len(session.autoExecuted) != 1 || session.autoExecuted[0] != "smsnet_liberacao" {
		t.Errorf("automatic write should execute inline: %v", session.autoExecuted)
	}
	if len(session.proposed) != 0 {
		t.Errorf("automatic write must NOT be proposed: %v", session.proposed)
	}
	if len(proposed) != 0 {
		t.Errorf("no proposal expected for an automatic write: %+v", proposed)
	}
}

// A write op left at the default mode (no WriteModes entry) is still proposed even
// when other ops are automatic — the safe default holds per operation.
func TestAgentic_WriteDefaultsToApproval(t *testing.T) {
	prov := &scriptedProvider{responses: []contracts.Response{
		{ToolCalls: []contracts.ToolCall{{ID: "c1", Name: "smsnet_chamado", Arguments: `{}`}}},
		{Text: "unused"},
	}}
	session := &fakeSession{
		tools:   []contracts.ToolDefinition{{Name: "smsnet_chamado", ReadOnly: false}},
		writes:  map[string]bool{"smsnet_chamado": true},
		actions: map[string]string{"smsnet_chamado": "chamado"},
	}
	svc, _ := agenticService(prov, session)
	// liberacao is automatic, but chamado has no mode → must default to approval.
	behavior := entity.Behavior{WriteModes: map[string]string{"liberacao": entity.WriteModeAuto}}

	_, proposed, err := svc.agenticLoop(allCtx(), "conv1", prov, contracts.Request{}, behavior)
	if err != nil {
		t.Fatalf("loop: %v", err)
	}
	if len(session.autoExecuted) != 0 {
		t.Errorf("chamado must not auto-execute: %v", session.autoExecuted)
	}
	if len(proposed) != 1 || proposed[0].Tool != "smsnet_chamado" {
		t.Fatalf("chamado must be proposed for approval: %+v", proposed)
	}
}

func TestAgentic_WriteIsProposedNeverExecuted(t *testing.T) {
	prov := &scriptedProvider{responses: []contracts.Response{
		{ToolCalls: []contracts.ToolCall{{ID: "c1", Name: "liberar_acesso", Arguments: `{"id":"c1"}`}}},
		{Text: "unused"},
	}}
	session := &fakeSession{
		tools:  []contracts.ToolDefinition{{Name: "liberar_acesso", ReadOnly: false}},
		writes: map[string]bool{"liberar_acesso": true},
	}
	svc, _ := agenticService(prov, session)

	res, err := svc.SuggestReply(allCtx(), contracts.SuggestReplyInput{ConversationID: "conv1"})
	if err != nil {
		t.Fatalf("suggest: %v", err)
	}
	if len(res.ProposedActions) != 1 || res.ProposedActions[0].Tool != "liberar_acesso" {
		t.Fatalf("write tool must be proposed: %+v", res.ProposedActions)
	}
	if !res.RequiresApproval {
		t.Error("a proposed write must require approval regardless of the config flag")
	}
	if len(session.executed) != 0 {
		t.Errorf("a write tool must NEVER execute automatically: %v", session.executed)
	}
	if len(session.proposed) != 1 {
		t.Errorf("the write should be recorded as a proposal: %v", session.proposed)
	}
}

// AgentChat (agent-facing Q&A) runs the SAME agentic loop as suggest_reply, so the
// model can call tools to answer the agent's question.
func TestAgentChat_RunsAgenticLoopWithHistory(t *testing.T) {
	prov := &scriptedProvider{responses: []contracts.Response{
		{ToolCalls: []contracts.ToolCall{{ID: "c1", Name: "consultar_cliente", Arguments: `{}`}}},
		{Text: "A fatura está vencida há 3 dias, R$94,90."},
	}}
	session := &fakeSession{
		tools:  []contracts.ToolDefinition{{Name: "consultar_cliente", ReadOnly: true}},
		writes: map[string]bool{},
	}
	svc, _ := agenticService(prov, session)

	res, err := svc.AgentChat(allCtx(), contracts.AgentChatInput{
		ConversationID: "conv1",
		Instruction:    "como está a fatura desse cliente?",
		History:        []contracts.AgentTurn{{Role: "agent", Text: "oi"}, {Role: "assistant", Text: "olá"}},
	})
	if err != nil {
		t.Fatalf("agent chat: %v", err)
	}
	if res.Action != "agent_chat" {
		t.Errorf("action = %q, want agent_chat", res.Action)
	}
	if res.Text != "A fatura está vencida há 3 dias, R$94,90." {
		t.Errorf("unexpected answer: %q", res.Text)
	}
	if len(session.executed) != 1 || session.executed[0] != "consultar_cliente" {
		t.Errorf("agent_chat must use tools (agentic loop): %v", session.executed)
	}
}
