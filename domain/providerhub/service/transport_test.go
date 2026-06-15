package service

import (
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/contracts"
	phentity "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
)

func TestProfileCreate_RejectsEmptyTransports(t *testing.T) {
	svc := newProfileSvc(newFakeProfileRepo())
	_, err := svc.Create(profileCtx(), contracts.CreateProfile{
		Label: "A", ISPType: "ixcsoft", Credentials: validCreds(), Transports: nil,
	})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("empty transports must be a validation error, got %v", err)
	}
}

func TestProfileCreate_RejectsUnknownTransport(t *testing.T) {
	svc := newProfileSvc(newFakeProfileRepo())
	_, err := svc.Create(profileCtx(), contracts.CreateProfile{
		Label: "A", ISPType: "ixcsoft", Credentials: validCreds(), Transports: []string{"http", "carrier-pigeon"},
	})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("an unknown transport must be a validation error, got %v", err)
	}
}

func TestProfileCreate_NormalizesTransportOrder(t *testing.T) {
	svc := newProfileSvc(newFakeProfileRepo())
	p, err := svc.Create(profileCtx(), contracts.CreateProfile{
		Label: "A", ISPType: "ixcsoft", Credentials: validCreds(), Transports: []string{"mcp", "http", "mcp"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(p.Transports) != 2 || p.Transports[0] != phentity.TransportHTTP || p.Transports[1] != phentity.TransportMCP {
		t.Errorf("transports not normalized/deduped to [http mcp], got %v", p.Transports)
	}
}

// TestResolve_MCPOnlyProfileBlocksManualSearch guards the gating: the manual search
// (HTTP gateway) must not resolve a profile that only enables the mcp transport.
func TestResolve_MCPOnlyProfileBlocksManualSearch(t *testing.T) {
	repo := newFakeProfileRepo()
	repo.byID["only-mcp"] = &phentity.ISPProfile{
		ID: "only-mcp", TenantID: "t1", Label: "M", ISPType: phentity.ISPIXCSoft,
		IsDefault: true, Enabled: true, Transports: []string{phentity.TransportMCP},
	}
	r := NewISPResolver(repo)

	// Explicit selection of a mcp-only profile → clear conflict (needs http).
	if _, err := r.Resolve(profileCtx(), "only-mcp"); apperror.From(err).Code != apperror.CodeConflict {
		t.Fatalf("explicit mcp-only profile must be a conflict for manual search, got %v", err)
	}
	// Auto-resolution skips the mcp-only default → no http-enabled profile → none.
	rr, err := r.Resolve(profileCtx(), "")
	if err != nil {
		t.Fatalf("auto resolve: %v", err)
	}
	if rr.Status != ResolveNone {
		t.Errorf("a mcp-only default must not serve the manual search; want ResolveNone, got %v", rr.Status)
	}
}

func TestResolve_HTTPProfileResolves(t *testing.T) {
	repo := newFakeProfileRepo()
	repo.byID["http1"] = &phentity.ISPProfile{
		ID: "http1", TenantID: "t1", Label: "H", ISPType: phentity.ISPIXCSoft,
		IsDefault: true, Enabled: true, Transports: []string{phentity.TransportHTTP, phentity.TransportMCP},
	}
	r := NewISPResolver(repo)
	rr, err := r.Resolve(profileCtx(), "")
	if err != nil || rr.Status != ResolveOK || rr.Profile.ID != "http1" {
		t.Fatalf("an http-enabled default must resolve OK, got status=%v err=%v", rr.Status, err)
	}
}
