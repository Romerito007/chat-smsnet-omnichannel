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
| `t:{tenant}:inbox:{sectorId}` | agentes do setor | novas conversas, mudanças de fila |
| `t:{tenant}:queue:{queueId}` | supervisores | stats de fila |
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

### Conversas e mensagens (`conversations`)
| Evento | Tópico | Payload (resumo) |
|---|---|---|
| `conversation.created` | inbox/setor | conversa (resumo) |
| `conversation.updated` | conversation, inbox | status, priority, tags, subject |
| `conversation.assigned` | conversation, user | `{ assignee_id, by }` |
| `conversation.transferred` | conversation, inbox | `{ from, to, by }` |
| `conversation.resolved` | conversation, inbox | `{ reason_id, by }` |
| `conversation.closed` | conversation, inbox | `{ by, closed_at }` |
| `conversation.reopened` | conversation, inbox | |
| `message.created` | conversation, inbox | mensagem completa |
| `message.status` | conversation | `{ message_id, status: sent|delivered|read|failed }` |
| `message.read` | conversation | `{ message_id, by }` |
| `typing` | conversation | `{ user_id|contact, on: bool }` |

### Presença (`presence`)
| Evento | Tópico | Payload |
|---|---|---|
| `presence.changed` | presence, user | `{ user_id, status, capacity }` |

### Filas (`queues`)
| Evento | Tópico | Payload |
|---|---|---|
| `queue.stats` | queue | `{ queue_id, waiting, serving, agents_online }` |

### SLA (`sla`)
| Evento | Tópico | Payload |
|---|---|---|
| `sla.warning` | conversation, user | `{ conversation_id, target, due_at }` |
| `sla.breached` | conversation, inbox, user | `{ conversation_id, target }` |

### Copilot (`copilot`)
| Evento | Tópico | Payload |
|---|---|---|
| `copilot.suggestion` | conversation, user | `{ run_id, kind, output }` |

### Notificações (`notifications`)
| Evento | Tópico | Payload |
|---|---|---|
| `notification.created` | user | notificação |

### Automation (`automation`)
| Evento | Tópico | Payload |
|---|---|---|
| `automation.updated` | conversation | `{ execution_id, status }` |

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
