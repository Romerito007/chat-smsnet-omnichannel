# Estrutura de pastas

Arquitetura em quatro camadas: `domain` / `infra` / `presenter` / `app`. Cada
**domínio** segue o mesmo molde, e o **repositório vive em dois lugares**
(interface em `domain`, implementação Mongo em `infra`).

## Árvore

```
chat-backend/
├── main.go
├── go.mod  go.sum  .env.example  README.md
├── Dockerfile  docker-compose.yml  Makefile      # mongodb, redis (+ app)
│
├── app/                                # composição / wiring
│   ├── run.go
│   ├── config/                         # carga de env → Config
│   ├── container/                      # injeção de dependência (singletons)
│   ├── factories/                      # controllers, services, repositories, adapters
│   ├── health/
│   ├── providers/                      # provisão de recursos compartilhados
│   ├── routes/
│   │   ├── http/                       # um <domínio>_routes.go por domínio
│   │   └── websocket/
│   ├── server/
│   └── start_routines/
│       ├── start.go                    # lê RUN_ROLE e sobe os routines do papel
│       ├── bootstrap_mongodb.go
│       ├── bootstrap_indexes.go        # roda migrations (índices)
│       ├── bootstrap_seeds.go          # tenant + owner + papéis + permissões
│       ├── bootstrap_asynq.go          # client + server + scheduler
│       ├── bootstrap_workers.go        # registra handlers Asynq
│       └── bootstrap_scheduler.go      # jobs periódicos Asynq
│
├── domain/                             # regra de negócio pura (sem driver)
│   ├── shared/                         # ids, paginação (cursor), tenant_scope, logger
│   ├── apperror/  authz/  policy/
│   └── <domínio>/
│       ├── contracts/                  # commands.go, queries.go, events.go, tasks.go
│       ├── entity/
│       ├── repository/                 # INTERFACES de repositório
│       ├── service/                    # regra de negócio + testes
│       └── (events/ provider/ cache/)  # quando o domínio precisar
│
├── infra/
│   ├── database/
│   │   └── mongodb/
│   │       ├── client.go
│   │       ├── models/                 # <domínio>_models.go (structs BSON)
│   │       ├── migrations/             # NNNN_*.go (índices + seeds)
│   │       └── repositories/
│   │           └── <domínio>/          # IMPLEMENTAÇÃO Mongo das interfaces
│   ├── asynq/                          # client, server, filas, middleware de job
│   ├── redis/                          # client, locks, cache, presença
│   ├── realtime/                       # manager, hub, pub/sub transport
│   ├── channels/                       # adapters: api (genérico), whatsapp, webchat + sign/
│   ├── automation/                     # client do flow externo (chamadas + callbacks)
│   ├── providerhub/                    # HTTP client da API smsnet-integrations (sob demanda)
│   ├── copilot/provider/               # adapters de IA REAIS: openai/mistral/deepseek/perplexity, anthropic, gemini (echo só em testes)
│   ├── mcp/                            # cliente MCP Streamable HTTP (JSON-RPC: list_tools / call_tool)
│   ├── webhooks/                       # delivery client + assinatura HMAC
│   ├── email/                          # smtp + templates
│   ├── storage/                        # anexos: S3 (aws-sdk-go-v2, presigned PUT/GET, MinIO/R2) + local
│   ├── secrets/                        # cifragem de segredos
│   └── http_client/
│
├── presenter/
│   ├── contracts/<domínio>/            # DTOs request/response
│   ├── controller/<domínio>/           # controllers + testes
│   ├── http/                           # health
│   ├── middleware/                     # auth, authz, tenant_context, error, request_id,
│   │                                   # idempotency, ratelimit, cors, ws_log_control
│   └── websocket/                      # upgrade + read/write pumps
│
├── docs/                               # esta documentação
├── templates/                          # e-mails
├── regression/                         # testes cross-module
├── utils/  validator/
```

## Molde de um domínio

Para o domínio `conversations` (exemplo), os arquivos se espalham assim:

| Artefato                          | Caminho                                                          |
|-----------------------------------|------------------------------------------------------------------|
| Entidade                          | `domain/conversations/entity/`                                   |
| Commands/queries/events/tasks     | `domain/conversations/contracts/`                                |
| Interface de repositório          | `domain/conversations/repository/`                               |
| Serviço                           | `domain/conversations/service/`                                  |
| Struct BSON                       | `infra/database/mongodb/models/conversations_models.go`          |
| Repositório (impl Mongo)          | `infra/database/mongodb/repositories/conversations/`             |
| Índices/seed                      | `infra/database/mongodb/migrations/NNNN_conversations.go`        |
| DTO request/response              | `presenter/contracts/conversations/`                             |
| Controller                        | `presenter/controller/conversations/`                            |
| Rotas                             | `app/routes/http/conversations_routes.go`                        |
| Handler Asynq                     | registrado em `bootstrap_workers.go` → chama `service`           |
| Wiring                            | `app/container/` + `app/factories/`                              |

## Regras de dependência

- `domain/<x>` importa apenas `domain/shared`, `domain/apperror`, `domain/authz`,
  `domain/policy` e, quando necessário, contratos de outro domínio.
- `infra/...` implementa interfaces de `domain` e nunca o contrário.
- `presenter/...` traduz HTTP/WS ↔ commands/queries do `domain`.
- `app/...` é o único lugar que conhece **todas** as camadas (composição).

## Pontos de extensão (onde “plugar” um domínio novo)

1. `app/routes/http/router.go` → `registerDomainRoutes`.
2. `app/start_routines/bootstrap_workers.go` → `registerHandlers`.
3. `app/start_routines/bootstrap_scheduler.go` → `scheduledJobs`.
4. `app/factories/` → factory do domínio (service + repo + controller).
5. `infra/database/mongodb/migrations/` → nova migration numerada.
