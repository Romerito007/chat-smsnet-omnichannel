# Canal de API — compatível com o canal API do Chatwoot

O **canal de API** troca mídia (áudio, imagem, vídeo, documento) no mesmo formato
que o **canal API do Chatwoot**, para sistemas já integrados ao Chatwoot plugarem
sem mudança. Suporta os **dois** formatos do Chatwoot: **(a) multipart/form-data**
com arquivo bruto e **(b) mídia por URL**.

## 1. Inbound (cliente → nós): `POST /v1/inbound/channel/{channel}/messages`

Autenticação: `X-Inbound-Token: <token>` (preferido) ou campo `inbound_token` no
corpo. HMAC é **opcional** (multipart não assina): o token sozinho autentica.

### (a) multipart/form-data (estilo create-message do Chatwoot)
Campos de formulário:
- `content` (texto, opcional) — alias aceito: `text`
- `message_type`, `private` (bool), `file_type ∈ image|audio|video|document` (informativo)
- `attachments[]` — um ou mais **arquivos brutos** (com `filename` + `Content-Type`
  por parte). Aliases de campo aceitos: `attachments`, `file`.
- Roteamento (nossos): `inbound_token`, `external_message_id`,
  `external_contact_id` **ou** `contact_phone`, `contact_name?`, `contact_document?`,
  `timestamp?` (epoch ms).

```bash
curl -X POST https://api.exemplo/v1/inbound/channel/api/messages \
  -H "X-Inbound-Token: <TOKEN>" \
  -F "external_message_id=m-123" \
  -F "external_contact_id=+5544999088478" \
  -F "content=segue o áudio" \
  -F "file_type=audio" \
  -F "attachments[]=@nota.ogg;type=audio/ogg"
```
O backend **persiste** cada arquivo no storage (S3/local já existente), cria a
`Message` com os anexos resolvidos (`id`, `content_type`, `size`, `url` =
`download_url` access-gated) e publica `message.created` no realtime. Áudio entra
como mensagem normal com anexo `audio/*`.

### (b) JSON com URL (já existente)
```json
{
  "inbound_token": "<TOKEN>",
  "external_message_id": "m-123",
  "external_contact_id": "+5544999088478",
  "text": "segue o áudio",
  "attachments": [
    { "url": "https://cdn.exemplo/nota.ogg", "content_type": "audio/ogg",
      "filename": "nota.ogg", "size": 12345 }
  ]
}
```

### Content-types de áudio aceitos
Não há allowlist restrita por padrão (`ATTACHMENTS_ALLOWED_CONTENT_TYPES` vazio =
aceita qualquer tipo). Áudio comum suportado: **`audio/mpeg` (mp3), `audio/ogg`
(opus), `audio/webm`, `audio/mp4`/`audio/m4a`, `audio/wav`**. Se você definir uma
allowlist, inclua `audio/*` (e `image/*`, `video/*`). Limite por arquivo: 25 MiB
(`ATTACHMENTS_MAX_SIZE_BYTES`); corpo multipart até 30 MiB.

## 2. Outbound (nós → endpoint do canal API do tenant)

Quando o atendente envia uma mensagem (incl. áudio), a **entrega** é um `POST`
assinado (HMAC do corpo com `outbound_secret`, headers `X-Chat-Event`,
`Timestamp`, `Signature`, `Delivery-Id`) ao `outbound_url` do canal, no formato
**compatível com o webhook do Chatwoot** (JSON com `data_url` + `file_type`):

```json
{
  "delivery_id": "d-1",
  "conversation_id": "conv-1",
  "timestamp": 1718000000000,
  "contact": { "id": "c1", "name": "Maria", "phone": "+5544...", "external_id": "+5544..." },
  "message": {
    "content": "segue o áudio",
    "text": "segue o áudio",
    "message_type": "outgoing",
    "private": false,
    "file_type": "audio",
    "attachments": [
      { "url": "https://api.exemplo/v1/attachments/<id>/download",
        "data_url": "https://api.exemplo/v1/attachments/<id>/download",
        "file_type": "audio", "content_type": "audio/ogg",
        "filename": "nota.ogg", "size": 12345 }
    ]
  }
}
```
`file_type` é derivado do `content_type`: `audio/*`→`audio`, `image/*`→`image`,
`video/*`→`video`, senão `file` (valor do Chatwoot para documentos). O receptor
que já fala Chatwoot processa via `data_url`/`file_type` sem adaptação.

> O shape outbound está modelado no OpenAPI como `ChannelOutboundMessage` (é um
> webhook que **nós** emitimos, não uma rota `/v1` servida).

## 3. Composição pelo atendente (nosso front → backend)

O front **mantém o fluxo interno** de anexo (`upload-url` → `PUT` → `confirm` →
`SendMessageRequest.attachments`). O front grava áudio, sobe por esse fluxo e a
mensagem sai; a **conversão para o "dialeto Chatwoot" acontece na entrega do
canal** (item 2), não no front. `SendMessageRequest.attachments` aceita anexos de
áudio e a `Message` resultante expõe `content_type` `audio/*` + `url`
(`download_url` access-gated) para o player.
