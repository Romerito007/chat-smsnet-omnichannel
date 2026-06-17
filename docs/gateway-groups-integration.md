# Integração do gateway de WhatsApp — Sincronização de grupos (Domínio 1)

Este documento explica **como o gateway integra** com o chat para o cadastro de
grupos de WhatsApp, e traz no final um **prompt pronto** para gerar a implementação
no lado do gateway.

> Contexto: o chat conhece a lista de grupos do canal (~5k, 1 por cliente) **só para
> marcar quais NÃO atender** (filtro `attend`; default = atende). O chat **não** muta
> o WhatsApp (não cria/edita/remove grupo nem participante). Ele é **agnóstico ao
> gateway** — o gateway é quem fala com o WhatsApp e empurra a lista.

---

## Visão geral do fluxo

```
                    1. POST /v1/groups/sync {channel_id}        (supervisor no chat)
                            │
                            ▼
   ┌──────────┐   2. webhook "group_sync_requested"   ┌────────────────────┐
   │   CHAT   │ ───────────────────────────────────►  │   GATEWAY WhatsApp  │
   │ (backend)│      (entregue ao outbound_url           │ (você implementa)  │
   └──────────┘       do canal, assinado HMAC)          └────────────────────┘
        ▲                                                         │
        │   3. POST /v1/inbound/channel/{channel}/groups          │
        │      { inbound_token, groups:[...] }  em LOTES (≤2000)   │
        └─────────────────────────────────────────────────────────┘
              (auth por inbound_token; upsert idempotente)
```

São **dois passos** do lado do gateway:

1. **Receber** o evento `group_sync_requested` (webhook que o chat envia ao
   `outbound_url` do canal).
2. **Responder** buscando os grupos no WhatsApp e fazendo `POST` da lista em lotes
   para o endpoint de inbound de grupos.

Nada disso é síncrono: o `POST /v1/groups/sync` responde `202` na hora; os grupos
chegam depois, nos lotes que o gateway empurra.

---

## Passo 2 — Receber o evento `group_sync_requested`

O chat entrega esse evento **só ao webhook gerenciado do canal** (a subscription que
nasce quando o canal tem `outbound_url`). É o **mesmo transporte** dos outros eventos
de webhook que o gateway já recebe (mensagens, etc.), então **não há rota nova** a
implementar — só tratar mais um `event`.

**HTTP:** `POST {outbound_url do canal}`

**Headers:**

| Header | Valor |
|---|---|
| `Content-Type` | `application/json` |
| `X-Webhook-Event` | `group_sync_requested` |
| `X-Webhook-Timestamp` | epoch em segundos (string) |
| `X-Webhook-Signature` | `sha256=<hex>` — veja validação abaixo |
| `X-Webhook-Delivery-Id` | id único da entrega (idempotência) |

**Body (envelope canônico de webhook):**

```json
{
  "id": "del_...",
  "event": "group_sync_requested",
  "created_at": "2026-06-17T12:00:00Z",
  "data": { "channel_id": "<id do canal>" }
}
```

**Validação da assinatura (recomendada):**
`X-Webhook-Signature` = `sha256=` + HMAC-SHA256, em **hex**, com chave =
`outbound_secret` do canal, sobre a string **`"<X-Webhook-Timestamp>.<corpo cru>"`**:

```
mac = HMAC_SHA256(outbound_secret, timestamp + "." + rawBody)
assinatura_esperada = "sha256=" + hex(mac)
```

Compare em **tempo constante** e rejeite timestamps fora de uma janela (ex.: ±5 min)
para anti-replay. O `outbound_secret` é o segredo que você definiu/rotacionou no
canal (`POST /v1/channels/{id}/rotate-outbound-secret`).

> O `data.channel_id` diz **qual canal** (qual número/instância de WhatsApp) deve ter
> seus grupos sincronizados. Use-o para escolher a instância certa do WhatsApp **e**
> o `inbound_token` correto no passo 3.

---

## Passo 3 — Empurrar os grupos (em lotes)

Depois de listar os grupos no WhatsApp, o gateway faz `POST` da lista para o chat.
Como são ~5k, **divida em lotes de no máximo 2000** (recomendado 500–1000 por
request).

**HTTP:** `POST {base_url_do_chat}/v1/inbound/channel/{channel}/groups`

- `{channel}` = o **type** do canal (ex.: `whatsapp`), não o id.

**Auth (sem JWT — borda de integração):**

| | |
|---|---|
| Preferido | header `X-Inbound-Token: <inbound_token do canal>` |
| Alternativa | campo `inbound_token` no corpo |

O `inbound_token` é o token de integração do canal (revelado uma vez na criação /
rotação do canal). O **tenant e o canal** são resolvidos **só** a partir desse token —
nunca de um header de tenant.

**Body:**

```json
{
  "inbound_token": "<opcional se já mandou no header>",
  "groups": [
    {
      "groupId": "120363000000000000@g.us",
      "subject": "Cliente Acme LTDA",
      "description": "Suporte Acme",
      "participants": ["5544999990000@s.whatsapp.net", "5544999991111@s.whatsapp.net"],
      "group_admins": ["5544999990000@s.whatsapp.net"],
      "owner_jid": "5544999990000@s.whatsapp.net",
      "owner_name": "João",
      "activated": true
    }
  ]
}
```

**Mapa de campos (aceita a shape do gateway):**

| Campo no chat | Aceita do gateway | Observação |
|---|---|---|
| `group_jid` | `groupId` **ou** `group_jid` | **obrigatório** — chave idempotente |
| `name` | `subject` **ou** `name` | |
| `description` | `description` | |
| `participants` | `participants` `[]string` | guardado **cru** (metadado, não vira contato) |
| `group_admins` | `group_admins` **ou** `admins` `[]string` | guardado **cru** |
| `company_id` | `company_id` | opcional |
| `whatsapp_wid` | `whatsapp_wid` | opcional |
| `owner_name` / `owner_jid` | idem | opcional |
| `activated` | `activated` bool | opcional |

**Resposta:** `200 OK` → `{ "ok": true, "upserted": <n> }`.

**Garantias do upsert (importante para o gateway poder re-sincronizar à vontade):**

- **Idempotente** por `(tenant, group_jid)`: reenviar os mesmos 5k **não duplica**.
- **Preserva a escolha do operador:** `attend` nasce `true` só no primeiro insert; um
  re-sync **nunca** reseta um grupo que o operador marcou como "não atender".
- Itens **sem** `group_jid` são ignorados (defensivo). Lote vazio → `400`.
- Lote `> 2000` → `400` (divida).

**Erros comuns:**

| Status | Causa |
|---|---|
| `401` | `inbound_token` inválido/ausente ou canal desabilitado |
| `400` | JSON inválido, lote vazio, ou lote > 2000 |
| `429` | rate limit da borda pública (faça backoff e reenvie o lote) |

---

## O que o chat faz com os grupos (gestão, lado do operador)

Só para contexto — **não** é responsabilidade do gateway:

- `GET /v1/groups?q=` — lista/busca por nome+descrição (perm `group.view`).
- `PATCH /v1/groups/{id} {attend}` — marca/desmarca atender (perm `group.manage`).
- `POST /v1/groups/sync {channel_id}` — dispara o fluxo acima (perm `group.manage`).

O **atendimento** de mensagens de grupo (o *gate* do `attend`) é o **Domínio 2**,
ainda não implementado.

---

## Prompt pronto (cole no agente que implementa o gateway)

> Você vai implementar, no gateway de WhatsApp, a **sincronização de grupos** com o
> chat omnichannel. São dois pontos:
>
> **1) Tratar o webhook `group_sync_requested`.** O chat já entrega webhooks ao
> `outbound_url` do canal. Adicione o tratamento do `event == "group_sync_requested"`.
> Headers relevantes: `X-Webhook-Event`, `X-Webhook-Timestamp`,
> `X-Webhook-Signature` (`sha256=<hex>`), `X-Webhook-Delivery-Id`. Valide a
> assinatura: `HMAC_SHA256(outbound_secret, timestamp + "." + rawBody)` em hex,
> comparação em tempo constante, janela anti-replay de ±5 min; rejeite duplicatas por
> `X-Webhook-Delivery-Id`. O corpo é
> `{ "id", "event", "created_at", "data": { "channel_id": "<id>" } }`. Responda `2xx`
> rápido e processe de forma assíncrona.
>
> **2) Empurrar a lista de grupos.** Ao receber o evento, liste os grupos da
> instância de WhatsApp correspondente ao `data.channel_id` e faça
> `POST {BASE_URL_CHAT}/v1/inbound/channel/whatsapp/groups`, autenticando com o header
> `X-Inbound-Token: <inbound_token do canal>` (sem JWT). Envie **em lotes de ≤2000**
> (use 500–1000). Corpo: `{ "groups": [ { ...grupo... } ] }`, onde cada grupo tem
> **obrigatoriamente** `groupId` (a JID, ex.: `"...@g.us"`) e, quando disponíveis:
> `subject`, `description`, `participants` (array de JIDs como string), `group_admins`
> (ou `admins`), `owner_jid`, `owner_name`, `activated`. NÃO normalize telefones —
> mande as strings cruas. Trate a resposta `{ "ok": true, "upserted": n }`. Em `429`,
> faça backoff exponencial e reenvie o lote; em `401`, pare e logue (token errado);
> em `400`, logue o lote ofensor. O upsert do chat é idempotente por
> `(tenant, group_jid)` e preserva o `attend`, então **re-sincronizar é seguro** —
> rode periodicamente e/ou sob demanda.
>
> Configuração necessária (por canal): `BASE_URL_CHAT`, `inbound_token`,
> `outbound_secret`. Não invente headers de tenant: tenant e canal saem do
> `inbound_token`.
