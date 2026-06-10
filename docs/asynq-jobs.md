# Filas e jobs Asynq

Trabalho assíncrono via [Asynq](https://github.com/hibiken/asynq) sobre Redis.
Processado pelo papel **`worker`**; agendamento periódico pelo papel
**`scheduler`**. Produtores (papéis `api`/`ws`/`worker`) enfileiram via o client
Asynq do container.

## Filas e prioridades

Prioridade ponderada (Asynq escolhe proporcionalmente ao peso):

| Fila | Peso | Uso |
|---|---|---|
| `critical` | 6 | mensagens, handoff, eventos importantes |
| `default` | 3 | trabalho geral |
| `channels` | 3 | entrega outbound de canais |
| `webhooks` | 2 | entrega de webhooks |
| `ai` | 2 | copiloto/IA |
| `reports` | 1 | relatórios/manutenção |

Configurável por env (`ASYNQ_QUEUE_*`, `ASYNQ_CONCURRENCY`).

## Padrão de implementação

- **Payload tipado:** `domain/<x>/contracts/tasks.go` define a struct do payload
  (JSON).
- **Handler:** registrado em `app/start_routines/bootstrap_workers.go`
  (`registerHandlers`), delega para `domain/<x>/service`.
- **Nomes de task:** centralizados em `infra/asynq/queues.go`.
- **Middleware:** logging (tipo, duração, resultado) em todo handler.
- **Retry/backoff:** do próprio Asynq (exponencial). Falha terminal vai para a
  **dead-letter** (archived) do Asynq.
- **Idempotência:** todo handler é idempotente (chave natural / `task_id` /
  verificação de estado) — re-execução não duplica efeito.
- **Tenant:** todo payload carrega `tenant_id`; jobs do scheduler **iteram os
  tenants** (multi-tenant) ou enfileiram um job por tenant.

## Jobs (event-driven)

| Task | Fila | Produzido por | Faz |
|---|---|---|---|
| `channel.deliver` | `channels` | conversations | entrega mensagem outbound via adapter |
| `channel.retry` | `channels` | channels | reentrega após falha transitória |
| `webhook.deliver` | `webhooks` | webhooks | POST assinado (HMAC) ao subscriber |
| `webhook.retry` | `webhooks` | webhooks | retry de entrega |
| `notification.send` | `default` | notifications | cria notificação in-app (+ WS) |
| `notification.email` | `default` | notifications | envia e-mail (SMTP/template) |
| `ai.suggest` | `ai` | copilot | sugestão de resposta |
| `ai.summarize` | `ai` | copilot | resumo da conversa |
| `ai.classify` | `ai` | copilot | intenção/sentimento |
| `csat.send` | `default` | csat | dispara pesquisa ao contato |
| `csat.expire` | `default` | csat | expira pesquisa não respondida |
| `attachment.process` | `default` | attachments | validação/AV/thumbnail |
| `automation.invoke` | `critical` | automation | chama o flow externo |
| `automation.callback` | `critical` | automation | processa callback do flow |
| `routing.assign` | `critical` | routing | resolve fila/agente e atribui |

> `ai.*` são assíncronos quando a latência permite; sugestão interativa curta
> pode ser síncrona (chamada direta ao provider) — decisão por caso.

## Jobs periódicos (scheduler)

Cron multi-tenant e idempotente. Cada job, ao rodar, **fan-out** sobre os
tenants (ou enfileira sub-jobs por tenant).

| Task | Frequência (sugerida) | Fila | Faz |
|---|---|---|---|
| `sla.check` | a cada 1 min | `critical` | varre relógios; emite `sla.warning`/`sla.breached` |
| `chat.close_inactive_conversations` | a cada 5 min | `default` | fecha conversas inativas (regra por tenant) |
| `channels.health_check` | a cada 2 min | `default` | checa saúde dos canais; marca status |
| `reports.snapshot` | de hora em hora | `reports` | snapshot de métricas/KPIs |
| `audit.compact` | diário (03:30) | `reports` | compacta/retém trilha de auditoria |
| `privacy.retention` | diário | `reports` | aplica retenção/expurgo (LGPD) |
| `csat.expire` (sweep) | a cada 15 min | `default` | expira pesquisas vencidas em lote |

> Frequências são *defaults* pragmáticos; ajustáveis por config/tenant.

## Privacy / manutenção (sob demanda)

| Task | Fila | Faz |
|---|---|---|
| `privacy.export` | `reports` | gera export de dados do titular |
| `privacy.erase` | `reports` | anonimiza/expurga dados do titular |

## Confiabilidade

- **Retry:** `MaxRetry` por task (ex.: entrega de canal/webhook alto; IA baixo).
- **Backoff:** exponencial padrão do Asynq.
- **Dead-letter:** falhas terminais ficam *archived* para inspeção/reprocesso.
- **Timeout/Deadline:** por task, para não travar worker.
- **Unicidade:** tasks que não podem duplicar usam `asynq.Unique(ttl)` ou chave
  idempotente no payload.
- **Observabilidade:** Asynqmon (UI) opcional; middleware de logging + métricas.
- **Scheduler singleton:** uma instância (ou lock) enfileira; handlers
  idempotentes cobrem corrida.

## Pontos de extensão

- Registrar handler: `bootstrap_workers.go → registerHandlers`.
- Registrar periódico: `bootstrap_scheduler.go → scheduledJobs`.
- Definir payload: `domain/<x>/contracts/tasks.go`.
- Nome da task: `infra/asynq/queues.go`.
