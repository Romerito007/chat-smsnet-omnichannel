package service

import (
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	centity "github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
	phentity "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
)

// assistantSvcWithProfile builds an AssistantService whose profiles repo holds one
// ISP profile "isp1" enabling the given transports.
func assistantSvcWithProfile(transports ...string) *AssistantService {
	arepo := &fakeAssistantRepo{byID: map[string]*centity.Assistant{}}
	profiles := &fakeProfileRepo{byID: map[string]*phentity.ISPProfile{
		"isp1": {ID: "isp1", TenantID: "t1", Label: "P", ISPType: phentity.ISPIXCSoft, Enabled: true, Transports: transports},
	}}
	return NewAssistantService(arepo, profiles, fakeChannelsRepo{}, fakeMCPServers{known: map[string]bool{}}, nil)
}

func TestAssistantTransport_RequiredWhenProfileEnablesBoth(t *testing.T) {
	svc := assistantSvcWithProfile(phentity.TransportHTTP, phentity.TransportMCP)
	_, err := svc.Create(aCtx(), CreateAssistant{Name: "A", ISPProfileID: "isp1"}) // no transport
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("transport must be required when the profile enables both, got %v", err)
	}
}

func TestAssistantTransport_DefaultsToSoleTransport(t *testing.T) {
	svc := assistantSvcWithProfile(phentity.TransportMCP) // only mcp
	a, err := svc.Create(aCtx(), CreateAssistant{Name: "A", ISPProfileID: "isp1"})
	if err != nil {
		t.Fatalf("a single-transport profile should default the transport: %v", err)
	}
	if a.Transport != phentity.TransportMCP {
		t.Errorf("transport should default to the sole enabled one, got %q", a.Transport)
	}
}

func TestAssistantTransport_RejectsTransportNotEnabled(t *testing.T) {
	svc := assistantSvcWithProfile(phentity.TransportMCP) // http not enabled
	_, err := svc.Create(aCtx(), CreateAssistant{Name: "A", ISPProfileID: "isp1", Transport: phentity.TransportHTTP})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("selecting a transport the profile does not enable must 422, got %v", err)
	}
}

func TestAssistantTransport_AcceptsEnabledSelection(t *testing.T) {
	svc := assistantSvcWithProfile(phentity.TransportHTTP, phentity.TransportMCP)
	a, err := svc.Create(aCtx(), CreateAssistant{Name: "A", ISPProfileID: "isp1", Transport: phentity.TransportMCP})
	if err != nil {
		t.Fatalf("an enabled transport must be accepted: %v", err)
	}
	if a.Transport != phentity.TransportMCP {
		t.Errorf("transport not stored, got %q", a.Transport)
	}
}

func TestAssistantTransport_RejectedWithoutISPProfile(t *testing.T) {
	svc := assistantSvcWithProfile(phentity.TransportMCP)
	_, err := svc.Create(aCtx(), CreateAssistant{Name: "A", Transport: phentity.TransportMCP}) // no ISP
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("transport without an isp_profile_id must 422, got %v", err)
	}
}
