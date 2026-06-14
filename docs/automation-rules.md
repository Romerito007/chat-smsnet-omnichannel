# Regras de Automação (estilo Chatwoot)

Motor de **regras** = **gatilho (evento) + condições (AND) + ações**. Quando um
evento de ciclo de vida ocorre, a regra avalia as condições contra a
conversa/contato; se casarem, executa as ações. Domínio `automationrules`
(coleção `automation_rules`). Permissão: `automation.manage`. Rotas:
`/v1/automation-rules` (CRUD) + `GET /v1/automation-rules/{id}/logs`.

## Avaliação assíncrona
A emissão do evento **não bloqueia** o caminho quente: o conversation Service
chama um `RuleEventSink` que apenas **enfileira** uma task Asynq
`automationrule.evaluate` (fila `default`). O worker carrega as regras habilitadas
do tenant para aquele evento, **hidrata** a conversa (+ contato), casa as condições
e dispara as ações.

## Eventos (enum) → evento interno emitido
Só eventos que o backend já emite (em `conversation_service.go`):

| `event` da regra | interno | emitido em |
|---|---|---|
| `conversation_created` | `conversation.created` | `Create()` (via `publishLifecycle`) |
| `conversation_updated` | `conversation.updated` | `Update()` (PATCH) |
| `conversation_resolved` | `conversation.resolved` | `Update(status=resolved)` |
| `conversation_opened` | `conversation.reopened` | `Reopen()` |
| `conversation_closed` | `conversation.closed` | `Close()`/`Update(status=closed)` |
| `message_created` | `message.created` | criação de mensagem real (não nota interna) |

> **`message_created`**: o payload do evento é a **mensagem** (só + `conversation_id`).
> As condições são sobre a **conversa/contato**, então o worker **hidrata a conversa
> da mensagem** (e o contato) e casa contra ela — nunca contra a mensagem. O `data`
> entregue ao webhook é o payload original (a mensagem).

## Condições (combinadas por AND)
Todas resolvem contra a conversa/contato hidratados (válidas em **todos** os
eventos). `conditions` vazio = **match-all** (dispara em toda ocorrência).

| Campo (`field`) | Fonte | Operadores |
|---|---|---|
| `status` | conversa | `equal_to`, `not_equal_to` |
| `channel` | conversa | `equal_to`, `not_equal_to` |
| `assigned_agent_id` | conversa (assigned_to) | `equal_to`, `not_equal_to` |
| `sector_id` (Time) | conversa | `equal_to`, `not_equal_to` |
| `queue_id` | conversa | `equal_to`, `not_equal_to` |
| `priority` | conversa | `equal_to`, `not_equal_to` |
| `tags` (`[]string`) | conversa | `contains`, `does_not_contain` (value = 1 tag id) |
| `contact_phone` | contato | `equal_to`, `contains` |

Condições do Chatwoot sem dado no nosso schema (idioma, e-mail, país, etc.) são
**ignoradas** (não existem no enum).

## Ação: `send_webhook`
Única ação implementada (campo `type` extensível). Referencia um **webhook já
cadastrado** por `webhook_id` (validado: existe no tenant). Ao disparar, reusa o
**pipeline de webhooks** via `Dispatcher.EmitTo` — que **ignora o `events[]` e os
`scopes` (setores)** da subscription, porque **quem decide o disparo é a regra**
(suas condições), não o webhook. Ganha de graça **HMAC, retry/backoff,
dead-letter e rate-limit**. O envelope é o padrão `{id,event,created_at,data}`.

## Anti-loop (premissa documentada)
Proteção: **dedup `(rule_id, conversation_id, event)`** numa janela de **10s**
(Redis `SETNX`). Cobre o re-disparo imediato da mesma regra (rajadas, callbacks
repetidos).

**Premissa:** como a **única ação é `send_webhook`** — que **sai do sistema** e
**não muda estado interno** (não atribui, não taggeia, não altera status) — ela
**não emite nenhum evento interno** e portanto **não realimenta** o motor. Não há
laço interno entre regras. O único encadeamento possível é **externo-mediado** (um
sistema lá fora reage ao webhook e chama nossa API), e cada salto é um evento
legítimo, limitado pelo sistema externo. Logo o dedup de 10s **é suficiente**.
⚠️ Esta premissa **vale enquanto a única ação não mutar estado interno**. Se uma
ação interna (atribuir/taggear/resolver) for adicionada no futuro, o anti-loop
precisa ser reavaliado (ex.: profundidade/origem do disparo).

## Integridade referencial
Deletar um webhook referenciado por uma regra é **bloqueado** com **409**
("webhook em uso pela regra X") — nunca anula a regra silenciosamente. Espelha o
bloqueio perfil-de-ISP↔assistente.

## Log de execução
Coleção `rule_evaluation_logs` (metadata only, sem payload): `rule_id`, `event`,
`conversation_id`, `status` (`action_enqueued` | `skipped_dedup` | `error`),
`error_summary`, `created_at`. Lido via `GET /v1/automation-rules/{id}/logs`
(paginado por keyset).
