# Modelo de dados

Persistência em **MongoDB** (uma ou mais collections por domínio) e estado
volátil em **Redis**. Convenções:

- **Multi-tenant:** todo documento persistido tem `tenant_id`. Toda query é
  escopada por `tenant_id`.
- **Base comum:** `_id` (UUID string), `tenant_id`, `created_at`, `updated_at`.
- **Keyset/paginação:** índice composto `(tenant_id, created_at desc, _id desc)`
  nas collections paginadas.
- **IDs:** UUIDv4 em string (domínio independente do driver; nada de
  `ObjectID` vazando para o domínio).
- **Soft delete:** `deleted_at` (nullable) onde retenção/auditoria exigem.
- **Índices** ficam em `infra/database/mongodb/migrations` (numerados,
  idempotentes).

Notação: 🔑 = índice; ⭐ = único.

---

## Núcleo

### `tenants`
| Campo | Tipo | Notas |
|---|---|---|
| `_id` | string | tenant id |
| `name` | string | ⭐🔑 |
| `status` | string | active/suspended |
| `plan` | string | |
| `settings` | object | flags, limites, fuso padrão |
| `created_at`/`updated_at` | date | |

### `users` (iam)
| Campo | Tipo | Notas |
|---|---|---|
| `_id` | string | |
| `tenant_id` | string | 🔑 |
| `email` | string | ⭐ por tenant `(tenant_id, email)` |
| `name` | string | |
| `password_hash` | string | argon2id/bcrypt |
| `roles` | []string | nomes de papéis |
| `sectors` | []string | setores do usuário |
| `status` | string | active/disabled |
| `last_seen_at` | date | |

Índices: ⭐`(tenant_id,email)`, 🔑`(tenant_id,created_at desc,_id desc)`.

### `roles` (iam)
| Campo | Tipo | Notas |
|---|---|---|
| `tenant_id` | string | 🔑 |
| `name` | string | ⭐ `(tenant_id,name)` |
| `permissions` | []string | catálogo em security-permissions.md |

> Permissões podem ser uma collection (`permissions`) somente-leitura/seed ou
> uma constante de código. **Decisão pragmática:** catálogo em código (seed),
> `roles.permissions` referencia por string.

### `sessions` / `refresh_tokens` / `api_keys` (auth)
- `sessions`: `tenant_id`, `user_id`🔑, `ip`, `user_agent`, `expires_at`(TTL).
- `refresh_tokens`: `tenant_id`, `user_id`, `token_hash`⭐, `expires_at`(TTL),
  `revoked_at`.
- `api_keys`: `tenant_id`, `name`, `key_hash`⭐, `scopes`, `last_used_at`.

> `expires_at` com **TTL index** para expurgo automático.

---

## Atendimento

### `contacts`
| Campo | Tipo | Notas |
|---|---|---|
| `tenant_id` | string | 🔑 |
| `name` | string | text index |
| `identities` | [] | `{ channel, value }` (telefone, @user, e-mail) |
| `custom_fields` | object | |
| `tags` | []string | |
| `anonymized_at` | date | privacy |

Índices: 🔑`(tenant_id, identities.channel, identities.value)` (lookup inbound),
text `(name)`, 🔑 keyset.

### `sectors`
`tenant_id`🔑, `name`⭐`(tenant_id,name)`, `description`.

### `queues`
`tenant_id`🔑, `sector_id`🔑, `name`, `strategy` (round_robin/least_busy/manual),
`capacity`, `member_user_ids` []string.

### `conversations`
| Campo | Tipo | Notas |
|---|---|---|
| `tenant_id` | string | 🔑 |
| `contact_id` | string | 🔑 |
| `channel_id` | string | 🔑 |
| `sector_id` / `queue_id` | string | 🔑 |
| `assignee_id` | string | 🔑 agente atual |
| `status` | string | open/assigned/pending/resolved/closed |
| `priority` | string | |
| `subject` | string | |
| `tags` | []string | |
| `reason_id` | string | motivo de fechamento |
| `last_message_at` | date | 🔑 ordenação inbox |
| `unread_count` | int | por participante (ver nota) |
| `sla` | object | snapshot de deadlines |
| `closed_at` | date | |

Índices: 🔑`(tenant_id,status,last_message_at desc)` (inbox),
🔑`(tenant_id,assignee_id,status)`, 🔑`(tenant_id,queue_id,status)`,
🔑`(tenant_id,contact_id)`, keyset.

### `messages`
| Campo | Tipo | Notas |
|---|---|---|
| `tenant_id` | string | 🔑 |
| `conversation_id` | string | 🔑 |
| `direction` | string | inbound/outbound |
| `author` | object | `{ type: contact|agent|system|bot, id }` |
| `type` | string | text/image/file/audio/template/event |
| `body` | string | text index |
| `attachments` | []string | ids de `attachments` |
| `channel_message_id` | string | id externo p/ dedupe/status |
| `status` | string | sent/delivered/read/failed |
| `internal` | bool | nota interna (não vai ao contato) |

Índices: 🔑`(tenant_id,conversation_id,created_at desc,_id desc)` (timeline),
🔑`(tenant_id,channel_message_id)` (dedupe/status), text `(body)`.

> **unread/leitura:** manter `read_receipts` por participante embutido na
> conversa **ou** collection `message_reads`. **Decisão pragmática:** começar
> com `unread_count` por conversa + `last_read_message_id` por participante
> (embutido); evoluir se necessário.

### `conversationtools`
- `tags`: `tenant_id`🔑, `name`⭐`(tenant_id,name)`, `color`.
- `canned_responses`: `tenant_id`🔑, `shortcut`🔑, `title`, `body`, `sector_id?`.
- `reasons`: `tenant_id`🔑, `name`, `kind` (close/transfer/...).

### `routing_rules`
`tenant_id`🔑, `priority` int, `conditions` (canal/keyword/horário),
`action` (sector/queue/agent), `enabled` bool.

### `assignments` (histórico de atribuição)
`tenant_id`🔑, `conversation_id`🔑, `from`, `to`, `by`, `at`, `reason`.

---

## Canais e integrações

### `channel_connections`
`tenant_id`🔑, `type` (api/whatsapp/webchat/…), `name`,
`status` (connected/disconnected/error), `base_url` (= `outbound_url` no canal
api), `auth_type`, `encrypted_secret` (= `outbound_secret` cifrado AES-GCM,
**nunca** em claro nem retornado após criação), `webhook_verify_token` (=
`inbound_token`, exibido uma vez), `default_sector_id`, `enabled`,
`automation_enabled`.

### `automation_bindings` / `automation_executions` / `automation_logs`
- `automation_bindings`: `tenant_id`🔑, `trigger`, `flow_ref` (id no flow
  externo), `enabled`.
- `automation_executions`: `tenant_id`🔑, `conversation_id`🔑,
  `external_execution_id`, `status` (invoked/callback_received/failed),
  `started_at`, `finished_at`.
- `automation_logs`: `tenant_id`🔑, `execution_id`🔑, `level`, `message`, `at`.

> **providerhub**: persiste apenas a config (`providerhub_configs`:
> `smsnet_base_url`, `encrypted_api_key`, `isp_type`, `encrypted_credentials`,
> `bot_id`, `options`, `enabled`, `timeout_ms`) e o log técnico mínimo
> (`provider_query_logs`, **sem** `response_body`). As consultas são sob demanda
> à API smsnet-integrations; o payload externo **não** é persistido.

### `copilot_runs`
`tenant_id`🔑, `conversation_id`🔑, `kind` (suggest/summarize/classify),
`provider`, `input_ref`, `output`, `tokens`, `latency_ms`, `status`.

---

## QoS e engajamento

### `sla_policies` / `sla_trackers`
- `sla_policies`: `tenant_id`🔑, `name`, `target` (first_response/resolution),
  `duration`, `applies_to` (sector/queue/priority).
- `sla_trackers`: `tenant_id`🔑, `conversation_id`⭐, `policy_id`,
  `deadlines` (first_response_at, resolution_at), `status`
  (ok/warning/breached), `paused` (fora de expediente).

Índice: 🔑`(tenant_id,status,deadlines.next desc)` p/ varredura do scheduler.

### `csat_surveys` / `csat_responses`
- `csat_surveys`: `tenant_id`🔑, `conversation_id`🔑, `sent_at`, `expires_at`(TTL),
  `status` (pending/answered/expired).
- `csat_responses`: `tenant_id`🔑, `survey_id`⭐, `score` int, `comment`,
  `answered_at`.

### `business_hours` / `holidays`
- `business_hours`: **não é coleção própria** — vive embutido na
  `ChannelConnection` (campo `business_hours`). Shape: `timezone` (IANA) +
  `weekly` (lista de `{day: 0..6, intervals: [{start:"HH:MM", end:"HH:MM"}]}`).
  Vários intervalos no mesmo dia modelam o almoço; `end > start` (sem cruzar
  meia-noite). Documento vazio/ausente = aberto 24/7. Avaliado na timezone do
  channel.
- `holidays`: `tenant_id`🔑, `date`, `name`, `recurring` bool, `scope`
  (`all_channels` | `channels`) e `channel_ids[]` (escopo por **channel**, não
  por setor). Um feriado fecha o dia quando seu escopo é `all_channels` ou sua
  lista inclui o `channel_id` da conversa.

---

## Plataforma

### `webhook_subscriptions` / `webhook_deliveries`
- `webhook_subscriptions`: `tenant_id`🔑, `url`, `events` []string,
  `secret_ref` (HMAC), `enabled`.
- `webhook_deliveries`: `tenant_id`🔑, `subscription_id`🔑, `event`,
  `status` (pending/success/failed), `attempts`, `response_code`,
  `next_retry_at`🔑.

### `notifications` / `notification_preferences`
- `notifications`: `tenant_id`🔑, `user_id`🔑, `type`, `payload`,
  `read_at`, keyset.
- `notification_preferences`: `tenant_id`🔑, `user_id`⭐, `channels`
  (in_app/email/push), `mute` regras.

### `attachments`
`tenant_id`🔑, `conversation_id?`🔑, `storage_key`, `filename`, `content_type`,
`size`, `checksum`, `status` (uploaded/processing/ready/blocked),
`scan_result`, `thumbnail_key`.

### `privacy_requests` / `retention_policies`
- `privacy_requests`: `tenant_id`🔑, `subject` (contact_id), `type`
  (export/erase), `status`, `requested_by`, `completed_at`.
- `retention_policies`: `tenant_id`🔑, `entity` (conversations/messages/...),
  `ttl_days`.

### `audit_logs`
`tenant_id`🔑, `actor` (user/api_key/system), `action`, `entity`, `entity_id`,
`before`, `after`, `request_id`, `at`. Índice: 🔑 keyset; opcional
**TTL/capped** por política de retenção (ver `audit.compact`).

### `_migrations`
Controle interno do runner: `_id` (version int), `name`, `applied_at`.

---

## Redis (estado volátil e infraestrutura)

| Chave (prefixo)                              | Uso                                    | TTL |
|----------------------------------------------|----------------------------------------|-----|
| `presence:{tenant}:{user}`                   | status/capacidade do agente            | heartbeat |
| `typing:{tenant}:{conversation}:{user}`      | indicador “digitando”                  | curto |
| `ratelimit:{tenant}:{actor}`                 | contador de rate limit                 | janela |
| `idem:{tenant}:{key}`                        | resposta idempotente (POST)            | 24h |
| `lock:{recurso}`                             | locks distribuídos (ex.: routing)      | curto |
| `cache:{domínio}:{...}`                      | caches de leitura (ex.: providerhub)   | curto |
| `realtime:fanout`                            | canal Pub/Sub de fan-out WS            | — |
| `asynq:*`                                    | filas/agenda do Asynq                  | — |

> **Presence é Redis-only**: nada em Mongo. Heartbeats renovam o TTL; a queda do
> agente expira a presença automaticamente.

---

## Notas de modelagem

- **Mensagens vs. conversas:** collections separadas; `conversations` mantém
  apenas o resumo/última mensagem para inbox rápido.
- **Embutir vs. referenciar:** participantes e leitura embutidos na conversa
  (poucos por conversa); mensagens referenciadas (crescem sem limite).
- **Eventos do sistema** (transferência, fechamento) podem ser `messages` com
  `type=event` para timeline unificada **ou** apenas auditoria. **Decisão
  pragmática:** ambos — evento na timeline (`type=event`) + `audit_log`.
