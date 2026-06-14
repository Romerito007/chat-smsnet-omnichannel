package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

type fakeServerUsage struct {
	inUse  bool
	usedBy string
}

func (f fakeServerUsage) IsMCPServerInUse(context.Context, string) (bool, string, error) {
	return f.inUse, f.usedBy, nil
}

func TestServerDelete_BlockedWhenReferencedByAssistant(t *testing.T) {
	servers := &fakeServers{byID: map[string]*entity.ServerConnection{
		"srv1": {ID: "srv1", TenantID: "t1", Name: "Custom", Kind: entity.KindRead, Enabled: true},
	}}
	svc := NewServerService(servers, &fakeClient{}, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	ctx := shared.WithTenant(context.Background(), "t1")

	svc.SetUsageChecker(fakeServerUsage{inUse: true, usedBy: "Atendimento"})
	if err := svc.Delete(ctx, "srv1"); apperror.From(err).Code != apperror.CodeConflict {
		t.Fatalf("deleting a referenced MCP server must be a 409 conflict, got %v", err)
	}
	if _, ok := servers.byID["srv1"]; !ok {
		t.Error("the server must NOT be deleted while in use")
	}

	// Once no assistant references it, the delete proceeds.
	svc.SetUsageChecker(fakeServerUsage{inUse: false})
	if err := svc.Delete(ctx, "srv1"); err != nil {
		t.Fatalf("an unreferenced server must delete cleanly: %v", err)
	}
	if _, ok := servers.byID["srv1"]; ok {
		t.Error("the server should be gone after a successful delete")
	}
}
