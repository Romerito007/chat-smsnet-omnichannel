package providerhub

import (
	"encoding/json"
	"strings"
	"testing"

	phentity "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
)

func TestNewCatalogResponse_MirrorsEntityCatalog(t *testing.T) {
	resp := NewCatalogResponse()

	if resp.Version != phentity.ISPCatalogVersion {
		t.Errorf("version = %q, want %q", resp.Version, phentity.ISPCatalogVersion)
	}
	if len(resp.ISPs) != len(phentity.ISPCatalog) {
		t.Fatalf("catalog has %d ISPs, want %d", len(resp.ISPs), len(phentity.ISPCatalog))
	}

	bySlug := map[string]ISPCatalogEntry{}
	for _, e := range resp.ISPs {
		bySlug[e.Slug] = e
	}

	for _, d := range phentity.ISPCatalog {
		e, ok := bySlug[d.Slug]
		if !ok {
			t.Errorf("missing ISP %q in DTO", d.Slug)
			continue
		}
		if e.Label != d.Label {
			t.Errorf("%s: label = %q, want %q", d.Slug, e.Label, d.Label)
		}
		if len(e.Credentials) != len(d.Credentials) {
			t.Errorf("%s: %d credentials, want %d", d.Slug, len(e.Credentials), len(d.Credentials))
		} else {
			for i, c := range d.Credentials {
				if e.Credentials[i].Key != c.Key || e.Credentials[i].Secret != c.Secret {
					t.Errorf("%s: credential[%d] = %+v, want key %q secret %v", d.Slug, i, e.Credentials[i], c.Key, c.Secret)
				}
			}
		}
		if len(e.Actions) != len(d.Actions) {
			t.Errorf("%s: %d actions, want %d", d.Slug, len(e.Actions), len(d.Actions))
		}
	}
}

func TestNewCatalogResponse_WHMCSHasChamadoButNoLiberacao(t *testing.T) {
	resp := NewCatalogResponse()
	var whmcs *ISPCatalogEntry
	for i := range resp.ISPs {
		if resp.ISPs[i].Slug == "whmcs" {
			whmcs = &resp.ISPs[i]
			break
		}
	}
	if whmcs == nil {
		t.Fatal("whmcs missing from catalog")
	}
	has := func(actions []string, a string) bool {
		for _, x := range actions {
			if x == a {
				return true
			}
		}
		return false
	}
	if has(whmcs.Actions, "liberacao") {
		t.Errorf("whmcs must not support liberacao: %v", whmcs.Actions)
	}
	if !has(whmcs.Actions, "chamado") {
		t.Errorf("whmcs must support chamado: %v", whmcs.Actions)
	}
	if len(whmcs.SearchBy) != 1 || whmcs.SearchBy[0] != "email" {
		t.Errorf("whmcs search_by = %v, want [email]", whmcs.SearchBy)
	}
}

func TestNewCatalogResponse_SerializesWithExpectedKeys(t *testing.T) {
	b, err := json.Marshal(NewCatalogResponse())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m["version"]; !ok {
		t.Error("missing version key")
	}
	isps, ok := m["isps"].([]any)
	if !ok || len(isps) == 0 {
		t.Fatalf("isps not a non-empty array: %T", m["isps"])
	}
	first, _ := isps[0].(map[string]any)
	for _, k := range []string{"slug", "label", "credentials", "actions", "search_by"} {
		if _, ok := first[k]; !ok {
			t.Errorf("ISP entry missing key %q", k)
		}
	}
}

func TestNewProfileResponse_MasksCredentialsAndAddsActions(t *testing.T) {
	p := &phentity.ISPProfile{
		ID: "p1", TenantID: "t1", Label: "IXC matriz", ISPType: "ixcsoft",
		Credentials: map[string]string{"ixcsoft_host": "h", "ixcsoft_token": "secret-token"},
		IsDefault:   true, Enabled: true,
	}
	resp := NewProfileResponse(p)
	// Credential values must never appear; only sorted keys.
	if len(resp.CredentialKeys) != 2 || resp.CredentialKeys[0] != "ixcsoft_host" || resp.CredentialKeys[1] != "ixcsoft_token" {
		t.Errorf("credential keys wrong/unsorted: %v", resp.CredentialKeys)
	}
	b, _ := json.Marshal(resp)
	if strings.Contains(string(b), "secret-token") {
		t.Fatalf("credential VALUE leaked into the response: %s", b)
	}
	// actions[] derived from the catalog (ixcsoft supports chamado + liberacao).
	has := func(a string) bool {
		for _, x := range resp.Actions {
			if x == a {
				return true
			}
		}
		return false
	}
	if !has("chamado") || !has("liberacao") || !has("cliente") {
		t.Errorf("ixcsoft actions incomplete: %v", resp.Actions)
	}
}
