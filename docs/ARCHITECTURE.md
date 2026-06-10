# Arquitetura

Documento de referência da fundação. Resume as convenções e mostra onde cada
peça vive, para que novos domínios sigam o mesmo molde.

## Camadas e direção de dependência

```
presenter ─┐
           ├─→ domain  (regra de negócio pura, sem driver/framework)
infra ─────┘
app ──────────→ tudo (composição / wiring)
```

- `domain/` não importa `infra/`, `presenter/` nem `app/`.
- `infra/` e `presenter/` importam `domain/`.
- `app/` importa todas as camadas para compor o processo.

## Mapa de arquivos por domínio

| Artefato                     | Caminho                                                     |
|------------------------------|------------------------------------------------------------|
| Entidade                     | `domain/<x>/entity/`                                        |
| Commands/queries/events/tasks| `domain/<x>/contracts/`                                     |
| Interface de repositório     | `domain/<x>/repository/`                                    |
| Serviço                      | `domain/<x>/service/`                                       |
| Struct BSON                  | `infra/database/mongodb/models/<x>_models.go`              |
| Repositório (impl Mongo)     | `infra/database/mongodb/repositories/<x>/`                 |
| Índices/seed                 | `infra/database/mongodb/migrations/NNNN_*.go`             |
| DTO request/response         | `presenter/contracts/<x>/`                                 |
| Controller                   | `presenter/controller/<x>/`                               |
| Rotas                        | `app/routes/http/<x>_routes.go`                           |
| Handler Asynq                | registrado em `bootstrap_workers.go` → `domain/<x>/service`|
| Wiring                       | `app/container/` + `app/factories/`                       |

## Papéis (`RUN_ROLE`)

Um único binário; `RUN_ROLE` seleciona os routines que sobem:

| Papel       | Routines                                                       |
|-------------|----------------------------------------------------------------|
| `api`       | servidor HTTP (router + middlewares) + bootstrap (índices/seed) |
| `ws`        | servidor HTTP do WS + loop de pub/sub Redis (fan-out)          |
| `worker`    | servidor Asynq + handlers                                       |
| `scheduler` | scheduler Asynq (jobs periódicos)                              |
| `all`       | todos acima                                                     |

`api` e `ws` compartilham um único listener HTTP quando rodam juntos.

## Padrões transversais

### Paginação por cursor (keyset)
`domain/shared/pagination.go`. Resposta:

```json
{ "data": [ ... ], "page": { "next_cursor": "...", "has_more": true } }
```

O cursor codifica `(created_at, id)` para ordenação total e estável. Os índices
keyset (`tenant_id, created_at desc, _id desc`) estão na migration baseline.

### Envelope de erro
`domain/apperror` define `AppError` (code/message/details + causa interna).
`presenter/middleware/response.go` renderiza:

```json
{ "error": { "code": "...", "message": "...", "details": {}, "request_id": "..." } }
```

Códigos: `validation_error`, `unauthorized`, `forbidden`, `not_found`,
`conflict`, `rate_limited`, `integration_unavailable`, `internal_error`.

### Idempotency-Key
`presenter/middleware/idempotency.go`. Em `POST`, a chave + hash do payload +
resposta ficam no Redis com TTL. Mesma chave e payload → replay; payload
diferente → `conflict`.

### Observabilidade
`presenter/middleware/request_id.go` garante `request_id` em todo request,
ecoa no header e loga método/rota/status/duração. O hook de OpenTelemetry pode
ser plugado no mesmo ponto (`OTEL_ENABLED`).

### Tenant scope
`domain/shared/tenant.go` carrega `tenant_id` e o `Actor` no context.
`RequireTenant` impõe o escopo nos repositórios/serviços.

## Filas e jobs Asynq

Filas e prioridades em `app/config` (`ASYNQ_QUEUE_*`); nomes de task em
`infra/asynq/queues.go`. Retentativa/backoff são do próprio Asynq; falha
terminal vai para a dead-letter do Asynq. Jobs periódicos (multi-tenant e
idempotentes) em `bootstrap_scheduler.go`.

## Integrações sob demanda

`providerhub` e `monitoring` são **consulta sob demanda**: sem sync, sem
ingestão em tempo real, sem persistir payload externo completo. Os adapters
ficam em `infra/providerhub/` e `infra/monitoring/` (a implementar).
