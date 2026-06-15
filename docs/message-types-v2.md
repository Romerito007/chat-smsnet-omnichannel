# Message Types v2 — `contact` & `location` (Phase 1)

Status: **contract approved, Phase 1**. Phase 2 (`interactive` buttons/list + `interactive_reply`)
is designed but deferred — not in this document.

This describes two new `message_type`s — **`contact`** and **`location`** — added to the
existing set (`text | image | audio | video | file | template | system`). The Chat never
talks to Meta: everything flows through the **WhatsApp gateway**, which translates this
generic contract to/from the WhatsApp Cloud API. So the shapes below are deliberately a 1:1
mirror of the Meta `contacts` / `location` blocks.

Both types are **bidirectional** (inbound: customer → chat; outbound: agent/automation → chat
→ gateway).

The new types follow the **same pattern as `template`**: a typed, structured field on the
message that travels across five surfaces — **storage, REST, realtime, creation
(SendMessage), and webhook+inbound**. `text` stays as an optional human-readable fallback
(used for inbox preview / search); the typed field is the source of truth.

---

## 1. `contact` — vCard(s)

A message carrying **1..10 contact cards** (WhatsApp allows several per message).

### Wire shape (identical on REST, realtime, webhook, and inbound)
```json
{
  "message_type": "contact",
  "text": "Cartão: Maria Silva (+5511999998888)",
  "contacts": [
    {
      "name": { "formatted": "Maria Silva", "first": "Maria", "last": "Silva" },
      "phones": [ { "phone": "+5511999998888", "type": "CELL", "wa_id": "5511999998888" } ],
      "emails": [ { "email": "maria@example.com", "type": "WORK" } ],
      "organization": { "company": "ACME", "title": "Gerente" }
    }
  ]
}
```
- `name.formatted` is the display name. `first`/`last` optional.
- `phones[]` — at least one. `type` is a free hint (`CELL|HOME|WORK|…`); `wa_id` is the
  WhatsApp id when known (optional).
- `emails[]`, `organization` — optional.

### Validation (enforced by the Chat, so the gateway only translates)
- `contacts`: **1..10** items.
- each contact: `name.formatted` (or `first`/`last`) **and** ≥1 `phone` (non-empty).
- `phone` should be E.164-ish (`+` + digits); not strictly enforced beyond non-empty.

### Storage
`Message.Contacts []ContactCard` (typed BSON sub-document). Nothing in `metadata`.

### Rendering (agent)
N cards: display name + clickable phones (`tel:` / "open chat") + emails + organization.

### Meta mapping (gateway)
- **Outbound** (Chat webhook → Meta send):
  ```json
  { "type": "contacts", "contacts": [ {
      "name": { "formatted_name": "Maria Silva", "first_name": "Maria", "last_name": "Silva" },
      "phones": [ { "phone": "+5511999998888", "type": "CELL", "wa_id": "5511999998888" } ],
      "emails": [ { "email": "maria@example.com", "type": "WORK" } ],
      "org": { "company": "ACME", "title": "Gerente" }
  } ] }
  ```
  Field renames the gateway applies: `name.formatted`→`formatted_name`, `first`→`first_name`,
  `last`→`last_name`, `organization`→`org`.
- **Inbound** (Meta webhook → Chat): Meta delivers `messages[].contacts[]` in the same shape;
  the gateway maps back to our `contacts[]` (reverse renames) and POSTs an inbound message
  with `message_type: "contact"`.

---

## 2. `location`

A single geographic point, optionally named.

### Wire shape (identical on all surfaces)
```json
{
  "message_type": "location",
  "text": "Loja Centro — Av. Brasil, 100",
  "location": {
    "latitude": -27.5954,
    "longitude": -48.5480,
    "name": "Loja Centro",
    "address": "Av. Brasil, 100"
  }
}
```

### Validation
- `latitude` ∈ [-90, 90], `longitude` ∈ [-180, 180] (required floats).
- `name`, `address` — optional strings.

### Storage
`Message.Location *Location`.

### Rendering (agent)
Static map thumbnail or an "open in maps" link (`https://maps.google.com/?q=lat,lng`) plus
`name`/`address`.

### Meta mapping (gateway)
- **Outbound**: `{ "type": "location", "location": { "latitude", "longitude", "name", "address" } }`
- **Inbound**: Meta delivers `messages[].location` in the same shape → POST inbound with
  `message_type: "location"`.

---

## 3. Where each field appears (the five surfaces)

| Surface | Carrier | Notes |
|---|---|---|
| **Storage** | `entity.Message.Contacts` / `.Location` | typed BSON sub-docs |
| **REST** | `MessageResponse.contacts` / `.location` | `GET /v1/conversations/{id}/messages` |
| **Realtime** | `MessagePayload.contacts` / `.location` | WS event `message.created` (and `.updated`) |
| **Creation** | `SendMessageRequest.contacts` / `.location` | `POST /v1/conversations/{id}/messages` (agent) and automation actions |
| **Webhook (out)** | `NewIntegrationMessagePayload` (built on `MessagePayload`) | delivered to the gateway `outbound_url`; the gateway translates to Meta |
| **Inbound (in)** | `InboundMessageRequest.contacts` / `.location` | gateway → `POST /v1/inbound/channel/{channel}/messages` |

`text` is optional for both types (a short human-readable fallback for inbox preview/search);
the structured field is authoritative.

---

## 4. Examples

### Agent sends a contact — `POST /v1/conversations/{id}/messages`
```json
{ "message_type": "contact",
  "contacts": [ { "name": { "formatted": "Suporte ACME" },
                  "phones": [ { "phone": "+5511888887777" } ] } ] }
```

### Customer shares a location — gateway → `POST /v1/inbound/channel/whatsapp/messages`
```json
{ "inbound_token": "…", "external_message_id": "wamid.X", "external_contact_id": "5511…",
  "contact_phone": "+5511…", "message_type": "location",
  "location": { "latitude": -23.56, "longitude": -46.64, "name": "Cliente" } }
```

### Outbound webhook to the gateway (Chat → gateway)
The `message.created` webhook payload carries `message_type` + the typed field exactly as in
§1/§2 (attachment URLs are already swapped for signed channel-media URLs as today).

---

## 5. Notes for the gateway team (translation checklist)
For **each** type, both directions:
- **contact** ⇄ Meta `contacts[]` (`name.formatted_name`, `phones[].phone/type/wa_id`,
  `emails[]`, `org{company,title}`).
- **location** ⇄ Meta `location{latitude,longitude,name,address}`.
- Delivery receipts (sent/delivered/read/failed) for these types use the existing flow — no
  change.
- The Chat validates everything before the webhook fires, so the gateway can assume a valid
  payload and only needs to **rename/restructure**, not re-validate.

Please confirm the field names above match what the gateway already exchanges with Meta. Any
mismatch (e.g. `formatted` vs `formatted_name`) is handled by the gateway's translation layer.
