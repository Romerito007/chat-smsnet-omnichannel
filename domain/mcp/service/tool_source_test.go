package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// fakeBridge resolves a fixed tool source and a configurable write gate.
type fakeBridge struct {
	source     contracts.ToolSource
	allowWrite bool
}

func (b fakeBridge) ToolSource(context.Context, string) (contracts.ToolSource, error) {
	return b.source, nil
}
func (b fakeBridge) AllowServer(_ context.Context, _, _ string, write bool) (bool, error) {
	if write {
		return b.allowWrite, nil
	}
	return true, nil
}
func (b fakeBridge) Decorate(_ context.Context, in contracts.DecorateInput) (map[string]any, error) {
	return in.Args, nil
}

// sourceFixture builds a ToolService over a custom tenant server + the two SMSNET
// servers, with a fixed bridge source.
func sourceFixture(src contracts.ToolSource, allowWrite bool) *ToolService {
	custom := &entity.ServerConnection{ID: "srv-custom", TenantID: "t1", Name: "Custom OLT", Kind: entity.KindRead, Enabled: true}
	smsnetRead := &entity.ServerConnection{ID: "env-smsnet-consultas", TenantID: "t1", Name: entity.SMSNETConsultasName, Kind: entity.KindRead, Enabled: true}
	smsnetWrite := &entity.ServerConnection{ID: "env-smsnet-operacoes", TenantID: "t1", Name: entity.SMSNETOperacoesName, Kind: entity.KindWrite, Enabled: true}
	servers := &fakeServers{byID: map[string]*entity.ServerConnection{
		"srv-custom": custom, "env-smsnet-consultas": smsnetRead, "env-smsnet-operacoes": smsnetWrite,
	}}
	client := &fakeClient{tools: map[string][]contracts.ToolSpec{
		"srv-custom":           {{Name: "olt_query", Description: "olt"}},
		"env-smsnet-consultas": {{Name: "consultar_cliente", Description: "lookup"}},
		"env-smsnet-operacoes": {{Name: "liberar_acesso", Description: "release"}},
	}}
	conv := &conventity.Conversation{ID: "conv1", TenantID: "t1", ChannelID: "ch1"}
	svc := NewToolService(servers, &fakeApprovals{byID: map[string]*entity.Approval{}}, &fakeCallLogs{}, &fakeConvRepo{conv: conv}, client, shared.NoopPublisher{}, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	svc.SetISPBridge(fakeBridge{source: src, allowWrite: allowWrite})
	return svc
}

func sessionToolNames(t *testing.T, svc *ToolService) map[string]bool {
	t.Helper()
	sess, err := svc.OpenToolSession(ctxWith(authz.IntegrationRead), "conv1")
	if err != nil {
		t.Fatalf("open session: %v", err)
	}
	names := map[string]bool{}
	for _, d := range sess.Tools() {
		names[d.Name] = true
	}
	return names
}

func TestOpenToolSession_MCPSourceExposesOnlyThatServer(t *testing.T) {
	svc := sourceFixture(contracts.ToolSource{Kind: contracts.ToolSourceMCP, MCPServerID: "srv-custom"}, false)
	names := sessionToolNames(t, svc)
	if !names["olt_query"] {
		t.Errorf("the assistant's MCP server tool must be exposed: %v", names)
	}
	if names["consultar_cliente"] || names["liberar_acesso"] {
		t.Errorf("SMSNET tools must NOT appear when the source is a custom MCP server: %v", names)
	}
}

func TestOpenToolSession_ISPSourceExposesSMSNETOnly(t *testing.T) {
	// ISP source, profile without write action → SMSNET read only, no write, no custom.
	svc := sourceFixture(contracts.ToolSource{Kind: contracts.ToolSourceISP}, false)
	names := sessionToolNames(t, svc)
	if !names["consultar_cliente"] {
		t.Errorf("SMSNET read tool must be exposed for the ISP source: %v", names)
	}
	if names["liberar_acesso"] {
		t.Errorf("SMSNET write tool must be gated out when the profile has no write action: %v", names)
	}
	if names["olt_query"] {
		t.Errorf("an unlinked custom MCP server must NOT appear for the ISP source: %v", names)
	}
}

func TestOpenToolSession_NoneSourceExposesNoTools(t *testing.T) {
	svc := sourceFixture(contracts.ToolSource{Kind: contracts.ToolSourceNone}, false)
	names := sessionToolNames(t, svc)
	if len(names) != 0 {
		t.Errorf("no assistant source must expose zero external tools, got %v", names)
	}
}
