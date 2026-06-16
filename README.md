# chat-smsnet-omnichannel — backend

Backend do chat omnichannel. Monólito modular em **binário único**, com papéis
selecionados em runtime via `RUN_ROLE` (`all | api | ws | worker | scheduler`).

> **Status:** MVP completo. A arquitetura em camadas, os padrões transversais e
> todos os domínios de negócio estão implementados, com testes (unitários +
> integração com Mongo/Redis sob a tag de build `e2e`).

## Domínios implementados

`auth` · `iam` (users/roles/permissões) · `tenant` · `sectors` · `queues` ·
`presence` · `conversations` (mensagens/notas/timeline) · `routing`
(atribuição/transferência) · `channels` (inbound/outbound) · `contacts` ·
`automation` · `providerhub` (smsnet-integrations sob demanda) ·
`webhooks` · `copilot` (IA) · `mcp` (ferramentas externas via MCP) ·
`conversationtools` (tags/respostas/motivos) ·
`businesshours` · `sla` · `notifications` · `csat` · `search` · `reports` ·
`privacy` (LGPD) · `audit` · `attachments`.

### Audit (trilha de auditoria)

`AuditLog` (`tenant_id, actor_id, actor_type, action, resource_type,
resource_id, ip, user_agent, data, created_at`) — `domain/audit`. O `ip` e o
`user_agent` são capturados na borda HTTP (`request_id` middleware) e o ator é
resolvido do token; serviços auditam via a porta `shared.Auditor`. Ações
auditadas: `auth.login`/`auth.logout`, `user.*`, `role.*` (alteração de
permissões), `webhook.*`, `conversation.closed`/`conversation.transferred`,
`providerhub.liberacao`/`providerhub.chamado`,
`ai.config.updated`, `privacy.*` (anonimização/exportação) e `report.export`.
Consulta: `GET /v1/audit` (permissão `audit.view`).

### Attachments (anexos por URL assinada)

Fluxo: `POST /v1/attachments/upload-url` (valida `content_type` + `size`, gera
`storage_key` + URL assinada) → upload direto no storage → `POST
/v1/attachments/confirm` (cria/atualiza o registro e vincula à mensagem) →
`GET /v1/attachments/{id}/download` (valida acesso à conversa e serve/redireciona
por URL assinada curta). Adapter de storage em `infra/storage` com backends
**local** (sink `PUT /v1/attachments/blobs/{token}` com token HMAC) e
**S3-compatível** (URLs pré-assinadas AWS SigV4, sem SDK). Selecionável por
`ATTACHMENTS_PROVIDER=local|s3`.

## Arquitetura em camadas

| Camada       | Responsabilidade                                                        |
|--------------|-------------------------------------------------------------------------|
| `domain/`    | Regra de negócio pura: entidades, contratos, interfaces, serviços.      |
| `infra/`     | Implementações concretas: Mongo, Redis, Asynq, realtime, clientes HTTP. |
| `presenter/` | Borda HTTP/WS: DTOs, controllers, middlewares.                          |
| `app/`       | Composição: config, container (DI), factories, rotas, start routines.   |

Fluxo de boot: `main.go → app.Run → start_routines.Start` lê `RUN_ROLE` e sobe
os routines do papel.

## Decisões fixas

- Fila assíncrona: **Asynq** (Redis).
- Tempo real: **WebSocket + Redis Pub/Sub**.
- Banco: **MongoDB**. Cache/presença/locks/rate limit: **Redis**.
- Toda entidade respeita `tenant_id`.

## Padrões transversais (já implementados)

- **Paginação por cursor (keyset)** — `domain/shared/pagination.go`.
- **Envelope de erro padrão** — `domain/apperror` + `presenter/middleware/response.go`.
- **Idempotency-Key** em POST — `presenter/middleware/idempotency.go` (Redis, TTL).
- **request_id + duração** em todo request — `presenter/middleware/request_id.go`.
- **OpenTelemetry** (traces + métricas) — `app/providers/observability.go` +
  middleware `presenter/middleware/telemetry.go` (gated por `OTEL_ENABLED`).
- **Rate limit** por tenant/IP — `presenter/middleware/ratelimit.go` (Redis).
- **Recovery** (panic → 500 padronizado) — `presenter/middleware/recover.go`.
- **CORS configurável** (`HTTP_ALLOWED_ORIGINS`) — `presenter/middleware/cors.go`.
- **Timeouts HTTP** (read/read-header/write/idle/shutdown) — `app/server/server.go`.
- **Validação de payload** (JSON estrito, sem campos desconhecidos) —
  `presenter/middleware/request.go`.
- **Seed inicial idempotente** (tenant + owner + papéis + permissões) —
  `app/start_routines/bootstrap_seeds.go`.
- **Índices centralizados e numerados** — `infra/database/mongodb/migrations`.

## Filas Asynq

| Fila       | Prioridade |
|------------|-----------|
| `critical` | 6         |
| `default`  | 3         |
| `channels` | 3         |
| `webhooks` | 2         |
| `ai`       | 2         |
| `reports`  | 1         |

Nomes de tasks e tipos de job ficam em `infra/asynq/queues.go`. Handlers são
registrados em `app/start_routines/bootstrap_workers.go`; jobs periódicos em
`bootstrap_scheduler.go`.

## Rodando localmente

```bash
cp .env.example .env
make docker-up        # sobe mongodb + redis
make seed             # cria tenant + owner + papéis (idempotente)
make run              # RUN_ROLE=all por padrão
```

O seed também roda automaticamente no boot dos papéis `api`/`all`; `make seed`
(equivalente a `chat-backend seed`) é o atalho para rodá-lo isoladamente.

> **⚠️ Pré-requisito de infra (presença ao vivo):** o Redis precisa emitir
> *keyspace expired events* (`notify-keyspace-events` contendo `E`+`x`), senão
> agentes que caem de forma abrupta continuam "Online" no painel até um refresh.
> O evento é **por database** — o subscriber escuta `__keyevent@{REDIS_DB}__:expired`,
> então mantenha o `REDIS_DB` consistente. Como habilitar (e como **não**
> sobrescrever uma config existente): ver **[docs/operations.md](docs/operations.md)**.

Endpoints de saúde:

```bash
curl localhost:8080/healthz   # liveness
curl localhost:8080/readyz    # readiness (pinga mongo + redis)
```

Papéis individuais:

```bash
make run-api
make run-ws
make run-worker
make run-scheduler
```

## Como adicionar um domínio

Cada domínio segue o molde `contracts/ + entity/ + repository/ + service/`:

1. `domain/<x>/entity/` — entidade pura.
2. `domain/<x>/contracts/` — `commands.go`, `queries.go`, `events.go`, `tasks.go`.
3. `domain/<x>/repository/` — **interface** do repositório.
4. `domain/<x>/service/` — regra de negócio (+ testes).
5. `infra/database/mongodb/models/<x>_models.go` — struct BSON.
6. `infra/database/mongodb/repositories/<x>/` — **implementação** Mongo.
7. `infra/database/mongodb/migrations/NNNN_*.go` — índices/seed.
8. `presenter/contracts/<x>/` e `presenter/controller/<x>/` — DTOs e controller.
9. `app/routes/http/<x>_routes.go` — rotas; registre em `registerDomainRoutes`.
10. Handlers Asynq — registre em `registerHandlers` (`bootstrap_workers.go`).
11. Wiring — `app/factories/` + `app/container/`.

Veja `docs/architecture.md` (e os demais documentos em `docs/`) para detalhes.

## Comandos úteis

```bash
make build    # compila o binário em bin/
make test     # testes unitários
make vet      # go vet
make fmt      # gofmt -s -w .

# testes de integração (precisam de Mongo em :27017 e Redis em :6379):
go test -tags e2e ./...
```

Exemplos de uso da API (curl) em [`docs/api-examples.md`](docs/api-examples.md).

## Contrato da API (OpenAPI)

O contrato de **todas** as rotas `/v1` (request/response reais, enums, envelope
de erro, paginação por cursor, auth Bearer JWT) é descrito em **OpenAPI 3.1**:

- **`GET /openapi.json`** — servido pelo backend (público em dev; atrás de HTTP
  basic auth em produção via `OPENAPI_BASIC_USER`/`OPENAPI_BASIC_PASS`).
- **[`docs/openapi.yaml`](docs/openapi.yaml)** — cópia versionada, fonte para o
  frontend gerar um cliente tipado.

A fonte única é Go (`presenter/openapi`); regenere a cópia com
`go run ./cmd/openapigen` (o teste `TestDocsCopyInSync` falha se ficar
desatualizada). Eventos WebSocket e handshake em
[`docs/realtime-events.md`](docs/realtime-events.md).
