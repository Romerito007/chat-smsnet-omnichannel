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

---

# Mensagens de grupo (Domínio 2) — a "língua oficial"

Esta é a parte que **os dois gateways (oficial e não-oficial) falam IGUAL**: o chat
**dita o contrato**, o gateway se adapta. Vale tanto pra canal `api` (WhatsApp não
oficial) quanto `whatsapp` (oficial) — **o reconhecimento é pela JID do grupo
(`@g.us`), nunca por tipo de canal**.

## Receber mensagem de grupo (gateway → chat)

**Mesmo endpoint de sempre:** `POST /v1/inbound/channel/{channel}/messages` (auth
`X-Inbound-Token`, sem JWT). Uma mensagem de grupo é uma mensagem normal **+ campos
novos, todos flat e opcionais**. **O sinal de reconhecimento é a presença de
`group_jid`.**

```json
{
  "external_message_id": "wamid.XYZ",
  "text": "alguém viu o boleto?",

  "group_jid":    "120363025246125486@g.us",
  "sender_jid":   "5544999990000@s.whatsapp.net",
  "sender_name":  "João",
  "sender_phone": "5544999990000",

  "attachments": [], "timestamp": 1718600000000
}
```

| Campo | Obrigatório? | Papel |
|---|---|---|
| `group_jid` | **sim, quando é grupo** (`@g.us`) | identidade da **conversa de grupo** (a chave). Presença = "é grupo" |
| `sender_jid` | recomendado | **quem** (membro) mandou — vira metadado da mensagem (`group_sender`) |
| `sender_name` | recomendado | pushname/exibição → o chat mostra "João:" |
| `sender_phone` | opcional | dígitos do membro, se tiver |

**Regras (idênticas pros dois gateways):**
- **Sem `group_jid`** → mensagem 1-para-1, comportamento atual **intacto**. Aí `sender_*`
  não se aplicam e `external_contact_id`/`contact_phone` continuam obrigatórios.
- **Com `group_jid`** → mensagem de grupo. A conversa é **uma só por grupo**;
  `external_contact_id`/`contact_phone` **não** são a chave (a chave é o `group_jid`).
  A autoria do membro vai em `sender_*` — o membro **nunca** vira contato.
- Sempre mande a **JID completa** do grupo (`...@g.us`). Reconhecimento é por ela,
  **nunca** por `if channel == whatsapp`.
- `text`, `attachments`, mídia, `timestamp`, `external_message_id` — iguais ao 1:1.

**Tipos ricos em grupo (contact e location) — nativos:** o chat tem
`message_type=contact` e `message_type=location` de primeira classe (não é texto
fingido). Para materializarem **na conversa do grupo**, mande-os **com `group_jid`**,
exatamente na mesma shape do 1:1:

```jsonc
// contato compartilhado no grupo (JSON)
{ "external_message_id":"...", "group_jid":"...@g.us",
  "sender_jid":"...@s.whatsapp.net", "sender_name":"João",
  "text":"compartilhou um contato",
  "contacts":[ { "name":{"formatted":"Maria"}, "phones":[{"phone":"5544111"}] } ] }

// localização compartilhada no grupo (JSON)
{ "external_message_id":"...", "group_jid":"...@g.us",
  "sender_jid":"...@s.whatsapp.net", "sender_name":"João", "text":"estou aqui",
  "location":{ "latitude":-23.55, "longitude":-46.63, "name":"SP", "address":"Centro" } }
```
- A shape de `contacts[]`/`location` é a **mesma do 1:1** — respeite os nomes
  (`name.formatted`, `phones[].phone`; `latitude`/`longitude`/`name`/`address`). Uma
  shape diferente faz o campo chegar **vazio** e o chat cai em `400` (sem nome no
  contato) ou cria só o texto.
- **NÃO esqueça o `group_jid`** nesses dois tipos. Sem ele, o chat trata como **1:1**
  e a mensagem **não** aparece na conversa do grupo. (Foi a causa do bug em que
  contact/location "sumiam": iam sem `group_jid` ou via multipart que descartava os
  campos.)
- Se mandar por **multipart** (em vez de JSON), os mesmos campos valem como campos de
  formulário: `group_jid`, `sender_jid`, `sender_name`, `sender_phone`, e
  `contacts`/`location` como **strings JSON**.

**Gate de atendimento (o chat decide, o gateway não precisa saber):**
- Se o grupo **não foi sincronizado** (não está no registry do Domínio 1) **ou**
  está marcado como **não atender** (`attend=false`) → o chat **descarta** a mensagem
  e responde **`200 OK`** assim mesmo (nada é persistido). O `200` evita o gateway
  re-tentar em loop; a próxima mensagem é reavaliada (se o operador ligar o atendimento
  depois, ela entra). **O gateway não muda nada** — só manda; o filtro é do chat.
- Se o grupo é atendido → o chat cria **UM** contato-tipo-grupo (identidade = a JID
  `@g.us`) e **UMA** conversa, e registra o `sender_*` como metadado da mensagem.

## Responder no grupo (chat → gateway)

Sem novidade de transporte: a resposta do atendente sai no **mesmo webhook
`message_created`** que o gateway já recebe. O destinatário vem no bloco `contact`:

```json
{
  "event": "message_created",
  "data": {
    "id": "msg_...", "direction": "outbound", "text": "claro, segue o boleto",
    "contact": {
      "id": "ct_...",
      "is_group": true,
      "identities": [{ "channel": "whatsapp", "external_id": "120363025246125486@g.us" }]
    }
  }
}
```
- **`contact.identities[].external_id` é a JID do GRUPO** (`@g.us`) — o gateway disca
  pra essa JID exatamente como discaria pra uma pessoa. **Nada muda no roteamento.**
- **`contact.is_group: true`** vem junto, pra o gateway saber que é grupo **sem
  precisar parsear o sufixo `@g.us`**.

---

# Sincronizar templates de WhatsApp (mesmo padrão dos grupos)

O cadastro de **templates** espelha **ponto a ponto** o sync de grupos (Domínio 1): o
chat **dispara um evento**, o gateway **empurra de volta** a lista. O chat **não
puxa** — é o mesmo trilho, só muda o `event` e o endpoint de inbound.

## Visão geral do fluxo

```
                  1. POST /v1/channels/{id}/sync-templates   (supervisor no chat)
                          │
                          ▼
   ┌──────────┐   2. webhook "templates_sync_requested"  ┌────────────────────┐
   │   CHAT   │ ───────────────────────────────────────► │   GATEWAY WhatsApp  │
   │ (backend)│      (entregue ao outbound_url             │ (você implementa)  │
   └──────────┘       do canal, assinado HMAC)            └────────────────────┘
        ▲                                                          │
        │   3. POST /v1/inbound/channel/whatsapp/templates         │
        │      { inbound_token, templates:[...] }   (lista INTEIRA) │
        └──────────────────────────────────────────────────────────┘
              (auth por inbound_token; substitui o mirror)
```

São **dois passos** do lado do gateway:

1. **Receber** o evento `templates_sync_requested` (webhook ao `outbound_url` do canal).
2. **Responder** listando os templates aprovados (Meta/BSP) e fazendo `POST` da lista
   **completa** para o endpoint de inbound de templates.

Nada disso é síncrono: o `POST /v1/channels/{id}/sync-templates` responde `202` na
hora; os templates chegam depois, no `POST` que o gateway empurra.

## Passo 2 — Receber o evento `templates_sync_requested`

Igual ao `group_sync_requested`: entregue **só ao webhook gerenciado do canal**, no
**mesmo transporte** dos outros eventos — **não há rota nova**, só tratar mais um
`event`.

**HTTP:** `POST {outbound_url do canal}`

**Headers:**

| Header | Valor |
|---|---|
| `Content-Type` | `application/json` |
| `X-Webhook-Event` | `templates_sync_requested` |
| `X-Webhook-Timestamp` | epoch em segundos (string) |
| `X-Webhook-Signature` | `sha256=<hex>` — `HMAC_SHA256(outbound_secret, timestamp + "." + rawBody)` |
| `X-Webhook-Delivery-Id` | id único da entrega (idempotência) |

**Body (envelope canônico de webhook):**

```json
{
  "id": "del_...",
  "event": "templates_sync_requested",
  "created_at": "2026-06-18T12:00:00Z",
  "data": { "channel_id": "<id do canal>" }
}
```

Valide a assinatura igual aos demais eventos (comparação em tempo constante, janela
anti-replay, dedupe por `X-Webhook-Delivery-Id`). Responda `2xx` rápido e processe
assíncrono. O `data.channel_id` diz **qual canal** (e, com ele, o `inbound_token` a
usar no passo 3). Sem webhook gerenciado no canal (sem `outbound_url`) o chat
responde `409` ao supervisor e o evento nem sai.

## Passo 3 — Empurrar os templates (lista inteira)

**HTTP:** `POST {base_url_do_chat}/v1/inbound/channel/whatsapp/templates`
(`{channel}` = o **type** do canal, `whatsapp`).

**Auth (sem JWT):** header `X-Inbound-Token: <inbound_token do canal>` (ou campo
`inbound_token` no corpo).

**Body** — a **lista completa** no shape `WhatsAppTemplate`:

```json
{
  "inbound_token": "<opcional se já mandou no header>",
  "templates": [
    {
      "id": "hsm_boas_vindas",
      "name": "Boas-vindas",
      "language": "pt_BR",
      "category": "MARKETING",
      "body": { "text": "Olá {{nome}}, tudo bem?", "variables": [ { "key": "nome", "label": "Nome", "example": "João" } ] },
      "header": { "type": "text", "text": "Oi" },
      "buttons": [ { "type": "url", "text": "Site", "url": "https://exemplo.com" } ],
      "footer": "Equipe Acme"
    }
  ]
}
```

| Campo | Obrigatório? | Observação |
|---|---|---|
| `id` | **sim** | id opaco do integrador (Meta/BSP); guardado e ecoado verbatim |
| `name` | **sim** | rótulo do seletor |
| `language` | opcional | ex.: `pt_BR` |
| `category` | opcional | só exibição |
| `body.text` | opcional | texto com `{{placeholders}}` |
| `body.variables[]` | opcional | `{ key, label?, example? }` — `key` é o nome do placeholder |
| `header` / `buttons` / `footer` | opcional | só exibição (render-only) |

**Resposta:** `200 OK` → `{ "ok": true, "count": <n> }`.

**Comportamento:**
- O chat valida (`id`/`name` obrigatórios; `variables[].key` obrigatório quando há
  variáveis) e **substitui o `whatsapp_templates` do canal INTEIRO** (é mirror) —
  mande sempre a lista **completa**, não um delta.
- **Idempotente:** reenviar a mesma lista substitui sem duplicar.
- **Tenant/canal** vêm só do `inbound_token` (nunca de header de tenant).
- Após salvar, o chat **notifica os atendentes** in-app (o sino) que os modelos do
  canal foram atualizados.

**Erros:**

| Status | Causa |
|---|---|
| `401` | `inbound_token` inválido/ausente ou canal desabilitado |
| `400` | JSON inválido ou template sem `id`/`name` (lista NÃO é substituída) |

O front continua lendo os templates por `GET /v1/channels/{id}` (inalterado).

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
> **3) Entregar mensagens DE grupo (Domínio 2).** Quando chegar uma mensagem num
> grupo de WhatsApp, faça o **mesmo** `POST {BASE_URL_CHAT}/v1/inbound/channel/whatsapp/messages`
> que você já faz pra 1:1, **adicionando 4 campos flat**: `group_jid` (a JID do grupo,
> `"...@g.us"` — OBRIGATÓRIO, é o que sinaliza "é grupo"), `sender_jid` (a JID do
> membro que mandou), `sender_name` (pushname dele) e, se tiver, `sender_phone`.
> Mantenha `text`/`attachments`/`timestamp`/`external_message_id` iguais ao 1:1. NÃO
> mande `external_contact_id`/`contact_phone` como chave num grupo (a chave é o
> `group_jid`). O chat pode responder `200` e **descartar** silenciosamente (grupo não
> sincronizado ou marcado pra não atender) — isso é esperado, **não** re-tente.
>
> **4) Responder no grupo.** Nada novo: a resposta sai no webhook `message_created`
> que você já consome. O destinatário é `data.contact.identities[].external_id`, que
> numa conversa de grupo é a **JID do grupo** (`@g.us`); o bloco traz também
> `contact.is_group: true`. Disque pra essa JID como discaria pra uma pessoa.
>
> Configuração necessária (por canal): `BASE_URL_CHAT`, `inbound_token`,
> `outbound_secret`. Não invente headers de tenant: tenant e canal saem do
> `inbound_token`.

---

## Prompt pronto — sincronização de TEMPLATES (cole no agente do gateway)

> Você vai implementar, no gateway de WhatsApp, a **sincronização de templates** com o
> chat omnichannel. É o **mesmo padrão** do sync de grupos — o chat dispara um evento e
> você empurra a lista de volta. São dois pontos:
>
> **1) Tratar o webhook `templates_sync_requested`.** O chat já entrega webhooks ao
> `outbound_url` do canal. Adicione o tratamento do `event == "templates_sync_requested"`
> (idêntico ao `group_sync_requested`). Headers: `X-Webhook-Event`,
> `X-Webhook-Timestamp`, `X-Webhook-Signature` (`sha256=<hex>`),
> `X-Webhook-Delivery-Id`. Valide a assinatura
> `HMAC_SHA256(outbound_secret, timestamp + "." + rawBody)` em hex, comparação em tempo
> constante, janela anti-replay de ±5 min; dedupe por `X-Webhook-Delivery-Id`. O corpo é
> `{ "id", "event", "created_at", "data": { "channel_id": "<id>" } }`. Responda `2xx`
> rápido e processe assíncrono.
>
> **2) Empurrar a lista de templates.** Ao receber o evento, monte a **lista completa**
> de templates aprovados da instância correspondente ao `data.channel_id` e faça
> `POST {BASE_URL_CHAT}/v1/inbound/channel/whatsapp/templates`, autenticando com o header
> `X-Inbound-Token: <inbound_token do canal>` (sem JWT). Corpo:
> `{ "templates": [ { ...template... } ] }`, onde cada template tem **obrigatoriamente**
> `id` (id opaco do integrador) e `name`, e quando disponíveis: `language`, `category`,
> `body { text, variables:[{key,label?,example?}] }`, `header { type, text }`,
> `buttons:[{type,text,url?}]`, `footer`. **Mande a lista INTEIRA** (é um mirror — o chat
> substitui tudo; não mande delta). Trate a resposta `{ "ok": true, "count": n }`. Em
> `401`, pare e logue (token errado); em `400`, logue (template sem `id`/`name` — a lista
> não foi substituída). É idempotente: re-enviar substitui sem duplicar, então rode sob
> demanda (ao receber o evento) e/ou periodicamente.
>
> Configuração necessária (por canal): `BASE_URL_CHAT`, `inbound_token`,
> `outbound_secret`. Tenant e canal saem do `inbound_token`.
