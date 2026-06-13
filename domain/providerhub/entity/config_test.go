package entity

import "testing"

// expectedISPs is the SMSNET-supported catalog this build promises the front:
// per slug, the credential keys (order matters), the supported actions and the
// search-by modes. It mirrors the SMSNET Integrations documentation and guards
// against accidental drift in ISPCatalog.
var expectedISPs = []struct {
	slug       string
	credKeys   []string
	actions    []ISPAction
	searchBy   []string
	hasLib     bool
	hasChamado bool
}{
	{"altarede", []string{"altarede_host", "altarede_token", "altarede_appkey"}, actStd, byDoc, true, false},
	{"beesweb", []string{"beesweb_host", "beesweb_email", "beesweb_password"}, actStd, byDoc, true, false},
	{"hubsoft", []string{"hubsoft_host", "hubsoft_client_id", "hubsoft_client_secret", "hubsoft_username", "hubsoft_password"}, actStd, byDocPhone, true, false},
	{"ispcloud", []string{"ispcloud_host", "ispcloud_token"}, actStd, byDoc, true, false},
	{"ispcontrollr", []string{"ispcontrollr_host", "ispcontrollr_usuario", "ispcontrollr_senha"}, actStd, byDoc, true, false},
	{"ispfy", []string{"ispfy_host", "ispfy_token"}, actStd, byDocPhone, true, false},
	{"ixcsoft", []string{"ixcsoft_host", "ixcsoft_token"}, actFull, byDocPhone, true, true},
	{"mikweb", []string{"mikweb_host", "mikweb_token"}, actStd, byDoc, true, false},
	{"mkauth", []string{"mkauth_host", "mkauth_token"}, actFull, byDocPhone, true, true},
	{"mksolutions", []string{"mksolutions_host", "mksolutions_token", "mksolutions_password"}, actStd, byDoc, true, false},
	{"netcontrol", []string{"netcontrol_host", "netcontrol_client_id", "netcontrol_client_secret"}, actStd, byDoc, true, false},
	{"radiusnet", []string{"radiusnet_host", "radiusnet_rtoken"}, actStd, byDoc, true, false},
	{"rbfull", []string{"rbfull_host", "rbfull_token"}, actStd, byDoc, true, false},
	{"rbxsoft", []string{"rbxsoft_host", "rbxsoft_token", "rbxsoft_appkey"}, actStd, byDoc, true, false},
	{"receitanet", []string{"receitanet_host", "receitanet_token"}, actFull, byDocPhone, true, true},
	{"sgmcloud", []string{"sgmcloud_host", "sgmcloud_token"}, actStd, byDoc, true, false},
	{"sgpnet", []string{"sgpnet_host", "sgpnet_token"}, actFull, byDocPhone, true, true},
	{"topsapp", []string{"topsapp_host", "topsapp_identificador", "topsapp_usuario", "topsapp_senha"}, actStd, byDoc, true, false},
	{"whmcs", []string{"whmcs_host", "whmcs_identifier", "whmcs_secret"}, actWHMCS, byEmail, false, true},
}

func hasAction(actions []ISPAction, a ISPAction) bool {
	for _, x := range actions {
		if x == a {
			return true
		}
	}
	return false
}

func TestISPCatalog_Has19ISPsMatchingTheDoc(t *testing.T) {
	if len(ISPCatalog) != 19 {
		t.Fatalf("catalog has %d ISPs, want 19", len(ISPCatalog))
	}
	bySlug := map[string]ISPDescriptor{}
	for _, d := range ISPCatalog {
		if _, dup := bySlug[d.Slug]; dup {
			t.Fatalf("duplicate slug in catalog: %q", d.Slug)
		}
		bySlug[d.Slug] = d
	}

	for _, want := range expectedISPs {
		d, ok := bySlug[want.slug]
		if !ok {
			t.Errorf("catalog missing ISP %q", want.slug)
			continue
		}
		if d.Label == "" {
			t.Errorf("%s: empty label", want.slug)
		}
		// Credential keys (exact set and order — the SMSNET config.<key> names).
		if len(d.Credentials) != len(want.credKeys) {
			t.Errorf("%s: %d credential fields, want %d", want.slug, len(d.Credentials), len(want.credKeys))
		} else {
			for i, k := range want.credKeys {
				if d.Credentials[i].Key != k {
					t.Errorf("%s: credential[%d] = %q, want %q", want.slug, i, d.Credentials[i].Key, k)
				}
			}
		}
		// Action support: liberacao on all but whmcs; chamado only on the doc's set.
		if got := hasAction(d.Actions, ActionLiberacao); got != want.hasLib {
			t.Errorf("%s: liberacao support = %v, want %v", want.slug, got, want.hasLib)
		}
		if got := hasAction(d.Actions, ActionChamado); got != want.hasChamado {
			t.Errorf("%s: chamado support = %v, want %v", want.slug, got, want.hasChamado)
		}
		// cliente/planos/empresa are supported by every ISP.
		for _, a := range []ISPAction{ActionCliente, ActionPlanos, ActionEmpresa} {
			if !hasAction(d.Actions, a) {
				t.Errorf("%s: missing always-supported action %q", want.slug, a)
			}
		}
		if len(d.SearchBy) != len(want.searchBy) {
			t.Errorf("%s: search_by = %v, want %v", want.slug, d.SearchBy, want.searchBy)
		}
	}
}

func TestISPCatalog_SecretFlagsMarkOnlySensitiveFields(t *testing.T) {
	publicSuffixes := map[string]bool{
		"host": true, "email": true, "usuario": true, "username": true,
		"client_id": true, "identificador": true, "identifier": true,
	}
	for _, d := range ISPCatalog {
		for _, c := range d.Credentials {
			suffix := c.Key
			if i := lastUnderscore(c.Key); i >= 0 {
				suffix = c.Key[i+1:]
			}
			// Reconstruct the logical name after the slug prefix for multi-word keys.
			logical := c.Key
			if i := firstUnderscore(c.Key); i >= 0 {
				logical = c.Key[i+1:]
			}
			isPublic := publicSuffixes[suffix] || publicSuffixes[logical]
			if isPublic && c.Secret {
				t.Errorf("%s: field %q marked secret but is a public field", d.Slug, c.Key)
			}
			if !isPublic && !c.Secret {
				t.Errorf("%s: field %q is sensitive but not marked secret", d.Slug, c.Key)
			}
			if c.Label == "" {
				t.Errorf("%s: field %q has empty label", d.Slug, c.Key)
			}
		}
	}
}

func firstUnderscore(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '_' {
			return i
		}
	}
	return -1
}

func lastUnderscore(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '_' {
			return i
		}
	}
	return -1
}

func TestISPCatalogVersion_IsSet(t *testing.T) {
	if ISPCatalogVersion == "" {
		t.Fatal("ISPCatalogVersion must be set so the front can cache the catalog")
	}
}

func TestIsKnownISPType(t *testing.T) {
	// Every catalog slug is accepted.
	for _, d := range ISPCatalog {
		if !IsKnownISPType(d.Slug) {
			t.Errorf("catalog slug %q rejected by IsKnownISPType", d.Slug)
		}
	}
	// Legacy aliases stay accepted for already-stored configs.
	for _, legacy := range []string{ISPVoalle, ISPSGP} {
		if !IsKnownISPType(legacy) {
			t.Errorf("legacy slug %q must remain accepted", legacy)
		}
	}
	// Unknown slugs are rejected (no free-form isp_type).
	for _, bad := range []string{"", "outro", "nope", "HUBSOFT", " ixcsoft"} {
		if IsKnownISPType(bad) {
			t.Errorf("unknown slug %q must be rejected", bad)
		}
	}
}

func TestKnownISPTypes_CoversCatalogPlusLegacy(t *testing.T) {
	if len(KnownISPTypes) != len(ISPCatalog)+2 {
		t.Fatalf("KnownISPTypes has %d entries, want %d (catalog + 2 legacy)", len(KnownISPTypes), len(ISPCatalog)+2)
	}
}
