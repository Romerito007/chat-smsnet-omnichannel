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
| `message_content` | **mensagem do evento** | `contains`, `does_not_contain` (substring, case-insensitive) |

`message_content` é o **único** campo resolvido contra a **mensagem** (texto do
`message_created`), não a conversa — para regras do tipo "se o cliente escrever
'suporte'". Condições do Chatwoot sem dado no nosso schema (idioma, e-mail, país,
custom attributes) são **ignoradas** (não existem no enum).

## Catálogo de ações
Executadas na **ordem do array**. Cada ação lê só o seu param.

| `type` | param | efeito |
|---|---|---|
| `send_webhook` | `webhook_id` | entrega o evento ao webhook via `Dispatcher.EmitTo` (ignora `events[]`/`scopes` — quem decide é a regra). HMAC, retry, dead-letter, rate-limit de graça. |
| `send_message` | `text` | injeta mensagem **outgoing** `SenderType=automation` ("System Automation"), reusando o pipeline normal (`message_created` → webhooks → integrador entrega). |
| `send_attachment` | `attachment_id` | idem, com anexo (ready, mesmo tenant). |
| `assign_agent` | `agent_id` | atribui a conversa ao agente. |
| `assign_team` | `sector_id` | põe a conversa no setor (Time = setor). |
| `remove_assigned_agent` | — | limpa o agente. |
| `remove_assigned_team` | — | limpa o setor. |
| `add_tag` / `remove_tag` | `tag_id` | adiciona/remove tag. |
| `change_priority` | `priority` | `low\|normal\|high\|urgent`. |
| `resolve_conversation` | — | status → `resolved`. |
| `open_conversation` | — | reabre uma conversa fechada. |
| `mark_pending` | — | status → `queued` (volta para a fila, "pending" do Chatwoot — não há status `pending` próprio). |

As ações de estado e de mensagem rodam **sob `origin=automation`** (ver anti-loop).
`priority` (campo da regra, int) ordena regras no mesmo evento (asc; desempate
`created_at`/`id`).

## Anti-loop (3 camadas)
Ações agora **mutam estado** e **criam mensagem**, então a premissa antiga (única
ação saía do sistema) não vale mais. Proteção em camadas:

1. **Origem (principal).** Todo evento de ciclo de vida produzido por uma ação de
   automação é marcado `origin=automation` (carregado no `context`; para mensagens,
   **derivado de `SenderType=automation`** no `persistMessage`, então vale mesmo se
   um caminho de envio futuro esquecer de carimbar o contexto). O evaluator
   **descarta** eventos `origin=automation` no topo → automação **não realimenta**
   automação. Mata o laço interno na raiz (ex.: regra em `message_created` com
   `send_message` não redispara a si mesma).
2. **Fusível por conversa (rede de segurança).** Máx. **100 mensagens de automação
   por conversa / 10 min** (Redis `INCR`+TTL). Não é controle de fluxo (funil
   legítimo nunca chega lá) — é disjuntor para integrador bugado que ecoa infinito.
   Estourou → ações de mensagem/anexo são **suprimidas** e logadas `skipped_budget`
   (a regra **não** é desabilitada).
3. **Teto de profundidade (defesa de borda).** A task carrega `depth`; ação emite
   `depth+1`; acima de `maxDepth=3` o evaluator descarta. Barato; cobre um caminho
   imprevisto que não tenha carimbado origem.

**Dedup por `event_id`** (substitui a janela de 10s): a chave `(rule, event_id)` é
**reivindicada ANTES** de executar as ações, então um **retry do Asynq** encontra a
chave tomada e pula — sem reenviar `send_message` duplicado.

## Ordem, concorrência, staleness e falha
- Regras no mesmo evento rodam por **`priority` asc** (desempate `created_at`/`id`);
  ações na **ordem do array**.
- **Lock por conversa** (Redis) serializa a execução de ações na mesma conversa.
- **Re-hidratação sob o lock**: uma regra que casou na emissão mas **não casa mais**
  com a conversa viva é pulada (`skipped_stale`).
- Falha de ação é **best-effort**: não aborta as outras, não faz a task reprocessar
  (evita reenvio), e cada ação loga seu próprio resultado.

## Integridade referencial
- **Webhook**: deletar um referenciado é **bloqueado com 409** (como hoje).
- **Agente / tag / setor / anexo**: integridade **soft** — o delete **não** é
  bloqueado. Em runtime a ação falha graciosamente e loga `skipped_missing_ref`; a
  regra exibe um **indicador de saúde** (`health.missing_refs`) na listagem/GET. Na
  **criação/edição** da regra, cada ação valida seus params (existe agente/tag/
  setor; `attachment` ready; `priority` válido; `webhook` existe) → 422 por campo.

## Log de execução
Coleção `rule_evaluation_logs` (metadata only): `rule_id`, `event`,
`conversation_id`, `action_type`, `status` (`action_enqueued` | `skipped_dedup` |
`skipped_automation` | `skipped_stale` | `skipped_budget` | `skipped_missing_ref` |
`error`), `error_summary`, `created_at` — **uma linha por ação**. Lido via
`GET /v1/automation-rules/{id}/logs` (keyset).
