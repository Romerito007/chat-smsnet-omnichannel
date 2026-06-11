# Relatório de Auditoria — Backend `chat-smsnet-omnichannel`

> **Papel:** Auditor backend sênior Go. **Escopo:** conformidade com a arquitetura
> e os contratos em `/docs`; **nenhuma regra de negócio foi alterada**. Este
> documento é a única escrita produzida pela auditoria.
>
> **Data:** 2026-06-11 · **Commit/branch:** `claude/admiring-knuth-2wk8xd`
>
> **Legenda de status:** `OK` · `PARCIAL` · `FALTANDO`
> **Severidade:** crítico · alto · médio · baixo

---

## 0. Veredito executivo

| Eixo | Status | Severidade do gap |
|------|--------|-------------------|
| Build / vet / testes | **OK** (0 falhas) | — |
| Arquitetura em camadas (domain/infra/presenter/app) | **OK** (desvios menores) | baixo |
| Isolamento por `tenant_id` (sempre do JWT) | **OK** | — |
| RBAC + escopo de setor nas rotas | **OK** | baixo |
| Asynq (handlers, periódicos, retry/backoff/dead-letter) | **OK** | baixo |
| Constraints providerhub/monitoring (sem payload externo, segredos cifrados) | **OK** | — |
| Sem chatbot / flow builder; automation só integra flow externo | **OK** | — |
| Copilot: provider por tenant + gates `allow_*_data` + AILog | **OK** | — |
| Inbound: idempotência + sem enriquecimento por provedor | **OK** | — |
| Padrões transversais (cursor, envelope, idempotency, OTel, seed) | **OK** | — |
| Eventos realtime vs `docs/realtime-events.md` | **PARCIAL** | médio |
| Mensagens: soft-delete operável | **PARCIAL** | médio |
| Testes de controllers/presenters | **PARCIAL** (ausentes) | médio |
| Domínio `contacts` sem camada presenter (sem API REST própria) | **PARCIAL** | baixo |

**Conclusão:** a implementação adere fortemente à arquitetura e às restrições de
segurança mais sensíveis (multi-tenant, RBAC, segredos, providerhub/monitoring,
LGPD, automação externa). Os achados abertos são de **conformidade funcional e
cobertura de testes**, não há gap **crítico/alto** de segurança.

---

## 1. Build & Testes (execução obrigatória)

| Comando | Resultado | Observação |
|---------|-----------|------------|
| `go build ./...` | **OK** — exit 0, sem saída | Compila inteiro |
| `go vet ./...` | **OK** — exit 0, sem saída | Sem diagnósticos |
| `go test ./...` | **OK** — **0 FAIL**, 36 pacotes com testes `ok` | Sem falhas |
| Linter dedicado | **N/A** | Não há config `golangci-lint`/`.golangci.yml` nem alvo no `Makefile` |

**Cobertura de testes (item 14):**
- 33 arquivos `_test.go` em `domain/`, cobrindo praticamente todos os serviços:
  auth, iam, contacts, sectors, queues, presence, conversations, conversationtools,
  routing, channels, automation, providerhub, monitoring, copilot, sla, csat,
  webhooks, notifications, businesshours, search, privacy, attachments, reports +
  `shared`, `apperror`, `maintenance`, `businesshours/entity`.
- Testes de integração ao vivo (Mongo) sob a tag `e2e`: `privacy` e `attachments`.
- **Gaps:** `domain/audit` **sem teste de serviço**; **nenhum** teste em
  `presenter/controller/*` (apenas `presenter/middleware/auth_context_test.go`,
  `presenter/http/health_handler_test.go`, `presenter/websocket/handler_test.go`).
  → Severidade **médio**: o enunciado pede testes em "serviços/controllers
  principais"; os serviços estão cobertos, os controllers não.

---

## 2. Camadas por domínio (item 1)

Matriz de presença de camadas (`Y` presente, `—` ausente; notas corrigem
falsos-negativos de nomenclatura de arquivo):

| Domínio | entity | contracts | repo iface | service | model BSON | repo Mongo | DTO | controller | rotas | Status |
|---|:--:|:--:|:--:|:--:|:--:|:--:|:--:|:--:|:--:|---|
| tenant | Y | — | Y | Y | inline | Y | Y | Y | Y | OK |
| auth | Y | Y | Y | Y | `auth_models.go` | Y | Y | Y | Y | OK |
| iam | Y | Y | Y | Y | `identity_models.go` | Y | Y | Y | Y | OK |
| contacts | Y | Y | Y | Y | `contacts_models.go` | Y | **—** | **—** | **—** | PARCIAL |
| sectors | Y | Y | Y | Y | Y | Y | Y | Y | Y | OK |
| queues | Y | Y | Y | Y | Y | Y | Y | Y | Y | OK |
| presence | Y | Y | Y | Y | Redis | Y | Y | Y | Y | OK |
| conversations | Y | Y | Y | Y | Y | Y | Y | Y | Y | OK |
| conversationtools | Y | Y | Y | Y | Y | Y | Y | Y | Y | OK |
| routing | n/a | Y | n/a | Y | n/a | n/a | Y | Y | Y | OK (opera sobre conversations) |
| channels | Y | Y | Y | Y | Y | Y | Y | Y | Y | OK |
| automation | Y | Y | Y | Y | Y | Y | Y | Y | Y | OK |
| providerhub | Y | Y | Y | Y | Y | Y | Y | Y | Y | OK |
| monitoring | Y | Y | Y | Y | Y | Y | Y | Y | Y | OK |
| copilot | Y | Y | Y | Y | Y | Y | Y | Y | Y | OK |
| sla | Y | Y | Y | Y | Y | Y | Y | Y | Y | OK |
| csat | Y | Y | Y | Y | Y | Y | Y | Y | Y | OK |
| webhooks | Y | Y | Y | Y | `webhook_models.go` | Y | Y | Y | `webhook_routes.go` | OK |
| notifications | Y | Y | Y | Y | `notification_models.go` | Y | Y | Y | Y | OK |
| businesshours | Y | Y | Y | Y | Y | Y | Y | Y | Y | OK |
| search | n/a | Y | Y | Y | n/a | Y | Y | Y | Y | OK (indexa dados de outros) |
| privacy | Y | Y | Y | Y | inline no repo | Y | Y | Y | Y | OK |
| audit | Y | — | Y | Y | inline no repo | Y | Y | Y | em `privacy_routes.go` | OK |
| attachments | Y | Y | Y | Y | inline no repo | Y | Y | Y | Y | OK |
| realtime | infra (`infra/realtime`) + WS (`presenter/websocket`) | — | — | — | — | — | — | — | — | OK (camada de transporte) |

**Achados:**
- **`contacts` sem camada presenter** (`presenter/contracts/contacts`,
  `presenter/controller/contacts`, `app/routes/http/contacts_routes.go` ausentes).
  Contatos são criados via `UpsertFromInbound` (channels/inbound) e lidos via
  `search` e no contexto da conversa; **não há CRUD REST de contato** apesar de
  existirem permissões `contact.read`/`contact.view_*`. → **PARCIAL / baixo**
  (decisão de design plausível, mas diverge do molde por-domínio).
- **BSON inline** em `privacy`, `audit`, `attachments` e `tenant`: a struct BSON
  vive no arquivo do repositório em vez de `models/<X>_models.go`. → **baixo**
  (consistência de convenção; sem impacto funcional).
- Nomes de arquivo no singular (`webhook_*`, `notification_*`, `identity_*`) e
  rota de `audit` montada em `privacy_routes.go` — **conformes**, apenas fora do
  padrão `<X>_…`.

---

## 3. Isolamento multi-tenant (item 2) — **OK**

- **Tenant sempre do JWT, nunca do cliente.**
  `presenter/middleware/auth_context.go:19-20,44` — "tenant taken exclusively from
  the signed token — never from a client header"; `ctx = shared.WithTenant(ctx,
  claims.TenantID)`. `app/routes/http/router.go:31-32` confirma que não há
  middleware global de tenant por header.
- `presenter/middleware/tenant_context.go` (header `X-Tenant-Id`) é **legado
  pré-auth e NÃO é usado nas rotas protegidas**; documentado como substituído pela
  claim verificada.
- **Todos os repositórios de produção filtram por `tenant_id`** via
  `shared.RequireTenant(ctx)`/`TenantFrom(ctx)` (≈37 repos verificados).
  Exceções legítimas, não-multi-tenant:
  - `repositories/auth/refresh_token_repository.go` — `FindByHash` é lookup
    pré-auth por hash não-adivinhável (comentado); `RevokeAllForUser` filtra tenant.
  - `repositories/tenant/tenant_repository.go` — registro de tenants (sistema).
  - `repositories/search/mappers.go` — só funções de mapeamento, sem query.

> Sem brechas de isolamento. Índices `(tenant_id, …)` reforçam o filtro
> (`migrations/0001_baseline_indexes.go`).

---

## 4. RBAC + escopo de setor (item 3) — **OK**

- **20 de 23** grupos de rotas aplicam `middleware.AuthContext` **e**
  `middleware.RequirePermission(...)`. Exemplos: iam→`UserManage`,
  sectors/businesshours/conversationtools(writes)→`SectorManage`,
  conversations→por ação (`ConversationRead/Assign/Close`, `MessageSend`),
  routing→`ConversationAssign`/`Transfer`, channels→`ChannelManage`,
  webhooks→`WebhookManage`, copilot→`CopilotConfigure`/`CopilotUse`,
  reports→`ReportView`/`ReportExport`, privacy→`PrivacyManage`, audit→`AuditView`.
- **3 grupos** (`presence`, `notifications`, `attachments`) usam só `AuthContext`
  e impõem a autorização na camada de serviço (recursos do próprio usuário /
  acesso à conversa). Documentado em comentário. → **OK / baixo** (aceitável;
  recomendável tornar explícito).
- **Endpoints públicos** propositais com credencial alternativa: inbound/receipts
  (assinatura), submissão CSAT (token), download de privacy e blob de attachments
  (token HMAC assinado).
- **Escopo de setor** aplicado no serviço: `domain/authz/context.go`
  (`CanAccessSector`, `ScopeAll`/`ScopeOwn`); `conversations` usa `visibleTo`/
  `loadVisible` retornando **404** (evita vazamento de existência). Papéis padrão:
  owner/admin=`ScopeAll`, agent=`ScopeOwn` (`domain/authz/authz.go`).

---

## 5. Eventos realtime (item 4) — **PARCIAL** · médio

Publicados a partir de 8+ serviços (conversations, channels, routing, sla,
presence, copilot, notifications, inbound). Constantes em
`domain/conversations/contracts/events.go`.

Comparação com `docs/realtime-events.md`:

| Evento (doc) | Publicado? | Evidência / observação |
|---|---|---|
| `conversation.created` | **—** | criação publica `conversation.updated` |
| `conversation.updated` | ✅ | `conversation_service.go` (RealtimeConversationUpdated, 10 usos) |
| `conversation.assigned` | ✅ | `routing_service.go` |
| `conversation.transferred` | ✅ | `routing_service.go` |
| `conversation.resolved` | **—** | fecha emitindo `conversation.updated` |
| `conversation.closed` | **—** | idem |
| `conversation.reopened` | **—** | idem |
| `message.created` | ✅ | conversations/inbound |
| `message.read` | ✅ | conversations + receipt |
| `message.status` | ⚠️ | dividido em `message.sent/delivered/read/failed` (mais granular que o doc) |
| `typing` | ✅ | `typing.started/stopped` |
| `presence.changed` | ✅ | `presence_service.go` |
| `queue.stats` | **—** | nenhum publisher em `domain/queues` |
| `sla.warning` / `sla.breached` | ✅ | `sla_service.go` |
| `copilot.suggestion` | ⚠️ | código emite `copilot.suggestion_completed` (nome divergente) |
| `automation.updated` | **—** | automação reusa `conversation.updated`/`message.created` |
| `notification.created` | ✅ | `notifications/service.go` |

**Achado:** eventos de ciclo de vida de conversa (`created/closed/resolved/
reopened`), `queue.stats` e `automation.updated` **não são emitidos com os nomes
especificados** — um frontend que assine esses nomes não os receberá. Sem impacto
de segurança; **conformidade de contrato realtime**. → **médio**.

---

## 6. Jobs Asynq (item 5) — **OK**

**Handlers** (`app/start_routines/bootstrap_workers.go`): `automation.invoke`,
`automation.timeout`, `channel.deliver`, `channel.retry`, `webhook.deliver`,
`webhook.retry`, `notification.send`, `notification.email`, `csat.send`,
`csat.expire`, `privacy.export`. Periódicos:
`chat.close_inactive_conversations`, `notifications.cleanup`,
`channels.health_check`, `audit.compact`, `privacy.retention`, `reports.snapshot`,
`sla.check`.

**Scheduler** (`bootstrap_scheduler.go`): os 7 crons exigidos +
`notifications.cleanup` e `privacy.retention`. → **OK** (cobre o conjunto pedido:
chat.close_inactive_conversations, sla.check, reports.snapshot, audit.compact,
channels.health_check).

**Retry / backoff / dead-letter:**
- Canais: `domain/channels/service/outbound_service.go` — `defaultMaxAttempts=5`,
  backoff exponencial `1<<attempt` cap 300s (`backoff.go`); Asynq com
  `MaxRetry(0)` (o domínio é dono do retry, reagenda via `ProcessIn`). Esgotado →
  `DeliveryFailed` + evento `message.failed` (sem fila DLQ; estado terminal).
- Webhooks: `delivery_service.go` — `defaultMaxAttempts=6`, backoff exponencial
  cap 300s; **dead-letter** explícito (`status = DeliveryDead`, `LastError`,
  `NextRetryAt` limpo). Rate-limit reagenda sem consumir tentativa.

> Observação (baixo): canais não têm DLQ formal — usam status `failed` terminal +
> evento. Aceitável; webhooks têm o `dead` state mais completo.

---

## 7. Canais — delivery_status (item 6) — **OK**

`domain/channels/entity/outbound_delivery.go`: enum `pending/sent/delivered/read/
failed` com `deliveryRank` garantindo transições **forward-only** e idempotentes.
`outbound_service.go`: `pending→sent` no envio (publica `message.sent`);
`sent→delivered`/`read` via receipt (`Advances` guarda ordem); `→failed` após
máx. tentativas ou receipt de erro (publica `message.failed`). Receipts
idempotentes por `FindByExternalMessageID`.

---

## 8. Mensagens (item 7) — **PARCIAL** · médio

- **Nota interna não vai ao cliente:** `AddInternalNote` cria mensagem com
  `Direction = DirectionInternal`; webhook outbound exclui notas
  (`conversation_service.go:309-344,639`). → **OK**.
- **Fechamento com `close_reason.requires_note=true` exige `note`:**
  `conversation_service.go:361-371` consulta `closeReasons.RequiresNote` e retorna
  `validation` se a nota faltar; política em
  `conversationtools/service/close_reason_service.go`. → **OK**.
- **Soft-delete (`edited_at`/`deleted_at`):** o modelo suporta — campos
  `EditedAt`/`DeletedAt` + `IsDeleted()` (`entity/message.go:86-91`), o repo
  persiste `deleted_at` (`message_repository.go:50`) e `ListByConversation` filtra
  `deleted_at: nil` (`:86`). **Porém não há operação de editar/excluir mensagem**
  (nenhum método de serviço `EditMessage`/`DeleteMessage`, nenhuma rota
  `PATCH/DELETE /conversations/{id}/messages/{mid}`). → **PARCIAL / médio**:
  capacidade modelada e leitura correta, mas **não há caminho para soft-deletar**.

---

## 9. Inbound (item 8) — **OK**

- **Idempotência** por `tenant_id + channel + external_message_id`: índice único
  `uniq_tenant_channel_external_msg` (`migrations/0005:46-52`) + checagem
  `FindByExternalID` e lock (`inbound_service.go:98-107,137-149`); duplicado →
  resultado idempotente / `Conflict` sob corrida.
- **Sem consulta automática ao providerhub** e **sem enriquecimento por provedor**:
  zero referência a `providerhub`/`monitoring` no inbound; contato via
  `UpsertFromInbound` apenas com dados locais (channel, external_id, nome, telefone,
  documento). → **OK**.

---

## 10. ProviderHub / Monitoring (item 9) — **OK**

- **Sem persistir payload externo:** `models/providerhub_models.go` e
  `monitoring_models.go` — query log contém apenas
  `query_type, status, latency_ms, error_summary, user/contact/conversation, created_at`.
  **Não há `response_body`/payload.** Sem job de sync no scheduler.
- **Segredos cifrados e mascarados:** AES-256-GCM em `infra/secrets/cipher.go`;
  repos cifram `EncryptedSecret` (providerhub/monitoring/channels); DTOs expõem
  `HasSecret bool` **sem** campo `Secret` (`contracts/providerhub/dto.go`,
  `channels/dto.go`, `webhooks/dto.go`). Webhook secret retornado **uma única vez**
  no create.
- **Permissões** `contact.view_financial` / `contact.view_connection_status`
  aplicadas nas rotas externas (`app/routes/http/external_routes.go:33-45`).

---

## 11. SLA (item 10) — **OK**

`BusinessHoursOnly` na política (`sla/entity/policy.go`) e o serviço usa
`bizClock` (relógio de business hours) na avaliação (`sla_service.go:93`). Breach
via scheduler (`sla.check`) → `fire()` publica realtime `sla.breached`
**e** emite webhook `s.webhooks.Emit(...)` (`sla_service.go:200-237`). → **OK**.

---

## 12. Copilot (item 11) — **OK**

- Provider configurável por tenant (`copilot/entity/config.go`: Provider, Model,
  Temperature, MaxTokens).
- Gates `allow_*_data` aplicados na montagem de contexto
  (`context_builder.go:47/53/59`): cliente/financeiro/monitoramento só entram
  quando o flag é `true` → **não envia financeiro com `allow_financial_data=false`**.
- **AILog persistido por inferência** (`entity/log.go`, `repositories/copilot/
  log_repository.go`, coleção `copilot_logs`); `input_summary` registra os flags,
  não o dado bruto.

---

## 13. Padrões transversais (item 12) — **OK**

| Padrão | Evidência |
|---|---|
| Paginação por cursor (keyset) | `domain/shared/pagination.go` + `ApplyKeyset`/`KeysetSort` nos repos |
| Envelope de erro padrão | `domain/apperror` + `presenter/middleware/response.go` (`WriteError`) |
| Idempotency-Key em POST | `presenter/middleware/idempotency.go`, aplicado em `/v1` (`router.go:35`) |
| Rate limit tenant/IP | `presenter/middleware/ratelimit.go` (`ratelimit:<tenant>:<ip>`) |
| OpenTelemetry | `app/providers/observability.go` + `presenter/middleware/telemetry.go` (gated por `OTEL_ENABLED`) |
| Recover / request_id / CORS / timeouts | `recover.go`, `request_id.go`, `cors.go`, `app/server/server.go` |
| Seed idempotente | `app/start_routines/bootstrap_seeds.go` (upserts por chave natural) |
| Binário único `RUN_ROLE` | `app/config` (`all|api|ws|worker|scheduler`) + start routines |

---

## 14. Segurança / Auditoria / LGPD (item 13) — **OK**

- **Audit log** nas ações sensíveis via porta `shared.Auditor`: `auth.login/logout`,
  `user.*`, `role.*` (alteração de permissões), `webhook.*`,
  `conversation.closed`/`transferred`, `providerhub.financial_status.viewed`/
  `ticket.opened`, `ai.config.updated`, `privacy.*`, `report.export`. `AuditLog`
  carrega `actor_type`, `ip`, `user_agent` (capturados na borda). Consulta
  `GET /v1/audit` (`audit.view`).
- **Privacy (LGPD)** presente: export (job + URL assinada temporária), anonimização
  (remove PII mantendo integridade, recusa sob legal hold), retenção configurável
  por tenant aplicada pelo scheduler (`privacy.retention`).

---

## 15. Pendências priorizadas

| # | Severidade | Achado | Local | Recomendação (não aplicada) |
|---|---|---|---|---|
| 1 | **médio** | Testes de **controllers/presenters** ausentes; `domain/audit` sem teste de serviço | `presenter/controller/*` (0 testes); `domain/audit` | Adicionar testes de controller (tabela request→status/JSON) e de `audit/service`. |
| 2 | **médio** | Eventos realtime de ciclo de vida (`conversation.created/closed/resolved/reopened`), `queue.stats` e `automation.updated` não emitidos com os nomes do doc; `copilot.suggestion` vs `copilot.suggestion_completed` | `domain/conversations/contracts/events.go`, `domain/queues`, `docs/realtime-events.md` | Emitir os eventos nominais **ou** atualizar `docs/realtime-events.md` para refletir `conversation.updated`/eventos granulares. |
| 3 | **médio** | Soft-delete de mensagem modelado mas **sem operação** (não há editar/excluir) | `domain/conversations/service`, rotas de conversas | Implementar `EditMessage`/`DeleteMessage` (soft) com permissão, ou documentar que está fora do MVP. |
| 4 | **baixo** | `contacts` sem camada presenter (sem CRUD/leitura REST próprios) apesar de `contact.read`/`contact.view_*` | `presenter/*/contacts`, `app/routes/http` | Expor `GET /v1/contacts` (lista/detalhe) respeitando visibilidade, se for requisito de produto. |
| 5 | **baixo** | BSON inline (fora de `models/<X>_models.go`) em privacy/audit/attachments/tenant | repos correspondentes | Mover structs BSON para `models/` por consistência. |
| 6 | **baixo** | `presence`/`notifications`/`attachments` sem `RequirePermission` (autorização só no serviço) | rotas correspondentes | Manter, mas tornar a regra explícita/coberta por teste. |
| 7 | **baixo** | Canais sem DLQ formal (usam status `failed` terminal) | `domain/channels/service/outbound_service.go` | Opcional: espelhar o `dead` state dos webhooks para inspeção. |
| 8 | **info** | Sem linter configurado | repo | Adicionar `golangci-lint` + alvo `make lint` no CI. |

> Nenhum item **crítico** ou **alto**. Os achados são de conformidade funcional,
> cobertura de testes e consistência de convenção.

---

## 16. Build & Testes — saída resumida

```text
$ go build ./...
# (sem saída) — exit 0

$ go vet ./...
# (sem saída) — exit 0

$ go test ./...
# 0 FAIL; 36 pacotes "ok" (demais: "no test files")
# domínios com teste de serviço: auth, iam, contacts, sectors, queues, presence,
#   conversations, conversationtools, routing, channels, automation, providerhub,
#   monitoring, copilot, sla, csat, webhooks, notifications, businesshours, search,
#   privacy, attachments, reports (+ shared, apperror, maintenance, businesshours/entity)
# sem teste: domain/audit (serviço) e presenter/controller/* (todos)
# integração viva (tag e2e): privacy, attachments  [requer Mongo:27017 + Redis:6379]

$ golangci-lint run
# N/A — nenhum linter configurado no projeto
```

— Fim do relatório.

---

## 17. Verificação manual de segurança (spot-check)

> **Data:** 2026-06-11 · **Commit verificado:** `7a82d07`
> (*audit: record AuditLog on message soft-delete*) · branch
> `claude/admiring-knuth-2wk8xd`.
>
> Inspeção **direta de código** (não confiando no "OK" de seções anteriores) dos
> três pontos de maior risco. Os três passaram. **Nenhum código foi alterado** —
> é registro de auditoria.

### 17.1 Filtro `tenant_id` em todos os repositórios — **APROVADO**

Varridos os 136 métodos de leitura/escrita em
`infra/database/mongodb/repositories/`. Todo filtro de query de domínio
(`Find/List/Get/Count/Update/Delete`) inclui `"tenant_id"` derivado de
`shared.RequireTenant(ctx)` / `TenantFrom(ctx)` — **nunca** de um parâmetro
vindo do corpo/query do request. Filtros de escopo adicional (visibilidade por
setor, `user_id` em notificações) e documentos `$set` / pipelines `$group` são
aplicados **por cima** do `tenant_id`, sem substituí-lo.

**Exceções intencionais sem `tenant_id` — NÃO "corrigir" no futuro:**

| Local | Por que é legítimo |
|---|---|
| `repositories/auth/refresh_token_repository.go` → `FindByHash` | Lookup **pré-autenticação** por hash não-adivinhável do refresh token; ainda não há tenant no contexto. O registro encontrado carrega o `tenant_id` autoritativo. `RevokeAllForUser` filtra por tenant. |
| `repositories/csat/response_repository.go` → `FindByToken` | O endpoint **público** de resposta da pesquisa (cliente final) só tem o token; sem contexto de tenant. O token é a credencial e o registro carrega o tenant. |
| `repositories/channels/connection_repository.go` → `FindByWebhookVerifyToken` | **Pré-auth** de webhook de entrada do canal: o provedor externo posta sem tenant; resolve-se a conexão pelo verify token e o registro carrega o `tenant_id`. |
| `repositories/tenant/tenant_repository.go` (todo) | É o **registro de tenants do próprio sistema** (metadados), não uma coleção multi-tenant. `ListActive` é usado pelos jobs para iterar tenants. |
| `repositories/sla/tracking_repository.go` → `ListRunningAcrossTenants` | **Intencionalmente cross-tenant** — alimenta o job `sla.check`, que avalia trackings de todos os tenants e depois opera por tenant. Comentado no código (`intentionally NOT tenant-scoped`). |

Jobs de sistema que rodam por-tenant (`maintenance.DayCounts`,
`privacy.ApplyRetention`/`heldConversationIDs`) **mantêm** `tenant_id` no filtro —
o "system actor" injeta o tenant no contexto antes de chamar o repositório.

### 17.2 Mascaramento de segredo nos DTOs — **APROVADO**

- `EncryptedSecret` / `encrypted_secret`: **zero** ocorrências em `presenter/` —
  o segredo cifrado nunca cruza a camada de apresentação.
- Structs de **response** verificadas (`ConnectionResponse` de channels,
  `ConfigResponse` de providerhub/monitoring, `IntegrationResponse` de
  automation, `SubscriptionResponse` de webhooks, `UserResponse` de iam) **não
  têm campo de segredo** — expõem apenas `HasSecret bool`. Os campos
  `Secret`/`Password` que existem estão em **requests** (entrada).
- **Webhook secret**: exposto **uma única vez** na criação
  (`CreatedResponse{ SubscriptionResponse; Secret }`) e nunca mais —
  `SubscriptionResponse` subsequente só reporta `HasSecret`.
- `webhook_verify_token` na `ConnectionResponse` é um **identificador público de
  verificação** (o admin com `channel.manage` precisa copiá-lo para configurar o
  webhook no provedor externo), não uma credencial cifrada. **Exposição
  intencional**, não é vazamento.

### 17.3 Gate `allow_*_data` do copiloto — **APROVADO**

`domain/copilot/service/context_builder.go` → `Build()`: cada categoria de dado
sensível está guardada **pelo próprio flag**, e o `if` envolve **a própria
busca** — não há o anti-padrão "montar tudo e filtrar depois" (com o flag off, o
provedor de dados nem é consultado):

```go
if cfg.AllowCustomerData   && b.customer   != nil { pc.Customer   = b.customer.Customer(...) }
if cfg.AllowFinancialData  && b.financial  != nil { pc.Financial  = b.financial.Financial(...) }
if cfg.AllowMonitoringData && b.monitoring != nil { pc.Monitoring = b.monitoring.Monitoring(...) }
```

- **Gate por categoria** (não tudo-ou-nada): `policy_test.go` cobre
  `allow_financial_data=false` com os outros `true` → financeiro excluído; e os
  três `false` → contexto sai só com canal + instrução + transcrição do chat.
- **AILog**: `inputSummary()` grava apenas `action=… customer=t financial=f
  monitoring=f` (flags), **sem dado bruto** — comentado *"without any raw data"*.

— Fim do spot-check.
