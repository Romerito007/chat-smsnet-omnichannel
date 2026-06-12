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
- **Permission** (`<recurso>:<ação>`) — capacidade fina.
- **Role** — bundle de permissões, por tenant.
- **User** — possui papéis; papéis resolvem o conjunto efetivo de permissões.
- **Authorizer** (`domain/authz`) — decide `Authorize(actor, permission)`;
  fonte de verdade vem de `iam` (papéis do usuário + catálogo).

### Papéis padrão (seed)
| Papel | Escopo |
|---|---|
| `owner` | tudo (`*`) — dono do tenant |
| `admin` | administração do tenant (usuários, canais, filas, configs) |
| `supervisor` | visão de equipe, filas, relatórios, reatribuição |
| `agent` | atender conversas atribuídas/da sua fila |

> `owner` recebe `*`. Demais papéis recebem subconjuntos. Papéis são
> **customizáveis** por tenant.

### Catálogo de permissões (base)
```
tenant:read  tenant:write
user:read    user:write     role:read    role:write
contact:read contact:write  contact:delete
sector:read  sector:write   queue:read   queue:write
conversation:read  conversation:write  conversation:assign  conversation:transfer
conversation:close conversation:read_all   # read_all = ver de toda a equipe
message:read message:write   note:write
tag:read tag:write   canned:read canned:write   reason:read reason:write
routing:read routing:write
channel:read channel:write
automation:read automation:write
integration:read   integration:configure   integration:execute_action
copilot:use
sla:read sla:write   csat:read csat:write   businesshours:read businesshours:write
webhook:read webhook:write   notification:read
search:use
privacy:read privacy:write   audit:read
attachment:read attachment:write
report:read
```

### Escopo de visibilidade
Além da permissão, há **escopo de dados**:
- `agent` vê conversas **atribuídas a si** ou **da sua fila/setor**.
- `conversation:read_all` (supervisor/admin) amplia para toda a equipe/tenant.
- Filtros de visibilidade aplicados no serviço/repositório, não só na rota.

### Aplicação
- **Middleware `authz`** valida a permissão da rota.
- **Serviço** valida escopo de dados e regras de negócio (ex.: só o assignee ou
  supervisor pode transferir).
- **WS** valida permissão ao assinar tópicos.

## 3. Isolamento multi-tenant

- `tenant_id` **derivado do token** (nunca confiar em header do cliente em
  produção). `X-Tenant-Id` só é aceito em contextos internos/dev.
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
| Inbound de canal (`/inbound/channel/{channel}/messages`, `.../delivery-receipts`) | **inbound_token do canal** (`X-Inbound-Token`/corpo, hash em tempo constante) + HMAC do corpo opcional |
| Callback de `automation` | HMAC com segredo do binding |
| Coleta de CSAT (link público) | token assinado de vida curta |
| Webhook **outbound** (nosso) | assinamos com HMAC; subscriber valida |

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

- Credenciais de canal, segredos de webhook/automation e chaves de provider são
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
  de canal/automation) geram `audit_log` imutável (`actor`, `action`, `entity`,
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
