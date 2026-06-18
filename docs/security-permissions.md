# Segurança e permissões

Modelo de segurança multi-tenant: **autenticação** (auth), **autorização**
RBAC (iam + authz) e **isolamento por tenant** em todas as camadas.

## 1. Autenticação (`auth`)

### Fluxos
- **Login senha:** `POST /auth/login` (email+senha) → `access` (JWT curto) +
  `refresh` (token longo, hash no banco). Senha com **argon2id** (ou bcrypt).
- **Refresh/rotação:** `POST /auth/refresh` → novo access; refresh é
  **rotacionado** (one-time) e o anterior revogado.
- **Logout:** revoga a sessão/refresh.
- **Reset de senha:** `forgot` → e-mail com token de uso único e TTL → `reset`.
- **Serviço (machine-to-machine):** `X-Api-Key` (hash no banco, escopos).

### Tokens
- **Access (JWT):** claims `tid` (tenant), `sub` (user), `roles`, `sid`
  (session), `exp` (curto, ex.: 15 min). Assinado (HS256/RS256 — ver dúvidas).
- **Refresh:** aleatório, **hash** persistido, TTL (ex.: 7–30 dias), rotacionado.
- **WebSocket:** autenticado no upgrade (Bearer ou token de query assinado de
  vida curta para browsers).

### Sessões
- `sessions` e `refresh_tokens` com `expires_at` (TTL index). Revogação por
  logout, troca de senha ou ação admin invalida sessões.

## 2. Autorização (RBAC: `iam` + `authz`)

### Modelo
- **Permission** no formato **`<recurso>.<ação>`** (ponto, não `:`) — capacidade
  fina. A **fonte da verdade** é `domain/authz/permission.go` (`AllPermissions()`):
  **26 permissões**, listadas abaixo. Não invente chaves; o que não estiver nessa
  lista não é emitido pelo backend.
- **Role** — bundle de permissões, por tenant.
- **User** — possui papéis; papéis resolvem o conjunto efetivo de permissões.
- **Authorizer** (`domain/authz`) — decide `Authorize(actor, permission)`;
  fonte de verdade vem de `iam` (papéis do usuário + catálogo).
- **Catálogo para o front:** ainda **não** existe `GET /v1/permissions`. Até
  existir, o front deve espelhar exatamente a lista abaixo (mesmas strings).

### Papéis padrão (seed)
Definidos em `domain/authz/authz.go` (`DefaultRoles()`). São **três** — não há
`supervisor`:

| Papel | Permissões | Escopo de setor |
|---|---|---|
| `owner` | **todas as 26** (`AllPermissions()`) | `ScopeAll` |
| `admin` | 23 — tudo **menos** `contact.view_financial`, `integration.execute_action`, `privacy.manage` | `ScopeAll` |
| `agent` | 8 — `conversation.read/assign/close`, `message.send/internal_note`, `contact.read/write`, `copilot.use` | `ScopeOwn` |

> Papéis são **customizáveis** por tenant; o seed é apenas o ponto de partida
> idempotente.

### Catálogo de permissões (as 30 reais)
```
conversation.read   conversation.assign   conversation.transfer   conversation.close
message.send        message.internal_note message.delete
contact.read        contact.write         contact.view_financial  contact.view_connection_status
sector.manage       queue.manage          user.manage
automation.manage
copilot.use         copilot.configure
integration.read    integration.configure integration.execute_action
channel.manage      webhook.manage
group.view          group.manage
pipeline.view       pipeline.manage
report.view         report.export
audit.view          privacy.manage
```

> O modelo é **grosso de propósito**: não existe `tag.*`, `canned.*`, `reason.*`,
> `sla.*`, `csat.*`, `businesshours.*`, `routing.*`, `notification.*`, `search.*`,
> `attachment.*`, `tenant.*`, `role.*` nem `sector.read/write`. Esses recursos são
> cobertos por permissões existentes (ver mapa abaixo). Chaves nesse formato
> antigo (`recurso:ação`) **não existem** e causam "Sem permissão" no front.

### Mapa permissão → o que libera (para o front montar os gates)
| Funcionalidade (rota) | Permissão exigida |
|---|---|
| Setores & filas (`/sectors`, `/queues`) | `sector.manage` / `queue.manage` |
| **Feriados** (`/holidays`) | `sector.manage` |
| **Horário do channel** (`/channels/{id}/business-status`, `business_hours` em `PATCH /channels/{id}`) | `channel.manage` |
| **Etiquetas, respostas prontas, motivos** (`/tags`, `/canned-responses`, `/close-reasons`) | ler=`conversation.read`, escrever=`sector.manage` |
| Aplicar tag em conversa (`/conversations/{id}/tags`) | `conversation.read` |
| Políticas de SLA (`/sla/policies`) / SLA em risco (`/sla/at-risk`) | escrever=`sector.manage`, ler=`conversation.read` |
| Pesquisas CSAT (`/csat/surveys`) / respostas (`/csat/responses`) | `sector.manage` / `report.view` |
| **Agentes** (`/users`, `/users/invite`) e **Papéis** (`/roles`) e settings do tenant (`/tenants/current`) | `user.manage` |
| Contatos (`/contacts`) | ler=`contact.read`, escrever=`contact.write` |
| Faturas no Customer-360 | `contact.view_financial` |
| Canais (`/channels`) | `channel.manage` |
| **Grupos de WhatsApp** (`/groups`) | ler/buscar=`group.view`, marcar atendimento + sync=`group.manage` |
| **Pipelines de vendas** (`/pipelines`) | ler=`pipeline.view`, configurar funil+estágios=`pipeline.manage` |
| Webhooks (`/webhooks`) | `webhook.manage` |
| Regras de automação (`/automation-rules/*`) | `automation.manage` |
| Copilot usar / configurar (`/copilot/*`) | `copilot.use` / `copilot.configure` |
| MCP servers / tools / aprovar ação (`/mcp/*`) | `integration.configure` / `integration.read` / `integration.execute_action` |
| Providerhub & ações externas (`/providerhub/*`, `/conversations/{id}/external/*`) | ler=`integration.read`, ações=`integration.execute_action` |
| Relatórios (`/reports/*`) / export (`/reports/export`) | `report.view` / `report.export` |
| Auditoria (`/audit`) | `audit.view` |
| Privacidade/retenção (`/privacy/*`) | `privacy.manage` |
| Inbox/mensagens (`/conversations/*`) | `conversation.read/assign/transfer/close`, `message.send/internal_note/delete` |
| `/me`, notificações, presença, busca¹, anexos | só autenticação (sem permissão específica) |

> ¹ `/search/*` exige `conversation.read`.

### Escopo de visibilidade
Além da permissão, há **escopo de dados** (`SectorScope` no papel):
- `ScopeOwn` (ex.: `agent`) — vê conversas **atribuídas a si** ou **dos seus
  setores**.
- `ScopeAll` (ex.: `owner`/`admin`) — vê toda a equipe/tenant.
- Não existe permissão `conversation.read_all`; a amplitude vem do `SectorScope`.
- Filtros de visibilidade aplicados no serviço/repositório, não só na rota.

### Aplicação
- **Middleware `authz`** valida a permissão da rota.
- **Serviço** valida escopo de dados e regras de negócio (ex.: só o assignee ou
  supervisor pode transferir).
- **WS** valida permissão ao assinar tópicos.

## 3. Isolamento multi-tenant

- `tenant_id` **derivado exclusivamente do JWT verificado**, em qualquer
  ambiente. Nenhum header de tenant do cliente é aceito (o middleware legado de
  `X-Tenant-Id` foi removido do código e do CORS).
- `tenant_id` no `context`; `RequireTenant` nos serviços/repositórios.
- **Toda query** Mongo filtra por `tenant_id`; índices compostos começam por
  `tenant_id`.
- **Chaves Redis** prefixadas por tenant (`presence:{tenant}:...`,
  `ratelimit:{tenant}:...`, `idem:{tenant}:...`).
- **Tópicos WS** prefixados por tenant; assinatura cross-tenant é rejeitada.
- **Jobs** carregam `tenant_id`; jobs periódicos iteram tenants explicitamente.

## 4. Endpoints públicos sem JWT

| Endpoint | Verificação |
|---|---|
| Inbound de canal (`/inbound/channel/{channel}/messages`, `.../delivery-receipts`, `.../contact-identity`, `.../groups`) | **inbound_token do canal** (`X-Inbound-Token`/corpo, hash em tempo constante) + HMAC do corpo opcional |
| Coleta de CSAT (link público) | token assinado de vida curta |
| Webhook **outbound** (nosso) | assinamos com HMAC; subscriber valida |
| **Provisionamento de tenant** (`POST /platform/tenants`) | **`X-Platform-Key: key_id.secret`** (plano de plataforma, ACIMA do tenant), `PlatformAuth` separado do `AuthContext`; chaves em `PLATFORM_API_KEYS` (hash em repouso, compare em tempo constante). NÃO carrega contexto de tenant; só cria tenant+owner. Auditado como `platform.tenant_provisioned` (ActorType `platform`, ActorID = key_id). |

### Integração por token de canal (sem JWT)

Para um sistema externo (ex.: gateway de WhatsApp) integrar **sem o Bearer/JWT
do front**, cada canal tem um **token de integração** estável e longo (estilo
Chatwoot `api_access_token`). O Bearer continua **exclusivo** do front↔back; o
`inbound_token` **não** é aceito nas rotas `/v1` protegidas — vale **só** para a
borda pública de canal (inbound/receipts).

- **Geração:** `POST /v1/channels` cria um `inbound_token` de alta entropia (32
  bytes, base62). É guardado **apenas como hash SHA-256** (`inbound_token_hash`);
  o texto em claro é revelado **uma única vez** no `ChannelCreated`. Depois, só
  `has_inbound_token`.
- **Rotação:** `POST /v1/channels/{id}/rotate-inbound-token` emite um novo token
  (revoga o anterior) e o mostra uma vez. Auditado: `channel.token_rotated`.
- **Autenticação:** header `X-Inbound-Token` (preferido) **ou** corpo
  `inbound_token`, comparado **por hash em tempo constante**
  (`subtle.ConstantTimeCompare`). Inválido/desabilitado → **401**.
- **Origem dos dados:** tenant, canal e `default_sector` vêm do **registro do
  canal** — **nunca** de um header de tenant do cliente.
- **HMAC opcional:** se houver `outbound_secret`, o corpo é validado por
  assinatura (`Signature`/`Timestamp`, janela anti-replay); sem assinatura, o
  token sozinho autentica.
- **Rate limit:** limite dedicado por IP na borda pública (`policy.InboundChannelRateLimit`).

## 5. Segredos (`infra/secrets`)

- Credenciais de canal, segredos de webhook e chaves de provider são
  **cifrados em repouso** (envelope encryption; KMS/chave mestra via env).
- O **plaintext** nunca sai do boundary de `secrets`; documentos guardam apenas
  `*_ref`/ciphertext.
- Logs/auditoria **nunca** registram segredos nem tokens.

## 6. Rate limiting e abuso

- Rate limit por **tenant + ator/IP** (contador Redis, janela fixa); `429` +
  `Retry-After`. Limites finos por rota via `policy`.
- Proteções de auth: limite de tentativas de login, lockout temporário,
  invalidação de sessões na troca de senha.

## 7. Auditoria (`audit`)

- Ações sensíveis (login, mudança de papel, exclusão, transferência, alteração
  de canal) geram `audit_log` imutável (`actor`, `action`, `entity`,
  `before/after`, `request_id`).
- Retenção/compactação via `audit.compact` (scheduler).

## 8. Privacidade (`privacy`, LGPD/GDPR)

- **Export** e **erase** (right to be forgotten) sob demanda (jobs
  `privacy.export`/`privacy.erase`).
- **Retenção** automática por entidade (`retention_policies` + scheduler).
- Anonimização de `contacts`/`messages` preservando integridade de métricas.

## 9. Transporte e hardening

- TLS terminado na borda (proxy); HSTS.
- CORS por allow-list (`HTTP_ALLOWED_ORIGINS`).
- Cookies (se usados p/ WS browser) `Secure`/`HttpOnly`/`SameSite`.
- Validação estrita de input (`validator`) → `validation_error`.
- `request_id` em tudo para rastreabilidade.
