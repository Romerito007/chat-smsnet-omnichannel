package providerhub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	phcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/contracts"
	phentity "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
)

// mockServer routes by request path + the cpfcnpj field to drive the four
// envelope outcomes, and records the last request for header/body assertions.
type capture struct {
	apiKey string
	path   string
	body   map[string]any
}

func newMock(t *testing.T) (*httptest.Server, *capture) {
	t.Helper()
	cap := &capture{}
	mux := http.NewServeMux()
	mux.HandleFunc("/consultar-cliente", func(w http.ResponseWriter, r *http.Request) {
		cap.apiKey = r.Header.Get("x-api-key")
		cap.path = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&cap.body)
		// Second step: a chosen contract (idCliente) returns that specific contract.
		if id, ok := cap.body["idCliente"].(string); ok && id != "" {
			writeJSON(w, `{"status":"success","data":{"nome":"Cliente Contrato `+id+`","cpfcnpj":"123","contrato_status_display":"Ativo `+id+`"}}`)
			return
		}
		switch cap.body["cpfcnpj"] {
		case "000": // not found
			writeJSON(w, `{"status":"not_found","message":"sem cadastro"}`)
		case "555": // needs input (multiple contracts)
			writeJSON(w, `{"status":"needs_input","data":{"options":[{"id_cliente":"A","label":"Contrato A"},{"id_cliente":"B","label":"Contrato B"}]}}`)
		case "boom": // fallback
			writeJSON(w, `{"status":"fallback","message":"erro upstream"}`)
		default: // success
			writeJSON(w, `{"status":"success","data":{"nome":"João Silva","cpfcnpj":"123","contrato_status_display":"Ativo","valor_check_out":99.9,"faturas":[{"valor":50.0,"vencimento":"2026-07-01","linha_digitavel":"34191..."}]}}`)
		}
	})
	mux.HandleFunc("/listar-planos", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"status":"success","data":{"planos":[{"nome":"100MB","valor":99.9,"velocidade":"100 Mbps"}]}}`)
	})
	mux.HandleFunc("/dados-empresa", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"status":"success","data":{"nome":"Provedor X","cnpj":"00.000/0001"}}`)
	})
	mux.HandleFunc("/liberar-acesso", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&cap.body)
		writeJSON(w, `{"status":"success","data":{"liberado":true,"protocolo":"LB-1","liberado_ate":"2026-07-10"}}`)
	})
	mux.HandleFunc("/abrir-chamado", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"status":"success","data":{"protocolo":"CH-1","msg":"aberto"}}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, cap
}

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

func cfgFor(url string) *phentity.ProviderIntegrationConfig {
	return &phentity.ProviderIntegrationConfig{
		TenantID:       "t1",
		SMSNetBaseURL:  url,
		SMSNetAPIKey:   "k-123",
		ISPType:        phentity.ISPHubsoft,
		ISPCredentials: map[string]string{"hubsoft_host": "h", "hubsoft_client_id": "cid"},
		BotID:          "bot-9",
		TimeoutMs:      2000,
	}
}

func TestGateway_ConsultarCliente_Success(t *testing.T) {
	srv, cap := newMock(t)
	g := NewGateway()
	res, err := g.ConsultarCliente(context.Background(), cfgFor(srv.URL), phcontracts.ConsultaClienteRequest{CpfCnpj: "123"})
	if err != nil {
		t.Fatalf("consulta: %v", err)
	}
	if res.Cliente == nil || res.Cliente.Nome != "João Silva" || res.Cliente.ValorCheckOut != 99.9 {
		t.Fatalf("unexpected cliente: %+v", res.Cliente)
	}
	if len(res.Cliente.Faturas) != 1 || res.Cliente.Faturas[0].LinhaDigitavel != "34191..." {
		t.Errorf("faturas not decoded: %+v", res.Cliente.Faturas)
	}
	// Header + body shape: x-api-key + { botId, config:{ type, credentials } }.
	if cap.apiKey != "k-123" {
		t.Errorf("x-api-key not sent: %q", cap.apiKey)
	}
	if cap.body["botId"] != "bot-9" {
		t.Errorf("botId not sent: %v", cap.body["botId"])
	}
	conf, _ := cap.body["config"].(map[string]any)
	if conf["type"] != phentity.ISPHubsoft || conf["hubsoft_host"] != "h" {
		t.Errorf("config body not built: %+v", conf)
	}
}

func TestGateway_ConsultarCliente_NotFound(t *testing.T) {
	srv, _ := newMock(t)
	g := NewGateway()
	_, err := g.ConsultarCliente(context.Background(), cfgFor(srv.URL), phcontracts.ConsultaClienteRequest{CpfCnpj: "000"})
	if apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("expected not_found, got %v", err)
	}
}

func TestGateway_ConsultarCliente_NeedsInput(t *testing.T) {
	srv, _ := newMock(t)
	g := NewGateway()
	res, err := g.ConsultarCliente(context.Background(), cfgFor(srv.URL), phcontracts.ConsultaClienteRequest{CpfCnpj: "555"})
	if err != nil {
		t.Fatalf("consulta: %v", err)
	}
	if !res.NeedsSelection || len(res.Options) != 2 || res.Options[0].IDCliente != "A" {
		t.Fatalf("expected needs_input with options, got %+v", res)
	}
}

// TestGateway_TwoStepSelection covers the acceptance flow: a lookup with multiple
// contracts returns needs_input with options; the follow-up call with the chosen
// idCliente returns that specific contract.
func TestGateway_TwoStepSelection(t *testing.T) {
	srv, _ := newMock(t)
	g := NewGateway()
	cfg := cfgFor(srv.URL)

	step1, err := g.ConsultarCliente(context.Background(), cfg, phcontracts.ConsultaClienteRequest{CpfCnpj: "555"})
	if err != nil || !step1.NeedsSelection || len(step1.Options) != 2 {
		t.Fatalf("step1 needs_input: %v %+v", err, step1)
	}
	chosen := step1.Options[1].IDCliente // "B"

	step2, err := g.ConsultarCliente(context.Background(), cfg, phcontracts.ConsultaClienteRequest{IDCliente: chosen})
	if err != nil {
		t.Fatalf("step2: %v", err)
	}
	if step2.Cliente == nil || step2.Cliente.ContratoStatusDisplay != "Ativo "+chosen {
		t.Fatalf("step2 must return the chosen contract %q, got %+v", chosen, step2.Cliente)
	}
}

func TestGateway_ConsultarCliente_Fallback(t *testing.T) {
	srv, _ := newMock(t)
	g := NewGateway()
	_, err := g.ConsultarCliente(context.Background(), cfgFor(srv.URL), phcontracts.ConsultaClienteRequest{CpfCnpj: "boom"})
	if apperror.From(err).Code != apperror.CodeIntegrationUnavailable {
		t.Fatalf("expected integration error, got %v", err)
	}
}

func TestGateway_HTTPErrorIsFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	g := NewGateway()
	_, err := g.ConsultarCliente(context.Background(), cfgFor(srv.URL), phcontracts.ConsultaClienteRequest{CpfCnpj: "1"})
	if apperror.From(err).Code != apperror.CodeIntegrationUnavailable {
		t.Fatalf("HTTP 500 must map to integration error, got %v", err)
	}
}

func TestGateway_ListarPlanos_And_Empresa(t *testing.T) {
	srv, _ := newMock(t)
	g := NewGateway()
	planos, err := g.ListarPlanos(context.Background(), cfgFor(srv.URL))
	if err != nil || len(planos) != 1 || planos[0].Nome != "100MB" {
		t.Fatalf("planos: %v %+v", err, planos)
	}
	emp, err := g.DadosEmpresa(context.Background(), cfgFor(srv.URL))
	if err != nil || emp.Nome != "Provedor X" {
		t.Fatalf("empresa: %v %+v", err, emp)
	}
}

func TestGateway_LiberarAcesso_And_Chamado(t *testing.T) {
	srv, cap := newMock(t)
	g := NewGateway()
	lib, err := g.LiberarAcesso(context.Background(), cfgFor(srv.URL), "idc-1")
	if err != nil || !lib.Liberado || lib.Protocolo != "LB-1" {
		t.Fatalf("liberacao: %v %+v", err, lib)
	}
	if cap.body["idCliente"] != "idc-1" {
		t.Errorf("idCliente not sent: %v", cap.body["idCliente"])
	}
	ch, err := g.AbrirChamado(context.Background(), cfgFor(srv.URL), "idc-1", "sem net", "detalhe")
	if err != nil || ch.Protocolo != "CH-1" {
		t.Fatalf("chamado: %v %+v", err, ch)
	}
}

func TestGateway_FixedPlanosSkipHTTP(t *testing.T) {
	g := NewGateway()
	cfg := cfgFor("http://127.0.0.1:0") // unreachable; fixed data must avoid the call
	cfg.Options.DadosPlanos = map[string]any{"planos": []map[string]any{{"nome": "Fixo", "valor": 10.0}}}
	planos, err := g.ListarPlanos(context.Background(), cfg)
	if err != nil || len(planos) != 1 || planos[0].Nome != "Fixo" {
		t.Fatalf("fixed planos: %v %+v", err, planos)
	}
}
