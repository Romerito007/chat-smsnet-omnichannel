# Arquitetura

Sistema de **atendimento via chat**, **multiempresa (multi-tenant)**,
**multiusuário** e **em tempo real**. Monólito modular em **binário único**, com
papéis selecionados em runtime via `RUN_ROLE`.

- **Stack:** Go, MongoDB, Redis, [Asynq](https://github.com/hibiken/asynq),
  WebSocket, REST, Docker Compose.
- **Camadas:** `domain` / `infra` / `presenter` / `app`.
- **Isolamento:** toda entidade respeita `tenant_id`.

---

## 1. Visão geral

Operadores (agentes, supervisores, admins) de várias empresas atendem contatos
que chegam por múltiplos canais (WhatsApp, Telegram, webchat, …). Cada empresa é
um **tenant** isolado. As conversas são roteadas para **setores/filas** e
distribuídas a **agentes**, com presença, SLA, CSAT, automações externas,
copiloto de IA, webhooks e auditoria.

```
                        ┌───────────────────────────────────────────────┐
   Canais externos      │                  Backend (1 binário)          │
   WhatsApp/Telegram ──▶│  ┌────────┐  ┌────────┐  ┌────────┐ ┌────────┐ │
   Webchat / API        │  │  api   │  │   ws   │  │ worker │ │schedule│ │
                        │  └───┬────┘  └───┬────┘  └───┬────┘ └───┬────┘ │
   Operadores (SPA) ───▶│      │ REST      │ WS        │ Asynq    │ cron │
                        │      └─────┬─────┴─────┬─────┴────┬─────┘      │
                        │            │           │          │           │
                        │        MongoDB       Redis     Asynq(Redis)   │
                        └───────────────────────────────────────────────┘
   Integração sob demanda: providerhub (smsnet-integrations)  │  Flow externo: automation
```

### Princípios

1. **Camadas com dependência unidirecional.** `domain` não conhece driver,
   framework nem transporte.
2. **Multi-tenant em tudo.** Todo documento carrega `tenant_id`; toda query é
   escopada; chaves de cache/locks/rate-limit são prefixadas por tenant.
3. **Tempo real de primeira classe.** Mudanças de estado emitem eventos
   WebSocket via Redis Pub/Sub, escalável horizontalmente.
4. **Trabalho pesado é assíncrono.** Entrega de canal, webhooks, IA, e-mail,
   relatórios e rotinas periódicas rodam no Asynq.
5. **Integração externa sob demanda.** `providerhub` consulta a API
   **smsnet-integrations** — *consulta/ação sob demanda*, sem sync, sem ingestão
   em tempo real, sem persistir payload externo. `automation` apenas integra com
   o flow externo já existente (chamadas + callbacks + logs).
6. **Idempotência e observabilidade** por padrão (request_id, Idempotency-Key,
   migrations/seeds idempotentes, jobs idempotentes).

---

## 2. Camadas

```
presenter ─┐
           ├─→ domain   (regra de negócio pura: entidades, contratos, interfaces, serviços)
infra ─────┘
app ───────────→ tudo   (composição / wiring)
```

| Camada       | Responsabilidade                                                                 | Pode importar       |
|--------------|----------------------------------------------------------------------------------|---------------------|
| `domain/`    | Entidades, contratos (commands/queries/events/tasks), **interfaces** de repositório, serviços. Zero driver. | só `domain`         |
| `infra/`     | Implementações Mongo, modelos BSON, migrations, Asynq, Redis, realtime, clientes externos, storage, secrets, e-mail. | `domain`            |
| `presenter/` | DTOs (`contracts/<domínio>`), controllers, middlewares HTTP/WS.                  | `domain`            |
| `app/`       | Config, container de DI, factories, rotas, `start_routines`, server.            | todas               |

**Repositório em dois lugares:** a *interface* vive em
`domain/<domínio>/repository/`; a *implementação* Mongo em
`infra/database/mongodb/repositories/<domínio>/`; o *modelo BSON* em
`infra/database/mongodb/models/<domínio>_models.go`.

Detalhes da árvore em [folder-structure.md](folder-structure.md).

---

## 3. Papéis (`RUN_ROLE`)

Um único binário; `RUN_ROLE` seleciona os *routines* que sobem. Permite escalar
cada papel de forma independente no Compose/Kubernetes.

| Papel       | Sobe                                                                 | Escala por          |
|-------------|---------------------------------------------------------------------|---------------------|
| `api`       | Servidor HTTP (REST) + middlewares + bootstrap (índices/seed).      | throughput de REST  |
| `ws`        | Servidor WebSocket + loop de Pub/Sub Redis (fan-out entre nós).     | nº de conexões      |
| `worker`    | Servidor Asynq + handlers de jobs.                                  | volume de jobs      |
| `scheduler` | Scheduler Asynq (jobs periódicos / cron).                           | singleton lógico    |
| `all`       | Todos acima (desenvolvimento / instâncias pequenas).               | —                   |

- `api` e `ws` **compartilham um único listener HTTP** quando rodam juntos (WS é
  um upgrade de HTTP). Em produção podem ser processos separados.
- `scheduler` deve rodar como **uma instância** (ou com lock) para não duplicar
  o enfileiramento periódico; os jobs em si são idempotentes como segunda linha
  de defesa.
- Boot de schema (migrations/índices + seed) roda nos papéis que “possuem” dados
  (`api`/`all`); é idempotente.

Fluxo de boot: `main.go → app.Run → start_routines.Start` lê `RUN_ROLE` e sobe
os routines do papel.

---

## 4. Dependências de infraestrutura

| Recurso     | Uso                                                                                   |
|-------------|---------------------------------------------------------------------------------------|
| **MongoDB** | Persistência de todos os domínios (collections por domínio).                           |
| **Redis**   | Cache, **presença**, locks distribuídos, rate limit, idempotência, **Pub/Sub** realtime, backend do **Asynq**. |
| **Asynq**   | Filas e jobs assíncronos + scheduler (cron).                                           |
| **WebSocket** | Canal tempo real com o front (eventos de conversa, presença, notificações).         |

---

## 5. Fluxos transversais

- **Mensagem inbound (canal → operador):** `channels` recebe → normaliza →
  `conversations` cria/atualiza → `routing` define fila/agente → evento WS para
  os assinantes → jobs (`sla.check`, `automation`, `copilot`) conforme regras.
- **Mensagem outbound (operador → canal):** `conversations` registra → job
  `channel.deliver` (fila `channels`) → adapter entrega → atualização de status
  → evento WS.
- **Eventos de domínio → tempo real:** serviços publicam eventos; o
  `realtime.Manager` faz fan-out por tópico (tenant-scoped) via Redis Pub/Sub.
- **Efeitos colaterais:** webhooks, notificações, e-mail, IA, snapshots de
  relatório — sempre via Asynq, com retry/backoff e dead-letter do próprio Asynq.

Catálogo de eventos em [realtime-events.md](realtime-events.md); filas e jobs em
[asynq-jobs.md](asynq-jobs.md).

---

## 6. Padrões transversais

| Padrão                        | Onde                                                                 |
|-------------------------------|----------------------------------------------------------------------|
| Paginação por cursor (keyset) | `{ data, page: { next_cursor, has_more } }`                          |
| Envelope de erro              | `{ error: { code, message, details, request_id } }`                  |
| Idempotency-Key (POST)        | chave + hash do payload + resposta no Redis com TTL                  |
| request_id + duração          | middleware em todo request; base para OpenTelemetry                  |
| Rate limit                    | por tenant + ator/IP, contador no Redis                              |
| Tenant scope                  | `tenant_id` no context; `RequireTenant` nos serviços/repositórios    |
| Migrations/seeds idempotentes | índices numerados; seed de tenant/owner/papéis                      |

Códigos de erro: `validation_error`, `unauthorized`, `forbidden`, `not_found`,
`conflict`, `rate_limited`, `integration_unavailable`, `internal_error`.

---

## 7. Documentos relacionados

- [folder-structure.md](folder-structure.md) — árvore de pastas em camadas.
- [backend-modules.md](backend-modules.md) — responsabilidade de cada domínio.
- [data-model.md](data-model.md) — collections MongoDB e chaves Redis.
- [api-design.md](api-design.md) — convenções e endpoints REST.
- [realtime-events.md](realtime-events.md) — protocolo e eventos WebSocket.
- [asynq-jobs.md](asynq-jobs.md) — filas, jobs e agendamentos.
- [security-permissions.md](security-permissions.md) — auth, RBAC, isolamento.
- [mvp-roadmap.md](mvp-roadmap.md) — fases de implementação.
