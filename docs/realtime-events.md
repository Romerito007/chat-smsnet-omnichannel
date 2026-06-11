# Eventos em tempo real (WebSocket)

Tempo real via **WebSocket** com fan-out entre nós por **Redis Pub/Sub**.
Servido pelo papel **`ws`** (compartilha listener com `api` quando `RUN_ROLE`
inclui ambos). Infra em `infra/realtime` (`Hub` + `PubSub` + `Manager`).

## Conexão

```
GET /realtime/ws        (Upgrade: websocket)
Authorization: Bearer <jwt>     # ou token de query assinado para browsers
```

- A conexão é **autenticada** no upgrade; o `tenant_id` e o `user_id` saem do
  token. Conexões herdam o escopo do tenant.
- Keepalive: ping/pong (servidor envia ping ~a cada 54s; cliente responde).
- Limite de leitura por frame; back-pressure: cliente lento tem entrega
  best-effort (buffer com descarte) para não travar o fan-out.

## Protocolo (frames cliente → servidor)

Frames de controle em JSON:

```json
{ "action": "subscribe",   "topic": "t:{tenant}:conversation:{id}" }
{ "action": "unsubscribe", "topic": "t:{tenant}:inbox:{sector}" }
{ "action": "typing",      "topic": "t:{tenant}:conversation:{id}", "data": { "on": true } }
{ "action": "presence",    "data": { "status": "online" } }     # heartbeat
```

- O servidor **valida** que o `topic` pertence ao tenant da conexão e que o ator
  tem permissão de ver aquele recurso (ex.: agente só assina conversas que pode
  ver). Tópicos não autorizados são ignorados.

## Tópicos (rooms)

Sempre **tenant-scoped**. Convenção `t:{tenant_id}:{escopo}[:{id}]`:

| Tópico | Quem assina | Conteúdo |
|---|---|---|
| `t:{tenant}:user:{userId}` | o próprio agente | notificações, atribuições, presença pessoal |
| `t:{tenant}:conversation:{id}` | participantes/observadores | mensagens, status, typing, SLA |
| `t:{tenant}:inbox:{sectorId}` | agentes do setor | novas conversas, lifecycle, `queue.stats` |
| `t:{tenant}:queue:{queueId}` | supervisores | reservado (futuro) |
| `t:{tenant}:presence` | quem precisa | mudanças de presença agregadas |

## Envelope (servidor → cliente)

```json
{
  "topic": "t:acme:conversation:123",
  "event": "message.created",
  "id": "evt_...",          // id do evento (dedupe no cliente)
  "ts": 1718000000000,      // epoch ms
  "data": { /* payload do evento */ }
}
```

## Catálogo de eventos

> Esta tabela reflete **exatamente** os nomes emitidos pelo backend (constantes em
> `domain/<x>/contracts`). Os eventos de **ciclo de vida** da conversa são
> publicados com nome próprio **e** acompanhados de `conversation.updated` (para
> clientes que só assinam o evento genérico de mudança). Os eventos de mensagem
> são **granulares** (`message.sent/delivered/read/failed`), não um `message.status`
> genérico.

### Conversas (`conversations`) — lifecycle
Publicados nos tópicos `conversation:{id}` **e** `inbox:{sectorId}`.

| Evento | Quando | Payload |
|---|---|---|
| `conversation.created` | criação (`Create`) | `ConversationPayload` |
| `conversation.updated` | qualquer mudança (acompanha todos os demais) | `ConversationPayload` |
| `conversation.closed` | `Close`/`CloseInactive` (status `closed`/`archived`) | `ConversationPayload` |
| `conversation.resolved` | `Update` com `status=resolved` | `ConversationPayload` |
| `conversation.reopened` | `Reopen` | `ConversationPayload` |
| `conversation.assigned` | atribuição (routing) | `ConversationPayload` |
| `conversation.transferred` | transferência (routing) | `ConversationPayload` |
| `conversation.tagged` | aplicação de tags | `{ conversation_id, tags }` |

`ConversationPayload` = `{ id, tenant_id, contact_id, channel, sector_id, queue_id,
status, assigned_to, priority, tags, last_message_at, updated_at }`.

### Mensagens (`conversations`)
Publicados no tópico `conversation:{id}` (`message.created` também em `inbox`).

| Evento | Quando | Payload |
|---|---|---|
| `message.created` | nova mensagem/nota | `MessagePayload` |
| `message.sent` | outbound entregue ao canal | `MessageStatusPayload` |
| `message.delivered` | receipt do canal: entregue | `MessageStatusPayload` |
| `message.read` | receipt do canal: lida / leitura do agente | `MessageStatusPayload` / `ReadPayload` |
| `message.failed` | falha de entrega (após retries) | `MessageStatusPayload` |
| `typing.started` / `typing.stopped` | digitação | `TypingPayload` |

`MessageStatusPayload` = `{ message_id, conversation_id, delivery_status, error? }`.

### Presença (`presence`)
| Evento | Tópico | Payload |
|---|---|---|
| `presence.changed` | presence, user | `{ user_id, status, ... }` |

### Filas (`queues`)
| Evento | Tópico | Payload |
|---|---|---|
| `queue.stats` | inbox/setor | `{ tenant_id, sector_id, queue_id, waiting_count, assigned_count }` |

Emitido quando a composição da fila muda — entrada (`enqueue`/criação em fila),
saída (fechamento) ou atribuição (`assign`/`transfer`).

### SLA (`sla`)
| Evento | Tópico | Payload |
|---|---|---|
| `sla.warning` | conversation | `{ conversation_id, target, ... }` |
| `sla.breached` | conversation (+ webhook `sla.breached`) | `{ conversation_id, target, ... }` |

### Copilot (`copilot`)
| Evento | Tópico | Payload |
|---|---|---|
| `copilot.suggestion_completed` | conversation, user | `{ run_id, action, output, ... }` |

### Notificações (`notifications`)
| Evento | Tópico | Payload |
|---|---|---|
| `notification.created` | user | notificação |

## Como os eventos são publicados

1. Um **serviço de domínio** conclui uma mudança de estado.
2. Chama `realtime.Manager.Publish(topic, payload)`.
3. O `Manager` publica no canal Redis `realtime:fanout`.
4. **Todos** os nós `ws` recebem via Pub/Sub e entregam aos clientes locais
   assinantes daquele tópico (`Hub.Deliver`).

Isso desacopla o nó que originou a mudança (pode ser `api` ou `worker`) do nó
que mantém a conexão WS do cliente.

## Garantias e padrões

- **At-most-once** na entrega WS (tempo real, não fonte de verdade). O estado
  canônico está no Mongo; o cliente reconcilia via REST ao (re)conectar.
- **Reconexão:** ao reconectar, o cliente re-assina tópicos e busca o delta via
  REST (último `cursor`/`last_message_at`).
- **Dedupe:** `event.id` permite ignorar duplicados.
- **Autorização contínua:** revogação de acesso encerra/limpa assinaturas.
- **Multi-aba:** várias conexões por usuário; cada uma assina o que precisa.
