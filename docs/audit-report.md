# Auditoria de backend — chat-smsnet-omnichannel

> **HISTÓRICO (2026-06-12).** Este relatório reflete o estado do código naquela
> data. Vários achados já foram resolvidos desde então (ex.: env órfã
> `MONITORING_RATE_PER_MINUTE` removida; horário/feriado movidos para o channel;
> flow externo `automation` e config singleton do providerhub removidos). Para a
> auditoria de legado mais recente e seu status, ver `docs/legacy-audit.md`.
> Mantido apenas como registro.

> Auditoria recurso-por-recurso contra `/docs` e contra o código real.
> Metodologia: leitura/grep do código-fonte (a documentação **não** foi tomada
> como verdade — onde diverge, o código prevalece e a divergência virou achado).
> Nenhum arquivo de código foi alterado; o único arquivo escrito é este relatório.
> Data: 2026-06-12. Branch: `claude/admiring-knuth-2wk8xd`.

---

## 1. Veredito executivo

| # | Eixo | Status | Severidade do gap |
|---|------|--------|-------------------|
| 1 | Build / vet / test / lint verdes | **OK** | — |
| 1b | Cobertura por pacote (serviços/controllers) | **PARCIAL** | médio (16 controllers sem teste) |
| 2 | Camadas (domain puro, repo iface em domain, BSON em models) | **OK** | baixo (2 ressalvas pontuais) |
| 3 | Isolamento multi-tenant + RBAC | **OK** | — (exceções todas legítimas) |
| 4 | Auth/conta (signup→owner, reconcile aditivo, refresh rotativo, /me) | **OK** | — |
| 5 | Conversas/mensagens (lifecycle, nota interna, soft edit/delete, unread, timeline) | **OK** | — |
| 6 | Realtime (eventos 1:1 com doc, handshake /realtime/ws + /ws, subscribe gated) | **OK** | — |
| 7 | Canais sem JWT (inbound_token hash, rotate, X-Inbound-Token const-time, rate limit) | **OK** | — |
| 8 | providerhub (on-demand, has_*, ClienteResult oneOf, ações auditadas) | **OK** | — |
| 9 | MCP/copilot (tools dinâmicas, write→approval, gate antes do provider, AILog limpo) | **OK** | baixo (superfície monitoring/financial morta) |
| 10 | contacts (CRM tenant-scoped, dedup→409, auditado, campos locais) | **OK** | — |
| 11 | Asynq (handlers/periódicos, retry/backoff/dead-letter, multi-tenant) | **OK** | — |
| 12 | Segredos (AES-GCM, nada de plaintext/decrypt em presenter, exibido 1x) | **OK** | — |
| 13 | Transversais (cursor, envelope, Idempotency-Key, request_id, rate limit, OTel, CORS, RUN_ROLE) | **OK** | baixo (CORS expõe header legado) |
| 14 | LGPD/auditoria (export/anonymize/retention, audit com actor/ip/request_id) | **OK** | — |
| L | **Resquícios legados** | **PARCIAL** | médio (4 itens acionáveis) |

**Resumo:** o backend está sólido, coeso e alinhado com a documentação nos 14
eixos. **Não há falhas de build/test/lint e nenhuma falha crítica de segurança
multi-tenant.** Os achados são: (a) **legado morto** (middleware `tenant_context`
que confia em `X-Tenant-Id`, env `MONITORING_RATE_PER_MINUTE` órfã, header
`X-Tenant-Id` no CORS, superfície de copilot `monitoring/financial` desconectada);
(b) **deriva de documentação** em `api-design.md` (endpoints aspiracionais que não
existem no router); (c) **cobertura de teste** ausente em 16 controllers.

---

## 2. Saída literal de build / vet / test / lint

```
$ go build ./...
(sem saída — exit 0)

$ go vet ./...
(sem saída — exit 0)

$ golangci-lint run
0 issues.

$ go test ./...
(sem FAIL; sem panic)
pacotes com teste OK : 54
pacotes sem teste    : 168   ([no test files])
pacotes com FAIL     : 0
```

Linters habilitados (`.golangci.yml`): conjunto `standard` (errcheck, govet,
ineffassign, staticcheck, **unused**) + `misspell` + `unconvert`. O `unused`
verde garante ausência de **símbolos não-exportados** mortos; símbolos
**exportados** sem chamador não são cobertos por esse linter (ver achado L-1).

---

## 3. Resquícios legados encontrados

| # | Item | Caminho:linha | Tipo | Sev. | Recomendação (NÃO aplicada) |
|---|------|---------------|------|------|------------------------------|
| L-1 | Middleware `TenantContext` + `HeaderTenantID` confiando no header `X-Tenant-Id` | `presenter/middleware/tenant_context.go:9-25` | Middleware legado / código morto (0 chamadores) | **Médio** | Remover o arquivo. O comentário ("until real auth is wired") é da fundação; o auth real (`auth_context.go`) já substituiu. Perigoso se religado: permitiria spoof de tenant via header. |
| L-2 | `X-Tenant-Id` ainda na allow-list de CORS | `presenter/middleware/cors.go:33` | Config legada | **Baixo** | Remover `"X-Tenant-Id"` dos `AllowedHeaders` (o tenant vem do JWT). |
| L-3 | Env `MONITORING_RATE_PER_MINUTE=60` sem leitor no código | `.env.example:66` | Config legada (domínio monitoring removido) | **Médio** | Remover do `.env.example` (nenhum `getString/getInt` lê essa chave em `app/config/config.go`). |
| L-4 | ✅ **RESOLVIDO** — superfície morta de pré-injeção `financial`/`monitoring` removida (gates `allow_financial_data`/`allow_monitoring_data`, campos `PromptContext.Financial/Monitoring`, ramos do `renderContext`, contracts `FinancialDataSource`/`MonitoringDataSource` + `FinancialInfo`/`MonitoringInfo`). Decisão de produto: o copiloto consulta financeiro/monitoramento **sob demanda via tool do ISP**, não por pré-injeção. Sobrou só `allow_customer_data` (com fonte real). | (resolvido) | — | Nada a fazer. |
| L-5 | Comentário "future OLT/monitoring" | `infra/mcp/client.go:4` | Comentário (menção a sistema futuro) | **Info** | Opcional: remover a menção para não sugerir domínio inexistente. |

### Itens verificados e considerados **legítimos** (não-legado)

- **`smsnet-integrations`** (dezenas de ocorrências em `domain/providerhub/*`,
  `infra/providerhub/*`, `presenter/openapi/*`): **não é legado** — é a API
  externa real que o providerhub consulta sob demanda (Customer 360). Integração
  viva e documentada.
- **Sem ocorrências** de `clickhouse`, `nats`, `outbox` em todo o código.
- **`auth_service.go:74`** (`dummy hash`): mitigação de timing-attack legítima,
  não é stub.
- **Nenhum** `mock`/`fake`/`echo`/`stub` adapter fora de `_test.go`.
- **Sem** `TODO`/`FIXME`/`HACK`/`XXX` no código de produção.

---

## 4. Achados por eixo (evidência + recomendação)

### Eixo 1 — Build/test/lint e cobertura
- **OK:** build, vet, lint (0 issues) e test todos verdes (§2).
- **PARCIAL (cobertura):** **todos os 25 serviços de domínio têm teste**; mas
  **16 controllers não têm** nenhum teste:
  `attachments, audit, automation, businesshours, conversationtools, csat, iam,
  mcp, notifications, presence, providerhub, queues, search, sectors, sla, tenant`.
  - *Severidade:* médio. *Recomendação:* adicionar testes de stack HTTP
    (status/authz/envelope) ao menos para `iam` (users/roles — superfície de
    segurança), `providerhub` e `sla`.

### Eixo 2 — Camadas / dependências
- **OK:** `domain/` não importa driver Mongo/Redis/Asynq; interfaces de repo em
  `domain/*/repository` e implementações em `infra/database/mongodb/repositories`;
  BSON concentrado em `models/`.
- **Ressalva (info):** `domain/apperror/apperror.go:13` importa `net/http`
  apenas para o mapa código→status (`httpStatus`, L33). Acoplamento deliberado e
  aceitável.
- **Ressalva (baixo):** `app/start_routines/bootstrap_seeds.go` usa `bson` direto
  (upserts de seed). Aceitável por ser rotina de bootstrap, mas é o único ponto
  fora de `infra/` que toca BSON — manter isolado.

### Eixo 3 — Multi-tenant + RBAC
- **OK (tenant):** toda query de lista/CRUD filtra `tenant_id` derivado do token
  (`shared.RequireTenant`). As **exceções sem tenant** são todas legítimas e
  pré-auth por natureza:
  - `tenant/tenant_repository.go:42,50,61` (coleção do próprio tenant; List ativo é cross-tenant intencional para jobs);
  - `auth/refresh_token_repository.go:35` (FindByHash) e `auth/account_token_repository.go:38,89,141` (verify/reset/invite por hash de token);
  - `csat/response_repository.go:76` (resposta pública por token);
  - `channels/connection_repository.go:121` `FindByInboundTokenHash` (inbound pré-auth);
  - `iam/user_repository.go:104` `FindByEmailAnyTenant` (idempotência de signup) — note que `FindByEmail` (L95) **é** tenant-scoped.
  - *Obs.:* o nome citado nas regras (`FindByWebhookVerifyToken`) foi **renomeado**
    para `FindByInboundTokenHash` (token agora é hash). A lista de exceções
    "permitidas" da regra está desatualizada; o código está correto.
- **OK (RBAC):** todas as rotas protegidas usam `AuthContext` + `RequirePermission`
  com **constantes do catálogo** (24 constantes distintas em uso; **zero** strings
  literais — `grep RequirePermission("` vazio). Escopo `ScopeOwn/ScopeAll` aplicado
  no serviço (`ResolveEffective`, `routing`/visibilidade).
- **OK (não-vazamento):** padrão "neutro" em auth (`auth_service.go:23`) e
  respostas de signup/forgot sempre neutras.

### Eixo 4 — Auth / conta
- **OK:** `Signup` cria tenant + owner com o papel **owner = `AllPermissions()`**
  (`account_service.go:160` `ownerRoleIDs` → `role_service.go:80` `SeedDefaults`
  → `authz.DefaultRoles()` owner = `AllPermissions()`).
- **OK (reconcile aditivo/idempotente):** `bootstrap_seeds.go` reconcilia o owner
  de **todos** os tenants para o catálogo completo (`reconcileOwnerRoles`,
  `UpdateMany name="owner"`), e `SeedDefaults` reconcilia o owner em conflito —
  aditivo, nunca destrutivo (admin/agent preservados).
- **OK:** login/refresh **rotacionam** o refresh (`auth_service.go:118` `Revoke`
  do antigo antes de emitir novo par); logout revoga (L150).
- **OK:** `/me` resolve permissões pela **união dos papéis filtrada pelo catálogo**
  (`ResolveEffective`, `user_service.go:254` — só inclui `p` se `p ∈ AllPermissions()`).

### Eixo 5 — Conversas / mensagens
- **OK:** eventos de ciclo de vida em `conversations/entity/event.go:7-20`
  (created/updated/closed/reopened/assigned/transferred/enqueued/tagged + automation);
  **nota interna** é evento próprio `internal_note.added` (L11) e nunca é enviada
  ao cliente; soft edit/delete gated por `MessageSend`/`MessageDelete`
  (`conversations_routes.go`); `unread_count`/`last_read_at` no schema; timeline
  (`events`) separada de `messages` (rotas `/events` vs `/messages`).

### Eixo 6 — Realtime
- **OK:** nomes de evento batem com `docs/realtime-events.md` (definidos em
  `conversations/entity/event.go`, `queues/contracts/events.go:7` `queue.stats`,
  `sla/contracts/contracts.go:6` `sla.warning`, `copilot/contracts/dtos.go:7`
  `copilot.suggestion_completed`, `mcp/contracts/contracts.go:13`
  `mcp.approval_requested`, `notifications/contracts/contracts.go:13`
  `notification.created`).
  - *Nuance documentada:* `sla.warning`/`sla.breached` são **eventos realtime**;
    `sla.at_risk`/`sla.breached` (`notifications/entity/notification.go:15-16`)
    são **tipos de notificação**. São conceitos distintos, ambos legítimos.
- **OK:** handshake em `/realtime/ws` **e** alias `/ws`
  (`app/routes/websocket/router.go:16`); upgrade exige JWT (Bearer ou `?token=`,
  `presenter/websocket/handler.go:70-77`); **subscribe a sala de conversa é
  gated por `conversation.read`** (`handler.go:159`); salas montadas
  server-side a partir do tenant da conexão (L156) — sem cross-tenant.

### Eixo 7 — Canais / integração sem JWT
- **OK:** `inbound_token` base62 de 32 bytes gerado na criação, guardado **só como
  hash** (`InboundTokenHash`), exibido 1x; `rotate-inbound-token`; comparação em
  **tempo constante** (`channel_service.go:281` `subtle.ConstantTimeCompare`);
  o token **não** abre rotas `/v1` (só é consumido em `ResolveInbound`); HMAC do
  corpo opcional; rate limit dedicado (`policy.InboundChannelRateLimit`, scope
  `inbound_channel`).

### Eixo 8 — providerhub
- **OK:** consultas sob demanda sem persistir payload externo
  (`query_service.go`); segredo cifrado e **mascarado** via `has_secret`
  (`schemas.go:164`); `ClienteResult` modelado como **oneOf**
  (`schemas.go:213-220`, ClienteFound/ClienteNeedsSelection); `liberacao`/`chamado`
  **auditados** (`query_service.go:123,150` actions `providerhub.liberacao`/
  `providerhub.chamado`) e atrás de `integration.execute_action`
  (`external_routes.go`).

### Eixo 9 — MCP / copilot
- **OK:** tools descobertas dinamicamente (`mcp` tools list); **read executa /
  write apenas propõe + aprova** (`copilot_service.go:139-140` "a proposed write
  action always needs approval, regardless of the config flag"); gates
  `allow_*_data` aplicados **antes do provider** no ponto único
  (`context_builder.go:17,47`); `AILog` guarda só resumo, **nunca prompt/raw**
  (`copilot/entity/log.go:26`, `copilot_models.go:22`); `echo` só em testes.
- **Achado (baixo):** ver L-4 — ✅ **RESOLVIDO**: os gates inertes
  `allow_financial_data`/`allow_monitoring_data` e toda a pré-injeção
  financeiro/monitoramento foram removidos (consulta agora é sob demanda via tool
  do ISP). Sobrou só `allow_customer_data`, com fonte real.

### Eixo 10 — contacts (CRM)
- **OK:** `GET/POST/PATCH /v1/contacts` tenant-scoped (`contacts_routes.go`,
  perms `contact.read`/`contact.write`); **dedup → 409** por documento e telefone
  (`contact_service.go:154-165` `apperror.Conflict`); create/update auditados;
  apenas campos locais (sem dados enriquecidos do provider).

### Eixo 11 — Asynq
- **OK:** handlers e periódicos registrados (`bootstrap_workers.go`,
  `bootstrap_scheduler.go`); retry/backoff/dead-letter (`client.go:30`
  `MaxRetry`; `bootstrap_workers.go:86` "retry/backoff and dead-lettering");
  retenção por tenant; fan-out multi-tenant (`eachTenant`).

### Eixo 12 — Segredos
- **OK:** `infra/secrets/cipher.go` usa **AES-GCM** (`crypto/aes` +
  `cipher.NewGCM`, L25-29; formato `base64(nonce||ciphertext)`); **nenhum**
  `Decrypt`/`EncryptedSecret` em `presenter/` (as ocorrências de `Secret` em
  `presenter/contracts/*` são DTOs de request na criação ou respostas one-time —
  `webhooks/dto.go:112`, rotate de canal); segredo exibido só na criação.

### Eixo 13 — Padrões transversais
- **OK:** cursor/keyset (`shared.DecodeCursor`/`ApplyKeyset`); envelope de erro
  `{error:{code,message,details?,request_id?}}`; `Idempotency-Key`
  (`middleware/idempotency.go`); `request_id` (`middleware/request_id.go`); rate
  limit (`middleware/ratelimit.go`); **OTel** (`app/providers/observability.go`,
  `presenter/middleware/telemetry.go`); CORS allow-list (`middleware/cors.go`);
  seed idempotente; `RUN_ROLE` (`config.RunsRole`).
- **Achado (baixo):** CORS ainda lista `X-Tenant-Id` (ver L-2).

### Eixo 14 — LGPD / auditoria
- **OK:** export/anonymize/retention (`/privacy/contacts/{id}/export`,
  `/privacy/contacts/{id}/anonymize`, `/privacy/retention`; job
  `privacy.retention` em `bootstrap_scheduler.go:30`); auditoria nas ações
  sensíveis com `actor/ip/request_id` (`shared.AuditEntry`, preenchido na borda
  HTTP) — auth, role, canal (token_rotated), providerhub, privacy, contacts.

---

## 5. Deriva de documentação (doc ≠ código)

| Item | Evidência | Sev. | Recomendação |
|------|-----------|------|--------------|
| `api-design.md` lista endpoints inexistentes no router | `docs/api-design.md` cita `/privacy/requests`, `/queues/{id}/members`, `/presence/me`; router real tem `/privacy/contacts/{id}/export`, `/privacy/retention`, `/agents/presence/status` etc. | Baixo | Marcar `api-design.md` explicitamente como **referência aspiracional** ou alinhar ao router. O **`openapi.yaml` é o contrato preciso** (gerado do código, em sync via `TestDocsCopyInSync`). |
| `api-design.md` usa base `/api/v1` | doc §"Convenções"; router real serve sob `/v1` | Baixo | Corrigir a base no doc. |

> Observação: o catálogo de permissões em `docs/security-permissions.md` e o
> bloco `iam` em `docs/api-design.md` **já foram corrigidos** em commits
> anteriores desta branch e agora batem com `domain/authz/permission.go`.

---

## 6. Pendências priorizadas (sem aplicar)

1. **(Médio) Remover legado de tenant-por-header** — apagar
   `presenter/middleware/tenant_context.go` (L-1) e o `X-Tenant-Id` do CORS (L-2).
   Risco de segurança latente se religado.
2. **(Médio) Remover env órfã** `MONITORING_RATE_PER_MINUTE` do `.env.example` (L-3).
3. **(Médio) Cobertura de controllers** — adicionar testes de stack HTTP aos 16
   controllers sem teste, priorizando `iam`, `providerhub`, `sla`.
4. **(Baixo-Médio) Decidir sobre a superfície `monitoring`/`financial` do copilot**
   (L-4): conectar as fontes ou remover gate/prompt mortos.
5. **(Baixo) Alinhar `api-design.md`** ao router real / marcar como aspiracional (§5).
6. **(Info) Limpar comentário** "future OLT/monitoring" (`infra/mcp/client.go:4`).

---

*Fim do relatório. Nenhuma alteração de código foi feita; apenas este arquivo
(`docs/audit-report.md`) foi escrito.*
