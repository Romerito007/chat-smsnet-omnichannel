# Design da API REST

API REST sob `/api`, servida pelo papel **`api`**. Convenções uniformes para
todos os domínios; o tempo real fica no WebSocket (ver
[realtime-events.md](realtime-events.md)).

## Convenções gerais

- **Base:** `/api/v1`. Versão no path.
- **Formato:** JSON; `Content-Type: application/json; charset=utf-8`.
- **Auth:** `Authorization: Bearer <jwt>` (ou `X-Api-Key` para serviço). Claims
  resolvem `tenant_id` + `user_id` + `roles`. Ver
  [security-permissions.md](security-permissions.md).
- **Tenant:** derivado do token (não confiar em header do cliente em produção).
- **Recursos:** substantivos no plural, kebab/ös minúsculo
  (`/canned-responses`).
- **Métodos:** `GET` (ler), `POST` (criar), `PATCH` (atualizar parcial), `PUT`
  (substituir), `DELETE` (remover/soft-delete).

### Paginação (cursor / keyset)

```
GET /api/v1/conversations?limit=25&cursor=<opaque>
```
Resposta:
```json
{
  "data": [ /* ... */ ],
  "page": { "next_cursor": "eyJ0Ijo...", "has_more": true }
}
```
- `limit` default 25, máx 100. `cursor` opaco (base64 de `created_at,_id`).

### Filtros e ordenação

- Filtros como query params (`?status=open&assignee_id=...&queue_id=...`).
- Ordenação padrão por `created_at desc` (ou `last_message_at` no inbox).

### Idempotência

- `POST` que cria recurso aceita `Idempotency-Key: <uuid>`. Mesma chave +
  payload → replay da resposta; payload diferente → `409 conflict`.

### Envelope de erro

```json
{
  "error": {
    "code": "validation_error",
    "message": "validation failed",
    "details": { "email": "must be a valid email" },
    "request_id": "..."
  }
}
```

| code | HTTP |
|---|---|
| `validation_error` | 400 |
| `unauthorized` | 401 |
| `forbidden` | 403 |
| `not_found` | 404 |
| `conflict` | 409 |
| `rate_limited` | 429 |
| `integration_unavailable` | 502 |
| `internal_error` | 500 |

### Cabeçalhos transversais

| Header | Direção | Uso |
|---|---|---|
| `Authorization` / `X-Api-Key` | req | auth |
| `Idempotency-Key` | req | POST idempotente |
| `X-Request-Id` | req/resp | correlação (gerado se ausente) |
| `Retry-After` | resp | em `429` |
| `Idempotent-Replay` | resp | `true` quando resposta foi replay |

### Saúde (fora de `/api`)

- `GET /healthz` — liveness.
- `GET /readyz` — readiness (pinga Mongo + Redis).

---

## Endpoints por domínio

> Lista de referência (MVP + evolução). `{id}` = identificador do recurso.
> Todas as rotas abaixo são tenant-scoped e exigem permissão (ver matriz em
> security-permissions.md).

### auth
```
POST   /auth/login                 # email+senha → access+refresh
POST   /auth/refresh               # refresh → novo access
POST   /auth/logout                # revoga sessão/refresh
POST   /auth/password/forgot       # envia e-mail de reset
POST   /auth/password/reset        # aplica novo password
GET    /auth/me                    # perfil + permissões do ator
```

### iam (users, roles)
```
GET    /users                      POST   /users
GET    /users/{id}                 PATCH  /users/{id}        DELETE /users/{id}
POST   /users/{id}/roles           # atribui papéis
GET    /roles                      POST   /roles
GET    /roles/{id}                 PATCH  /roles/{id}        DELETE /roles/{id}
GET    /permissions                # catálogo (read-only)
```

### tenant
```
GET    /tenant                     PATCH  /tenant            # settings do tenant atual
```

### contacts (CRM)
```
GET    /contacts                   # lista paginada por cursor; ?q= (name/phone/document/email) — contact.read
POST   /contacts                   # criar — contact.write; dedup por documento/telefone -> 409
GET    /contacts/{id}              # leitura — contact.read
PATCH  /contacts/{id}              # editar (parcial) — contact.write; dedup -> 409
DELETE /contacts/{id}              POST /contacts/{id}/merge   # design futuro
```
`Contact = { id, tenant_id, name, phones[], document?, email?,
external_ids[]{channel, external_id}, tags[], notes?, created_at, updated_at }`.
Só campos **locais** (nunca dados enriquecidos do provider). Tenant do token;
`create`/`update` auditados (`contact.created`/`contact.updated`).

- **Criar/editar** aceitam `phones[]` (normalizadas para dígitos; a 1ª vira a
  primária), `document`, `email`, `external_ids[]`, `tags[]`, `notes`.
- **Histórico do contato:** use `GET /conversations?contact_id={id}`.
- **Iniciar conversa:** `POST /conversations` aceita o `contact_id` recém-criado.

### sectors / queues
```
GET/POST/PATCH/DELETE  /sectors[/{id}]
GET/POST/PATCH/DELETE  /queues[/{id}]
POST   /queues/{id}/members        DELETE /queues/{id}/members/{userId}
GET    /queues/{id}/stats          # agregado (também via WS)
```

### conversations / messages
```
GET    /conversations                      # inbox (filtros: status, assignee, queue, tag)
POST   /conversations                      # iniciar (outbound)
GET    /conversations/{id}
PATCH  /conversations/{id}                  # status, priority, tags, subject
POST   /conversations/{id}/assign           # atribuir a agente/fila
POST   /conversations/{id}/transfer         # transferir setor/fila/agente
POST   /conversations/{id}/resolve          # resolver (+ reason)
POST   /conversations/{id}/close            # fechar
POST   /conversations/{id}/reopen
GET    /conversations/{id}/messages          # mensagens do fio (keyset)
GET    /conversations/{id}/events             # timeline de ciclo de vida/automação (keyset)
POST   /conversations/{id}/messages          # enviar (text/template/attachment)
POST   /conversations/{id}/read              # marca lida → zera unread_count, grava last_read_at
POST   /conversations/{id}/typing            # (também via WS)
POST   /conversations/{id}/internal-notes    # nota interna
```
**Não-lido:** `Conversation` expõe `unread_count` (incrementado por mensagem
inbound do cliente) e `last_read_at` (gravado no `POST /read`); ambos refletem em
`conversation.updated`.

**Timeline vs mensagens (decisão):** eventos estruturados de ciclo de
vida/automação/conexão são persistidos como `ConversationEvent` na coleção
`conversation_events` — **não** como mensagens — e lidos por
`GET /conversations/{id}/events`. *Mensagens de sistema* (`sender_type=system`,
`message_type=system`), quando enviadas ao fio, são mensagens reais e aparecem em
`GET /messages`. Ver também `docs/realtime-events.md`.

### conversationtools
```
GET/POST/PATCH/DELETE  /tags[/{id}]
GET/POST/PATCH/DELETE  /canned-responses[/{id}]
GET/POST/PATCH/DELETE  /reasons[/{id}]
```

### routing
```
GET/POST/PATCH/DELETE  /routing/rules[/{id}]
POST   /routing/rules/reorder
```

### presence
```
GET    /presence                   # agentes online do tenant (agregado)
PUT    /presence/me                 # define status (online/away/busy)
# heartbeat e typing preferencialmente via WS
```

### channels
```
GET/POST/PATCH/DELETE  /channels[/{id}]
GET    /channels/{id}/health
POST   /channels/{id}/test
POST   /channels/{id}/webhook       # inbound do provedor (assinado)
```

### automation (integra flow externo)
```
GET/POST/PATCH/DELETE  /automation/bindings[/{id}]
POST   /automation/executions       # disparar flow externo
GET    /automation/executions/{id}
POST   /automation/callbacks         # callback do flow externo (assinado)
GET    /automation/executions/{id}/logs
```

### providerhub (smsnet-integrations, consulta sob demanda)
```
# Config (integration.read / integration.configure)
GET    /providerhub/config
POST   /providerhub/config
PATCH  /providerhub/config
POST   /providerhub/config/test

# Sob a conversa — não persiste payload externo
GET    /conversations/{id}/external/cliente   # ?cpfcnpj|phone|email&id_cliente=  (integration.read; faturas omitidas sem contact.view_financial)
GET    /conversations/{id}/external/planos    # integration.read
GET    /conversations/{id}/external/empresa   # integration.read
POST   /conversations/{id}/external/liberacao # {id_cliente} (integration.execute_action; auditado)
POST   /conversations/{id}/external/chamado   # {id_cliente, subject, message} (integration.execute_action; auditado)
```
**Customer 360 (tipado no OpenAPI):** as respostas têm schemas reais —
`Cliente { nome, cpfcnpj, contrato_status_display, valor_check_out, faturas[] }`,
`Fatura { valor, vencimento, link, linha_digitavel, pix }`,
`Plano { nome, valor, velocidade, descricao }`,
`Empresa { nome, cnpj, telefone, email, endereco, site }`.

`external/cliente` retorna **`ClienteResult`**, modelado como **`oneOf`** para o
front gerar o seletor de contrato:
- `{ needs_selection: true, options: [{ id_cliente, label, endereco?, status? }] }`
  → o cliente tem mais de um contrato; renderize um seletor e **re-consulte** com
  `?id_cliente=<options[].id_cliente>` (este é o caso “needs_input”);
- `{ needs_selection: false, cliente: Cliente }` → cliente resolvido.

### copilot
```
POST   /copilot/suggest            # sugestão de resposta (sync curto ou job)
POST   /copilot/summarize          # resumo da conversa
POST   /copilot/classify           # intenção/sentimento
GET    /copilot/runs/{id}
```

### sla / csat / businesshours
```
GET/POST/PATCH/DELETE  /sla/policies[/{id}]
GET    /sla/conversations/{id}      # estado do relógio

POST   /csat/conversations/{id}/send
GET    /csat/responses              # agregação/listagem
POST   /csat/responses              # coleta (link público assinado)

GET/POST/PATCH/DELETE  /business-hours[/{id}]
```

### webhooks / notifications
```
GET/POST/PATCH/DELETE  /webhooks[/{id}]
GET    /webhooks/{id}/deliveries
POST   /webhooks/{id}/test

GET    /notifications               # do ator (keyset)
POST   /notifications/{id}/read
GET/PUT /notifications/preferences
```

### search
```
GET    /search?q=...&type=conversations|contacts|messages&...
```

### privacy / audit
```
POST   /privacy/requests            # export | erase
GET    /privacy/requests/{id}
GET    /audit                       # leitura (filtros: actor, action, entity)
```

### attachments
```
POST   /attachments                 # upload (multipart) → cria registro + job
GET    /attachments/{id}
GET    /attachments/{id}/url        # URL assinada
```

---

## Notas

- **Endpoints públicos assinados:** webhook inbound de canal, callback de
  automation e coleta de CSAT usam assinatura (HMAC) em vez de JWT.
- **Bulk/admin:** operações em lote ficam fora do MVP, exceto onde citado.
- **Rate limit:** aplicado por tenant+ator; limites finos por rota podem ser
  adicionados via `policy`.
