# chat-smsnet-omnichannel — backend

Backend do chat omnichannel. Monólito modular em **binário único**, com papéis
selecionados em runtime via `RUN_ROLE` (`all | api | ws | worker | scheduler`).

> **Status:** fundação (scaffold). A arquitetura em camadas, os padrões
> transversais e o bootstrap por papel estão implementados e compilando. Os
> domínios de negócio ainda não foram adicionados — há um único ponto de
> extensão em cada camada para incluí-los.

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
make test     # testes
make vet      # go vet
make fmt      # gofmt -s -w .
```
