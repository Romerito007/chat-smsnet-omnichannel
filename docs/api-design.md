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

### contacts
```
GET    /contacts                   POST   /contacts
GET    /contacts/{id}              PATCH  /contacts/{id}     DELETE /contacts/{id}
POST   /contacts/{id}/merge        # dedupe/merge
GET    /contacts/{id}/conversations
```

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
GET    /conversations/{id}/messages          # timeline (keyset)
POST   /conversations/{id}/messages          # enviar (text/template/attachment)
POST   /conversations/{id}/messages/{mid}/read
POST   /conversations/{id}/typing            # (também via WS)
POST   /conversations/{id}/notes             # nota interna
```

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

### providerhub / monitoring (consulta sob demanda)
```
GET    /providerhub/...             # proxy/consulta (sem persistir payload completo)
GET    /monitoring/...              # consulta de métricas/saúde externas
```

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
