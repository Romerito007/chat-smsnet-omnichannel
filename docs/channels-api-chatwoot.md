# Dois trilhos: interno (dashboard/JWT) × integração (canal/token)

O backend mantém **dois contratos estritamente separados e não intercambiáveis**,
como o Chatwoot (Application API × API de canal):

| | **Trilho INTERNO** (dashboard/agente) | **Trilho de INTEGRAÇÃO** (máquina-a-máquina) |
|---|---|---|
| Auth | JWT Bearer (humano) | `X-Inbound-Token` do canal (`security: []`, sem JWT) |
| Rotas | `/v1/conversations/...`, `/v1/attachments/upload-url\|confirm`, `/v1/attachments/{id}/download` | `/v1/inbound/channel/{channel}/messages`, entrega ao `outbound_url`, `/v1/channel-media/{token}` |
| Mídia | fluxo de anexo (upload-url → PUT → confirm → `SendMessageRequest.attachments`); download **JWT-gated** | "dialeto Chatwoot" (multipart na entrada; `data_url`+`file_type` na saída); download por **token assinado** |
| Formato | nossos DTOs internos | espelha a superfície do Chatwoot |

O "dialeto Chatwoot" de mídia entra e sai **exclusivamente** pelo trilho de
integração. O front (dashboard) **só** usa o trilho interno e nunca vê tokens de
canal nem URLs assinadas de integração.

---

## Trilho INTERNO — apenas destravar áudio (contrato inalterado)

O agente compõe pelo fluxo interno: `POST /v1/attachments/upload-url` →
`PUT` (signed) → `POST /v1/attachments/confirm` → `SendMessageRequest.attachments`
em `POST /v1/conversations/{id}/messages` (tudo **JWT**).

- **Áudio aceito** (sem allowlist restrita por padrão): `audio/mpeg` (mp3),
  `audio/ogg` (opus), `audio/webm`, `audio/mp4`/`audio/m4a`, `audio/wav` — além de
  imagem/vídeo/documento. Se você ativar `ATTACHMENTS_ALLOWED_CONTENT_TYPES`,
  inclua `audio/*` (e `image/*`, `video/*`).
- A `Message` resultante expõe `content_type` (`audio/*`) e `url` = **URL de mídia
  assinada SEM JWT** (`/v1/channel-media/{token}`, mesmo mecanismo HMAC do webhook/
  avatar): carrega direto em `<img>/<audio>/<video> src`, sem header Authorization e
  sem access-check por imagem. É um token portador, time-boxed (`MediaURLTTL`, ~24h),
  regenerado a cada leitura/realtime. Para download access-checked por permissão,
  continua existindo `GET /v1/attachments/{id}/download` (JWT).
- O endpoint interno **não** vira endpoint de integração; a conversão Chatwoot
  ocorre só na entrega do canal (abaixo).

---

## Trilho de INTEGRAÇÃO — formato compatível com o canal API do Chatwoot

### 1. Inbound (sistema externo → nós) — `POST /v1/inbound/channel/{channel}/messages`
Auth: `X-Inbound-Token: <token>` (ou campo `inbound_token`). HMAC **opcional**
(o token já autentica). Dois modos:

**(a) multipart/form-data** (estilo create-message do Chatwoot):
- `content` (ou `text`, opcional), `message_type`, `private`,
  `file_type ∈ image|audio|video|document`
- **`attachments[]`** = arquivo(s) bruto(s) com `filename` + `Content-Type` por parte
  (aliases de campo: `attachments`, `file`)
- roteamento: `inbound_token`, `external_message_id`,
  `external_contact_id` **ou** `contact_phone`, `contact_name?`,
  `contact_document?`, `timestamp?` (epoch ms)

```bash
curl -X POST .../v1/inbound/channel/api/messages -H "X-Inbound-Token: <T>" \
  -F external_message_id=m1 -F external_contact_id=+5544999088478 \
  -F content="segue áudio" -F file_type=audio \
  -F "attachments[]=@nota.ogg;type=audio/ogg"
```

**(b) JSON** com mídia por URL:
```json
{ "inbound_token":"<T>","external_message_id":"m1","external_contact_id":"+5544999088478",
  "text":"segue áudio",
  "attachments":[{"url":"https://cdn/nota.ogg","content_type":"audio/ogg","filename":"nota.ogg","size":12345}] }
```
Em ambos: o backend **persiste no storage interno**, cria a `Message` com o anexo
resolvido e publica `message.created`. Content-types de áudio aceitos: `audio/mpeg`,
`audio/ogg`, `audio/webm`, `audio/mp4`, `audio/wav`.

### 2. Outbound (nós → sistema externo) — entrega ao `outbound_url` do canal
`POST` assinado (HMAC do corpo com `outbound_secret`; headers `X-Chat-Event`,
`Timestamp`, `Signature`, `Delivery-Id`). Corpo no shape do webhook do Chatwoot,
com `file_type` e a mídia por **URL assinada e pública** (trilho de integração —
**não** o download JWT-gated do trilho interno):

```json
{ "delivery_id":"d1","conversation_id":"conv1","timestamp":1718000000000,
  "contact":{"id":"c1","name":"Maria","phone":"+55..","external_id":"+55.."},
  "message":{ "content":"segue áudio","text":"segue áudio","message_type":"outgoing","private":false,
    "file_type":"audio",
    "attachments":[{
      "url":"https://api.exemplo/v1/channel-media/<token-assinado>",
      "data_url":"https://api.exemplo/v1/channel-media/<token-assinado>",
      "file_type":"audio","content_type":"audio/ogg","filename":"nota.ogg","size":12345 }] } }
```
`file_type` derivado do `content_type`: `audio/*→audio`, `image/*→image`,
`video/*→video`, senão `file`. O `data_url` aponta para
**`GET /v1/channel-media/{token}`** (`security: []`): token HMAC-assinado e
expirável (`ATTACHMENTS_SIGNING_SECRET`, TTL ~24h) que o sistema externo busca
**sem JWT**. O receptor que já fala Chatwoot processa via `data_url`/`file_type`
sem adaptação. Modelado no OpenAPI como `ChannelOutboundMessage` + a rota pública
`/v1/channel-media/{token}`.

> A mesma `Message` tem **duas** URLs por trilho: interna (JWT, p/ o front) e
> assinada/pública (integração, p/ o sistema externo). A interna fica armazenada;
> a de integração é gerada e assinada **na hora da entrega** do canal.
