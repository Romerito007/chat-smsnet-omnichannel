# Webhooks (saída HTTP) — paridade de FORMA com o Chatwoot

Webhooks são entregas HTTP **assinadas** (HMAC) para sistemas externos — **não
confundir** com os eventos de **WebSocket** (tempo real, dashboard; ver
`realtime-events.md`). São sistemas diferentes.

Régua: **FORMA** (nomes de evento + shape do payload) aproxima do Chatwoot quando
barato; **PLUMBING** (rotas, auth, HMAC, multi-tenant, keyset, envelope) é o nosso.

## Convenção e catálogo de eventos

Convenção: **underscore** (Chatwoot), ex. `conversation_created`. (Internamente os
serviços usam chaves com ponto compartilhadas com o realtime; o domínio de webhook
mapeia ponto→underscore na borda — `domain/webhooks/entity`.)

| Evento (wire) | Quando dispara | `data` (payload) | Sector? |
|---|---|---|---|
| `conversation_created` | nova conversa | objeto **conversa** (enriquecido) | sim |
| `conversation_status_changed` | conversa fechada (status muda) | objeto **conversa** (com `status`) | sim |
| `conversation_assigned` | atribuída a agente | objeto **conversa** (enriquecido) | sim |
| `conversation_transferred` | transferida de setor/fila | objeto **conversa** (enriquecido) | sim |
| `conversation_updated` | mudança relevante (PATCH, reabertura) | objeto **conversa** (enriquecido) | sim |
| `message_created` | nova mensagem (não notas internas) | objeto **mensagem** (enriquecido) | sim |
| `message_updated` | edição de mensagem do agente | objeto **mensagem** (enriquecido) | sim |
| `sla_breached` | SLA estourado | `{ conversation_id, policy_id, leg, sector_id, ...due_at, breached }` | sim |

### Equivalência Chatwoot ↔ nosso (guia de migração)
| Chatwoot | Nosso | Igual (forma)? |
|---|---|---|
| `conversation_created` | `conversation_created` | **igual** |
| `conversation_status_changed` | `conversation_status_changed` | **igual** |
| `message_created` | `message_created` | **igual** |
| `conversation_updated` (assign/transfer) | `conversation_assigned` / `conversation_transferred` | **diferente** (somos mais específicos; mesma convenção) |
| — | `sla_breached` | **nosso** (extensão, mesma convenção) |
| — (Chatwoot tem mais: contact_*, webwidget_*) | — | **não emitimos** (ainda) |

## Envelope entregue (nosso contrato, estável)
Toda entrega tem o mesmo invólucro (os bytes exatos são o que é HMAC-assinado):
```json
{ "id": "<delivery_id>", "event": "message_created", "created_at": "2026-...Z",
  "data": { /* objeto conversa | mensagem | run, conforme o evento */ } }
```
Headers: `X-Webhook-Event: <event>`, mais a assinatura HMAC sobre o corpo bruto
(o subscriber valida com o `secret` do webhook). O objeto `data` usa **nossos**
nomes de campo (`id`, `sector_id`, `contact_id`, `status`, `assigned_to`, ...). Um
integrador Chatwoot reconhece os **nomes de evento** e a estrutura
`{event, data:{conversa|mensagem}}` com adaptação mínima de campos.

## Entrega filtrada (confirmado)
- **`events[]`** (enum acima): um webhook só recebe os eventos que **assinou**
  (`ListEnabledByEvent` casa pertinência no array). Evento fora do `events[]` **não
  é entregue** — nem cria registro de delivery.
- **`scopes`** = **ids de setor**. Vazio = **todos os setores**. Com setores
  listados, o webhook só recebe eventos **daquele(s) setor(es)**; eventos sem setor
  (automação) vão **apenas** para webhooks sem `scopes`. (Antes `scopes` era
  ignorado — agora é aplicado.)
- **`GET /v1/webhooks/{id}/deliveries`** reflete **só** o que foi criado/entregue
  (auditoria por tentativa: status, attempts, last_error, next_retry_at).

## Enriquecimento: destinatário (contato) e remetente (agente)

Para o gateway de canal **rotear** uma mensagem (saber para qual JID/número enviar)
sem uma segunda chamada, os payloads de **mensagem** e de **conversa** carregam
blocos de enriquecimento. Eles aparecem **apenas** nos webhooks — o tempo real
(WebSocket) e o motor de automação seguem com o payload enxuto.

- **`contact`** (destinatário): `{ id, name, phone, identities:[{channel,
  external_id}], custom_attributes }`. O gateway roteia pela
  **`contact.identities`** com `channel == "whatsapp"` — o `external_id` é o **JID**
  do WhatsApp (a identity já existente do contato; não há campo novo nem "oculto").
- **`agent`** (só em `message_created`/`message_updated`, **só** quando
  `sender_type == "agent"`): `{ id, name }` — **sem e-mail nem PII**. Ausente para
  mensagens de cliente/automação/sistema e para entrada (inbound).
- **`conversation`** (no payload de mensagem): `{ custom_attributes }` da conversa.
- No payload de **conversa**: `custom_attributes`, `contact` e **`assigned_agent`**
  (`{id,name}`, `null` quando não atribuída — inclusive em inbound, onde o agente
  não é resolvido).

Exemplo (`message_created`, mensagem do agente):
```json
{ "id": "m_123", "conversation_id": "c_1", "sender_type": "agent", "sender_id": "u_9",
  "direction": "outbound", "message_type": "text", "text": "Olá!",
  "attachments": [], "internal": false, "delivery_status": "queued",
  "created_at": "2026-...Z",
  "contact": { "id": "ct_1", "name": "Maria", "phone": "+5511999998888",
    "identities": [ { "channel": "whatsapp", "external_id": "5511999998888@s.whatsapp.net" } ],
    "custom_attributes": { "plano": "gold" } },
  "agent": { "id": "u_9", "name": "João" },
  "conversation": { "custom_attributes": { "ticket": "42" } } }
```

> **Resolução LAZY (custo zero sem assinante):** o contato/agente só são buscados
> **depois** que o dispatcher confirma (via `ListEnabledByEvent`) que existe **ao
> menos uma** assinatura para o evento. Sem webhook inscrito, o builder do payload
> **não roda** — **nenhuma** consulta de contato/agente acontece (crítico no
> `message_created` de entrada, o evento de maior volume). Best-effort: se a
> resolução falhar, o bloco é omitido e a entrega **nunca** quebra.

## Plumbing (nosso, difere do Chatwoot — de propósito)
- Sem `api_access_token` do Chatwoot: criação/edição via `/v1/webhooks` (JWT,
  `webhook.manage`); a entrega é assinada por **HMAC** com o `secret` do webhook
  (exibido **uma vez** na criação).
- Multi-tenant: tudo escopado por tenant do JWT. Retry/backoff + dead-letter,
  rate-limit por webhook, paginação **keyset**. OpenAPI tipa o enum de eventos e o
  `WebhookEnvelope`.
