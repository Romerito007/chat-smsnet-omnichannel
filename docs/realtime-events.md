# Eventos em tempo real (WebSocket)

Tempo real via **WebSocket** com fan-out entre nós por **Redis Pub/Sub**.
Servido pelo papel **`ws`** (compartilha listener com `api` quando `RUN_ROLE`
inclui ambos). Infra em `infra/realtime` (`Hub` + `PubSub` + `Manager`);
handshake em `presenter/websocket/handler.go`.

## Handshake / conexão

```
GET /realtime/ws            (Upgrade: websocket)   # canônico
GET /ws                     (Upgrade: websocket)   # alias equivalente
Authorization: Bearer <jwt>          # access token JWT
# — ou, para browsers que não setam headers no handshake —
GET /realtime/ws?token=<jwt>
```

O handler é exposto nos **dois** caminhos (`/realtime/ws` e `/ws`) no mesmo
listener do API (porta 8080), então o upgrade nunca cai no `404` do roteador
REST. O browser deve usar `?token=<jwt>` (não dá para setar `Authorization` no
handshake do WebSocket).

- **Autenticação** no upgrade: o token é verificado (`VerifyAccess`); sem token
  válido → `401`. O `tenant_id`, o `user_id`, os papéis/permissões e os setores
  saem **do token** (nunca de header). A conexão herda o escopo do tenant.
- **Rooms automáticas** (entrada no connect, não controláveis pelo cliente):
  - `t:{tenant}:tenant` — broadcast do tenant;
  - `t:{tenant}:user:{userId}` — notificações/atribuições/presença pessoal;
  - `t:{tenant}:presence` — quadro de presença da equipe;
  - `t:{tenant}:inbox:{sectorId}` — uma por setor que o ator pode ver.
- **Keepalive (dois níveis):**
  - **Heartbeat de aplicação:** o servidor envia, a cada **20s**, um frame de
    dados no envelope padrão `{ "event": "ping", "ts": <ms>, "data": {} }`. Como
    os clientes (browser) só reiniciam o *silence timer* em frames de dados — e
    **não** em `pong` de protocolo — é este frame que mantém a conexão viva. Não
    exige `subscribe` a nenhuma conversa.
  - **Ping de protocolo (WS control):** além do heartbeat, o servidor envia um
    `PingMessage` a cada ~54s; o cliente responde `pong`, o que renova o
    *read deadline* (`pongWait` 60s). Ambos os intervalos (20s e 54s) são
    menores que `pongWait`, então o servidor **nunca** derruba uma conexão
    ociosa antes do keepalive. Limite de leitura de **4096 bytes/frame**.
- **Back-pressure:** buffer de envio por cliente (64); cliente lento tem entrega
  best-effort (descarte) para não travar o fan-out.
- **Multi-aba:** várias conexões por usuário (limite opcional
  `REALTIME_MAX_CONN_PER_USER`); cada conexão assina o que precisa.

## Protocolo (frames cliente → servidor)

O cliente só pode **assinar/desassinar uma conversa** (as demais rooms são
automáticas). Frame de controle em JSON:

```json
{ "action": "subscribe",   "conversation_id": "conv_123" }
{ "action": "unsubscribe", "conversation_id": "conv_123" }
```

- `subscribe` exige a permissão **`conversation.read`**; o servidor monta a room
  a partir do **tenant da conexão** (`t:{tenant}:conversation:{id}`), então um
  cliente nunca assina fora do seu tenant.
- Comandos sem `conversation_id`, com `action` desconhecida ou não autorizados são
  **ignorados silenciosamente**.

## Tópicos (rooms)

Sempre **tenant-scoped**. Convenção `t:{tenant_id}:{escopo}[:{id}]`
(`domain/shared/topics.go`):

| Tópico | Quem assina | Conteúdo |
|---|---|---|
| `t:{tenant}:tenant` | toda conexão do tenant | broadcasts do tenant |
| `t:{tenant}:user:{userId}` | o próprio agente (auto) | notificações, atribuições, presença pessoal |
| `t:{tenant}:presence` | toda conexão (auto) | mudanças de presença agregadas |
| `t:{tenant}:inbox:{sectorId}` | agentes do setor (auto) | novas conversas, lifecycle, `queue.stats` |
| `t:{tenant}:conversation:{id}` | sob demanda (`subscribe`) | mensagens, status, typing, SLA, aprovações |

## Envelope (servidor → cliente)

Cada frame entregue ao socket é o JSON (`infra/realtime/publisher.go`):

```json
{
  "event": "message.created",   // nome do evento (ver catálogo)
  "ts": 1718000000000,          // epoch ms da publicação
  "data": { /* payload do evento */ }
}
```

> O frame **não** carrega `topic` nem um `id` de evento: o cliente sabe a room
> pela assinatura e deduplica pelo id de domínio dentro de `data` (ex.:
> `message_id`, `conversation_id`).

## Catálogo de eventos

> Esta tabela reflete **exatamente** os nomes emitidos pelo backend (constantes
> em `domain/<x>/contracts`). Os eventos de **ciclo de vida** da conversa são
> publicados com nome próprio **e** acompanhados de `conversation.updated` (para
> clientes que só assinam o evento genérico). Os eventos de mensagem são
> **granulares** (`message.sent/delivered/read/failed`), não um `message.status`.

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

`ConversationPayload` = `{ id, tenant_id, contact_id, channel, channel_id, sector_id, queue_id,
status, assigned_to, priority, tags, last_message_at, unread_count, last_read_at,
updated_at }`. `channel_id` é o id da `ChannelConnection` específica à qual a
conversa pertence (vazio só em conversas criadas sem uma).

**Não-lido por conversa:** `unread_count` é incrementado a cada mensagem
**inbound** (cliente) e **zerado** em `POST /v1/conversations/{id}/read` (que
também grava `last_read_at`). Ambas as transições publicam `conversation.updated`,
então o badge da inbox reflete em tempo real. (Alternativa equivalente para o
cliente: comparar `last_read_at` com `last_message_at`.)

### Timeline de ciclo de vida/automação — **não** são mensagens
Os eventos estruturados de ciclo de vida (`conversation.assigned/transferred/
enqueued/...`) e de automação (`automation.decision`, `automation.escalated`) são
persistidos numa coleção **separada** (`conversation_events`, entidade
`ConversationEvent`) — **não** como mensagens de sistema — e portanto **não**
aparecem em `GET /v1/conversations/{id}/messages`. Eles são lidos por
**`GET /v1/conversations/{id}/events`** (paginado por cursor). As *mensagens de
sistema* visíveis no fio (ex.: avisos enviados via `SendSystemMessage`) são
mensagens reais (`sender_type=system`, `message_type=system`) e **aparecem** em
`GET messages`. Decisão registrada também em `docs/api-design.md`.

### Mensagens (`conversations`)
Publicados no tópico `conversation:{id}` (`message.created` também em `inbox`).

| Evento | Quando | Payload |
|---|---|---|
| `message.created` | nova mensagem/nota | `MessagePayload` |
| `message.updated` | edição (soft) do texto (`edited_at` setado) | `MessagePayload` |
| `message.deleted` | exclusão (soft) — some das listagens | `MessageRefPayload` |
| `message.sent` | outbound entregue ao canal | `MessageStatusPayload` |
| `message.delivered` | receipt do canal: entregue | `MessageStatusPayload` |
| `message.read` | receipt do canal: lida / leitura do agente | `MessageStatusPayload` / `ReadPayload` |
| `message.failed` | falha de entrega (após retries) | `MessageStatusPayload` |
| `typing.started` / `typing.stopped` | digitação | `TypingPayload` |

`MessagePayload` = a `Message` REST (ver `openapi.yaml`): `{ id, conversation_id,
sender_type, sender_id, direction, message_type, text, attachments, metadata,
delivery_status, external_message_id, created_at, edited_at }`.
`MessageRefPayload` = `{ message_id, conversation_id }` (sem corpo).
`MessageStatusPayload` = `{ message_id, conversation_id, delivery_status, error? }`.
`TypingPayload` = `{ conversation_id, user_id }`.

### Presença (`presence`)
| Evento | Tópico | Payload |
|---|---|---|
| `agent.presence_changed` | `presence`, `user` | `{ tenant_id, user_id, status, current_load, max_concurrent_chats, last_seen_at }` |

### Filas (`queues`)
| Evento | Tópico | Payload |
|---|---|---|
| `queue.stats` | inbox/setor | `{ tenant_id, sector_id, queue_id, waiting_count, assigned_count }` |

Emitido quando a composição da fila muda — entrada (`enqueue`/criação em fila),
saída (fechamento) ou atribuição (`assign`/`transfer`).

### SLA (`sla`)
| Evento | Tópico | Payload |
|---|---|---|
| `sla.warning` | `conversation` | `{ conversation_id, policy_id, target, due_at }` |
| `sla.breached` | `conversation` (+ webhook `sla.breached`) | `{ conversation_id, policy_id, target, breached_at }` |

`target` ∈ `first_response` | `resolution`.

### Copilot (`copilot`)
| Evento | Tópico | Payload |
|---|---|---|
| `copilot.suggestion_completed` | `conversation`, `user` | `CopilotResult` |

`CopilotResult` = `{ action, provider, model, text, categories, tokens_input,
tokens_output, estimated_cost, requires_approval, proposed_actions }` (ver
`openapi.yaml`). Quando a IA propõe uma ação de **escrita**, `proposed_actions`
traz os cards e segue um `mcp.approval_requested`.

### Ferramentas externas / MCP (`mcp`)
| Evento | Tópico | Payload |
|---|---|---|
| `mcp.approval_requested` | `conversation` | `{ approval_id, conversation_id, server, tool, args, proposed_by }` |

Emitido quando uma ferramenta **write** é proposta (pela IA ou por um run manual)
e aguarda aprovação explícita do atendente. A execução só ocorre via
`POST /v1/conversations/{id}/copilot/approvals/{approvalId}`.

### Notificações (`notifications`)
| Evento | Tópico | Payload |
|---|---|---|
| `notification.created` | `user` | `{ id, type, title, body, link, read, created_at }` |

`type` inclui, entre outros: `conversation.assigned_to_you`,
`conversation.transferred_to_you`, `mention.internal_note`,
`channel.connection_error`.

### Keepalive (sistema)
| Evento | Tópico | Payload |
|---|---|---|
| `ping` | — (por conexão) | `{}` |

Heartbeat de aplicação enviado pelo próprio pump da conexão
(`presenter/websocket/handler.go`) a cada **20s**, fora do fluxo
`EventPublisher`. Não exige `subscribe`. O cliente deve apenas tratá-lo como
sinal de vida (reiniciar seu *silence timer*); não há ação associada.

## Como os eventos são publicados

1. Um **serviço de domínio** conclui uma mudança de estado.
2. Chama `EventPublisher.Publish(ctx, topic, event, data)` (o `Manager`).
3. O `Manager` serializa o envelope e publica no canal Redis `realtime:fanout`.
4. **Todos** os nós `ws` recebem via Pub/Sub e entregam aos clientes locais
   assinantes daquele tópico (`Hub`).

Isso desacopla o nó que originou a mudança (pode ser `api` ou `worker`) do nó que
mantém a conexão WS do cliente.

## Garantias e padrões

- **At-most-once** na entrega WS (tempo real, não é fonte de verdade). O estado
  canônico está no Mongo; o cliente reconcilia via REST ao (re)conectar.
- **Reconexão:** ao reconectar, o cliente re-assina as conversas abertas e busca
  o delta via REST (último `cursor`/`last_message_at`).
- **Dedupe:** sem id de envelope; o cliente ignora duplicados pelo id de domínio
  no `data` (ex.: `message_id`).
- **Autorização:** `subscribe` é checado contra `conversation.read`; rooms padrão
  refletem os setores do token.
