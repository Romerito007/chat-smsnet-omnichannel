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
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	phcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/contracts"
	phentity "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
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
	logger shared.Logger
	clock  shared.Clock
}

// NewGateway builds the gateway.
func NewGateway() *Gateway {
	return &Gateway{client: infrahttp.New(20 * time.Second), logger: slog.Default(), clock: shared.SystemClock{}}
}

// SetClock overrides the clock used to derive a fatura's vencida/dias_atraso
// (defaults to the system clock). For tests.
func (g *Gateway) SetClock(c shared.Clock) {
	if c != nil {
		g.clock = c
	}
}

// SetLogger wires the structured logger used to record the REAL cause of a gateway
// failure (status/url/body/transport error) — the client only ever sees the generic
// "serviço indisponível", so without this a misconfigured host/path/key is opaque.
func (g *Gateway) SetLogger(l shared.Logger) {
	if l != nil {
		g.logger = l
	}
}

// envelope is the smsnet-integrations response wrapper. options/selectionType are
// top-level (a needs_input multi-contract response), NOT inside data.
type envelope struct {
	Status        string          `json:"status"`
	Message       string          `json:"message"`
	Data          json.RawMessage `json:"data"`
	Options       []smsnetOption  `json:"options"`
	SelectionType string          `json:"selectionType"`
}

// smsnetOption is one selectable contract on a needs_input response: value is the
// idCliente to resend, label is the human display.
type smsnetOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
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

	env, err := g.call(ctx, cfg, "/cliente", body)
	if err != nil {
		return phcontracts.ClienteResult{}, err
	}
	switch env.Status {
	case statusSuccess:
		var raw smsnetCliente
		if err := json.Unmarshal(env.Data, &raw); err != nil {
			return phcontracts.ClienteResult{}, apperror.Integration(friendly)
		}
		cli := mapCliente(raw, g.clock.Now())
		// Diagnostic: how many invoices the gateway actually returned to us, and the
		// option flags we sent. If faturas_count is 1 while a direct gateway call (no
		// flags) returns more, the gateway is honoring usa_pegar_fatura_atrasada — the
		// backend maps every fatura it receives (no truncation here).
		shared.LoggerFrom(ctx, g.logger).Info("SMSNET_CLIENTE_RESPONSE",
			"faturas_count", len(raw.Faturas),
			"usa_pegar_fatura_atrasada", cfg.Options.UsaPegarFaturaAtrasada,
			"usa_extrair_linha_digitavel_pdf", cfg.Options.UsaExtrairLinhaDigitavelPDF,
			"data_bytes", len(env.Data))
		return phcontracts.ClienteResult{Cliente: &cli}, nil
	case statusNeedsInput:
		opts := make([]phcontracts.ContratoOption, 0, len(env.Options))
		for _, o := range env.Options {
			opts = append(opts, phcontracts.ContratoOption{IDCliente: o.Value, Label: o.Label})
		}
		return phcontracts.ClienteResult{NeedsSelection: true, Options: opts}, nil
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
	env, err := g.call(ctx, cfg, "/planos", g.baseBody(cfg))
	if err != nil {
		return nil, err
	}
	if env.Status != statusSuccess {
		return nil, mapNonSuccess(env.Status, "planos não localizados")
	}
	// SMSNET returns data as the plans array directly ([{descricao, valor}]).
	var raw []smsnetPlano
	if err := json.Unmarshal(env.Data, &raw); err != nil {
		return nil, apperror.Integration(friendly)
	}
	out := make([]phcontracts.Plano, 0, len(raw))
	for _, p := range raw {
		out = append(out, mapPlano(p))
	}
	return out, nil
}

// DadosEmpresa returns the company profile (from fixed config data when present).
func (g *Gateway) DadosEmpresa(ctx context.Context, cfg *phentity.ProviderIntegrationConfig) (phcontracts.Empresa, error) {
	if len(cfg.Options.DadosEmpresa) > 0 {
		return empresaFromFixed(cfg.Options.DadosEmpresa), nil
	}
	env, err := g.call(ctx, cfg, "/empresa", g.baseBody(cfg))
	if err != nil {
		return phcontracts.Empresa{}, err
	}
	if env.Status != statusSuccess {
		return phcontracts.Empresa{}, mapNonSuccess(env.Status, "empresa não localizada")
	}
	var raw smsnetEmpresa
	if err := json.Unmarshal(env.Data, &raw); err != nil {
		return phcontracts.Empresa{}, apperror.Integration(friendly)
	}
	return mapEmpresa(raw), nil
}

// LiberarAcesso performs a trust-unlock for a contract.
func (g *Gateway) LiberarAcesso(ctx context.Context, cfg *phentity.ProviderIntegrationConfig, idCliente string) (phcontracts.Liberacao, error) {
	body := g.baseBody(cfg)
	body["idCliente"] = idCliente
	env, err := g.call(ctx, cfg, "/liberacao", body)
	if err != nil {
		return phcontracts.Liberacao{}, err
	}
	if env.Status != statusSuccess {
		return phcontracts.Liberacao{}, mapNonSuccess(env.Status, "contrato não localizado")
	}
	var raw smsnetLiberacao
	if err := json.Unmarshal(env.Data, &raw); err != nil {
		return phcontracts.Liberacao{}, apperror.Integration(friendly)
	}
	return mapLiberacao(raw), nil
}

// AbrirChamado opens a support ticket for a contract.
func (g *Gateway) AbrirChamado(ctx context.Context, cfg *phentity.ProviderIntegrationConfig, idCliente, subject, message string) (phcontracts.Chamado, error) {
	body := g.baseBody(cfg)
	body["idCliente"] = idCliente
	body["subject"] = subject
	body["message"] = message
	env, err := g.call(ctx, cfg, "/chamado", body)
	if err != nil {
		return phcontracts.Chamado{}, err
	}
	if env.Status != statusSuccess {
		return phcontracts.Chamado{}, mapNonSuccess(env.Status, "contrato não localizado")
	}
	var raw smsnetChamado
	if err := json.Unmarshal(env.Data, &raw); err != nil {
		return phcontracts.Chamado{}, apperror.Integration(friendly)
	}
	return phcontracts.Chamado{Protocolo: raw.Protocolo, Msg: raw.Msg}, nil
}

// Ping validates connectivity + credentials by hitting /empresa.
func (g *Gateway) Ping(ctx context.Context, cfg *phentity.ProviderIntegrationConfig) error {
	_, err := g.call(ctx, cfg, "/empresa", g.baseBody(cfg))
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
	// Forward the idempotency key (set for side-effect calls) so the upstream API
	// can dedup retries — it warns against retrying writes without one.
	if key := phcontracts.IdempotencyKeyFrom(ctx); key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	resp, err := g.client.Do(req)
	if err != nil {
		// Transport failure (DNS/connect/timeout): the host is configured but the
		// request never completed. Log the URL + cause so a wrong host/port/path is
		// diagnosable; the caller only sees the generic friendly message.
		g.logFailure(ctx, "transport", url, cfg, 0, err.Error())
		return nil, apperror.Integration(friendly)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		// Non-2xx: the host answered but rejected the call (wrong path → 404, bad
		// x-api-key → 401/403, upstream error → 5xx). Log status + body snippet.
		g.logFailure(ctx, "http_status", url, cfg, resp.StatusCode, string(snippet))
		return nil, apperror.Integration(friendly)
	}
	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		g.logFailure(ctx, "decode", url, cfg, resp.StatusCode, err.Error())
		return nil, apperror.Integration(friendly)
	}
	return &env, nil
}

// logFailure records the concrete cause of a gateway failure (the api key is never
// logged). request_id/tenant_id come from the context.
func (g *Gateway) logFailure(ctx context.Context, reason, url string, cfg *phentity.ProviderIntegrationConfig, status int, detail string) {
	shared.LoggerFrom(ctx, g.logger).Error("SMSNET_GATEWAY_CALL_FAILED",
		"reason", reason, "url", url, "status", status, "isp_type", cfg.ISPType,
		"base_url", cfg.SMSNetBaseURL, "has_api_key", cfg.SMSNetAPIKey != "", "detail", truncate(detail, 300))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ── SMSNET response shapes (camelCase) → normalized DTOs ──────────────────────
// The smsnet-integrations API returns camelCase keys and some numeric fields as
// strings (e.g. valorCheckOut "99.900000", liberado 1). We decode into these
// internal structs and map to the chat's normalized (snake_case) DTOs.

type smsnetCliente struct {
	Nome                  string          `json:"nome"`
	CpfCnpj               string          `json:"cpf_cnpj"`
	ContratoStatusDisplay string          `json:"contrato_status_display"`
	ValorCheckOut         json.RawMessage `json:"valor_check_out"` // string or number
	Faturas               []smsnetFatura  `json:"faturas"`
}
type smsnetFatura struct {
	Valor          json.RawMessage `json:"valor"` // number or string
	Vencimento     string          `json:"vencimento"`
	Link           string          `json:"link"`
	LinhaDigitavel string          `json:"linhaDigitavel"`
	Pix            string          `json:"pix"`
}
type smsnetPlano struct {
	Nome       string          `json:"nome"`
	Descricao  string          `json:"descricao"`
	Velocidade string          `json:"velocidade"`
	Valor      json.RawMessage `json:"valor"`
}
type smsnetEmpresa struct {
	Nome      string `json:"nome"`
	CNPJ      string `json:"cnpj"`
	Endereco  string `json:"endereco"`
	Telefone1 string `json:"telefone1"`
	Telefone2 string `json:"telefone2"`
	Email     string `json:"email"`
	Site      string `json:"site"`
}
type smsnetLiberacao struct {
	Liberado    json.RawMessage `json:"liberado"` // number 1 / bool
	Protocolo   string          `json:"protocolo"`
	LiberadoAte string          `json:"liberadoAte"`
	Msg         string          `json:"msg"`
}
type smsnetChamado struct {
	Protocolo string `json:"protocolo"`
	Msg       string `json:"msg"`
}

func mapCliente(s smsnetCliente, now time.Time) phcontracts.Cliente {
	out := phcontracts.Cliente{
		Nome:                  s.Nome,
		CpfCnpj:               s.CpfCnpj,
		ContratoStatusDisplay: s.ContratoStatusDisplay,
		ValorCheckOut:         looseFloat(s.ValorCheckOut),
	}
	for _, f := range s.Faturas {
		vencida, dias := dueState(f.Vencimento, now)
		out.Faturas = append(out.Faturas, phcontracts.Fatura{
			Valor:          looseFloat(f.Valor),
			Vencimento:     f.Vencimento,
			Vencida:        vencida,
			DiasAtraso:     dias,
			Link:           f.Link,
			LinhaDigitavel: f.LinhaDigitavel,
			Pix:            f.Pix,
		})
	}
	return out
}

// brLocation is America/Sao_Paulo; the fatura due-state is computed in the
// tenant's local calendar day. Falls back to UTC if the zone db is unavailable.
var brLocation = loadBR()

func loadBR() *time.Location {
	if loc, err := time.LoadLocation("America/Sao_Paulo"); err == nil {
		return loc
	}
	return time.UTC
}

// dueState reports whether an invoice is overdue and by how many days, comparing
// the parsed vencimento with `now`, BOTH reduced to a calendar DAY in
// America/Sao_Paulo (time-of-day ignored). due today → not overdue; due yesterday
// → overdue, dias=1. An unparseable date degrades safe to (false, 0).
func dueState(vencimento string, now time.Time) (vencida bool, diasAtraso int) {
	due, ok := looseDate(vencimento)
	if !ok {
		return false, 0
	}
	today := startOfDayBR(now)
	dueDay := time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, brLocation)
	days := int((today.Sub(dueDay) + 12*time.Hour) / (24 * time.Hour)) // round to whole days (DST-safe)
	if days > 0 {
		return true, days
	}
	return false, 0
}

func startOfDayBR(t time.Time) time.Time {
	b := t.In(brLocation)
	return time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, brLocation)
}

// looseDate parses an invoice due date. PRIMARY format is dd/mm/aaaa (the
// confirmed SMSNET format, e.g. "15/03/2026"); a few alternatives are tolerated as
// fallbacks in case one of the 19 ISPs varies. ok is false when none parse.
func looseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		"02/01/2006", // dd/mm/aaaa — PRIMARY
		"2006-01-02", // aaaa-mm-dd — ISO fallback
		"02/01/06",   // dd/mm/aa — 2-digit year fallback
	} {
		if t, err := time.ParseInLocation(layout, s, brLocation); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func mapPlano(p smsnetPlano) phcontracts.Plano {
	nome := p.Nome
	if nome == "" {
		nome = p.Descricao
	}
	return phcontracts.Plano{Nome: nome, Descricao: p.Descricao, Velocidade: p.Velocidade, Valor: looseFloat(p.Valor)}
}

func mapEmpresa(e smsnetEmpresa) phcontracts.Empresa {
	tel := e.Telefone1
	if tel == "" {
		tel = e.Telefone2
	}
	return phcontracts.Empresa{Nome: e.Nome, CNPJ: e.CNPJ, Telefone: tel, Email: e.Email, Endereco: e.Endereco, Site: e.Site}
}

func mapLiberacao(l smsnetLiberacao) phcontracts.Liberacao {
	return phcontracts.Liberacao{Liberado: looseBool(l.Liberado), Protocolo: l.Protocolo, LiberadoAte: l.LiberadoAte, Msg: l.Msg}
}

// looseFloat parses a JSON value that may be a number OR a quoted numeric string
// (the SMSNET API returns e.g. valorCheckOut as "99.900000"). Returns 0 on failure.
func looseFloat(raw json.RawMessage) float64 {
	if len(raw) == 0 {
		return 0
	}
	var f float64
	if json.Unmarshal(raw, &f) == nil {
		return f
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
		return v
	}
	return 0
}

// looseBool reads a JSON value that may be a bool OR a number (liberado: 1) OR a
// quoted "1"/"true". Returns true for true / non-zero / "1"/"true".
func looseBool(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var b bool
	if json.Unmarshal(raw, &b) == nil {
		return b
	}
	var n float64
	if json.Unmarshal(raw, &n) == nil {
		return n != 0
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		s = strings.TrimSpace(strings.ToLower(s))
		return s == "1" || s == "true"
	}
	return false
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
