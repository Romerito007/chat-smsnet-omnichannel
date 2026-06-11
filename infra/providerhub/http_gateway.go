// Package providerhub is the HTTP client for the tenant's smsnet-integrations
// API. It talks ONLY to that API (never to IXC/SGP/MK/Voalle directly), performs
// no caching and persists nothing. Each call builds the body
// { botId, <route fields>, config: { type, <isp_credentials> } } and sends the
// x-api-key header; the response envelope { status, message, data } is mapped to
// normalized DTOs or domain errors.
package providerhub

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	phcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/contracts"
	phentity "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
	infrahttp "github.com/romerito007/chat-smsnet-omnichannel/infra/http_client"
)

// friendly is the user-safe message for any non-success external outcome.
const friendly = "o serviço do provedor está indisponível, tente novamente"

// envelope statuses returned by the smsnet-integrations API.
const (
	statusSuccess    = "success"
	statusNotFound   = "not_found"
	statusNeedsInput = "needs_input"
)

// Gateway is the HTTP implementation of the providerhub gateway.
type Gateway struct {
	client *http.Client
}

// NewGateway builds the gateway.
func NewGateway() *Gateway {
	return &Gateway{client: infrahttp.New(20 * time.Second)}
}

// envelope is the smsnet-integrations response wrapper.
type envelope struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// ConsultarCliente looks up a customer; maps needs_input to a selection result.
func (g *Gateway) ConsultarCliente(ctx context.Context, cfg *phentity.ProviderIntegrationConfig, req phcontracts.ConsultaClienteRequest) (phcontracts.ClienteResult, error) {
	body := g.baseBody(cfg)
	if req.CpfCnpj != "" {
		body["cpfcnpj"] = req.CpfCnpj
	}
	if req.Phone != "" {
		body["phone"] = req.Phone
	}
	if req.Email != "" {
		body["email"] = req.Email
	}
	if req.IDCliente != "" {
		body["idCliente"] = req.IDCliente
	}

	env, err := g.call(ctx, cfg, "/consultar-cliente", body)
	if err != nil {
		return phcontracts.ClienteResult{}, err
	}
	switch env.Status {
	case statusSuccess:
		var cli phcontracts.Cliente
		if err := json.Unmarshal(env.Data, &cli); err != nil {
			return phcontracts.ClienteResult{}, apperror.Integration(friendly)
		}
		return phcontracts.ClienteResult{Cliente: &cli}, nil
	case statusNeedsInput:
		var d struct {
			Options []phcontracts.ContratoOption `json:"options"`
		}
		_ = json.Unmarshal(env.Data, &d)
		return phcontracts.ClienteResult{NeedsSelection: true, Options: d.Options}, nil
	case statusNotFound:
		return phcontracts.ClienteResult{}, apperror.NotFound("cliente não localizado")
	default:
		return phcontracts.ClienteResult{}, apperror.Integration(friendly)
	}
}

// ListarPlanos returns the tenant's plans (from fixed config data when present).
func (g *Gateway) ListarPlanos(ctx context.Context, cfg *phentity.ProviderIntegrationConfig) ([]phcontracts.Plano, error) {
	if len(cfg.Options.DadosPlanos) > 0 {
		return planosFromFixed(cfg.Options.DadosPlanos), nil
	}
	env, err := g.call(ctx, cfg, "/listar-planos", g.baseBody(cfg))
	if err != nil {
		return nil, err
	}
	if env.Status != statusSuccess {
		return nil, mapNonSuccess(env.Status, "planos não localizados")
	}
	var d struct {
		Planos []phcontracts.Plano `json:"planos"`
	}
	if err := json.Unmarshal(env.Data, &d); err != nil {
		return nil, apperror.Integration(friendly)
	}
	return d.Planos, nil
}

// DadosEmpresa returns the company profile (from fixed config data when present).
func (g *Gateway) DadosEmpresa(ctx context.Context, cfg *phentity.ProviderIntegrationConfig) (phcontracts.Empresa, error) {
	if len(cfg.Options.DadosEmpresa) > 0 {
		return empresaFromFixed(cfg.Options.DadosEmpresa), nil
	}
	env, err := g.call(ctx, cfg, "/dados-empresa", g.baseBody(cfg))
	if err != nil {
		return phcontracts.Empresa{}, err
	}
	if env.Status != statusSuccess {
		return phcontracts.Empresa{}, mapNonSuccess(env.Status, "empresa não localizada")
	}
	var out phcontracts.Empresa
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return phcontracts.Empresa{}, apperror.Integration(friendly)
	}
	return out, nil
}

// LiberarAcesso performs a trust-unlock for a contract.
func (g *Gateway) LiberarAcesso(ctx context.Context, cfg *phentity.ProviderIntegrationConfig, idCliente string) (phcontracts.Liberacao, error) {
	body := g.baseBody(cfg)
	body["idCliente"] = idCliente
	env, err := g.call(ctx, cfg, "/liberar-acesso", body)
	if err != nil {
		return phcontracts.Liberacao{}, err
	}
	if env.Status != statusSuccess {
		return phcontracts.Liberacao{}, mapNonSuccess(env.Status, "contrato não localizado")
	}
	var out phcontracts.Liberacao
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return phcontracts.Liberacao{}, apperror.Integration(friendly)
	}
	return out, nil
}

// AbrirChamado opens a support ticket for a contract.
func (g *Gateway) AbrirChamado(ctx context.Context, cfg *phentity.ProviderIntegrationConfig, idCliente, subject, message string) (phcontracts.Chamado, error) {
	body := g.baseBody(cfg)
	body["idCliente"] = idCliente
	body["subject"] = subject
	body["message"] = message
	env, err := g.call(ctx, cfg, "/abrir-chamado", body)
	if err != nil {
		return phcontracts.Chamado{}, err
	}
	if env.Status != statusSuccess {
		return phcontracts.Chamado{}, mapNonSuccess(env.Status, "contrato não localizado")
	}
	var out phcontracts.Chamado
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return phcontracts.Chamado{}, apperror.Integration(friendly)
	}
	return out, nil
}

// Ping validates connectivity + credentials by hitting dados-empresa.
func (g *Gateway) Ping(ctx context.Context, cfg *phentity.ProviderIntegrationConfig) error {
	_, err := g.call(ctx, cfg, "/dados-empresa", g.baseBody(cfg))
	return err
}

// baseBody builds the common request body: botId + config{type, credentials, opts}.
func (g *Gateway) baseBody(cfg *phentity.ProviderIntegrationConfig) map[string]any {
	config := map[string]any{
		"type":                            cfg.ISPType,
		"usa_pegar_fatura_atrasada":       cfg.Options.UsaPegarFaturaAtrasada,
		"usa_extrair_linha_digitavel_pdf": cfg.Options.UsaExtrairLinhaDigitavelPDF,
	}
	for k, v := range cfg.ISPCredentials {
		config[k] = v
	}
	return map[string]any{
		"botId":  cfg.ResolveBotID(),
		"config": config,
	}
}

// call POSTs the body with x-api-key and decodes the envelope. Transport, HTTP or
// decode failures map to a friendly integration error (the "fallback" path).
func (g *Gateway) call(ctx context.Context, cfg *phentity.ProviderIntegrationConfig, path string, body map[string]any) (*envelope, error) {
	timeout := time.Duration(cfg.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	raw, err := json.Marshal(body)
	if err != nil {
		return nil, apperror.Integration(friendly)
	}
	url := strings.TrimRight(cfg.SMSNetBaseURL, "/") + path
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, apperror.Integration(friendly)
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.SMSNetAPIKey != "" {
		req.Header.Set("x-api-key", cfg.SMSNetAPIKey)
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, apperror.Integration(friendly)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return nil, apperror.Integration(friendly)
	}
	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, apperror.Integration(friendly)
	}
	return &env, nil
}

// mapNonSuccess maps a non-success envelope status to a domain error.
func mapNonSuccess(status, notFoundMsg string) error {
	if status == statusNotFound {
		return apperror.NotFound(notFoundMsg)
	}
	return apperror.Integration(friendly)
}

func planosFromFixed(fixed map[string]any) []phcontracts.Plano {
	raw, err := json.Marshal(fixed)
	if err != nil {
		return nil
	}
	var d struct {
		Planos []phcontracts.Plano `json:"planos"`
	}
	if json.Unmarshal(raw, &d) == nil && len(d.Planos) > 0 {
		return d.Planos
	}
	var arr []phcontracts.Plano
	_ = json.Unmarshal(raw, &arr)
	return arr
}

func empresaFromFixed(fixed map[string]any) phcontracts.Empresa {
	raw, err := json.Marshal(fixed)
	if err != nil {
		return phcontracts.Empresa{}
	}
	var out phcontracts.Empresa
	_ = json.Unmarshal(raw, &out)
	return out
}

var _ phcontracts.Gateway = (*Gateway)(nil)
