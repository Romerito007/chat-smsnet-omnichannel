# Módulos de backend (domínios)

Cada domínio segue o molde `contracts/ + entity/ + repository/ + service/`
(adicione `events/`, `provider/` ou `cache/` quando necessário). Abaixo, a
responsabilidade, as entidades principais, as dependências e o que cada domínio
expõe (REST / WS / jobs).

> Convenções: **REST** = recursos HTTP; **WS** = eventos de tempo real
> emitidos; **Jobs** = tasks Asynq produzidas/consumidas. Detalhes completos em
> [api-design.md](api-design.md), [realtime-events.md](realtime-events.md) e
> [asynq-jobs.md](asynq-jobs.md).

---

## Núcleo de identidade e organização

### `tenant`
- **Responsabilidade:** empresa/conta. Raiz do isolamento; configurações da
  organização (nome, plano, feature flags, limites, fuso padrão).
- **Entidades:** `Tenant`, `TenantSettings`.
- **Depende de:** —
- **Expõe:** REST (admin do tenant); seed inicial cria o primeiro tenant.
- **Nota:** todo outro domínio referencia `tenant_id`.

### `auth`
- **Responsabilidade:** autenticação e ciclo de vida de conta. Login (senha),
  emissão/rotação de **JWT** + refresh tokens, logout; **signup self-service**
  (cria empresa + owner pending), **verificação de e-mail**, **convite/aceite**
  de usuário, **esqueci/redefinir senha** e **perfil** (PATCH `/me`,
  troca de senha). Tokens de conta são **uso único, com hash no banco e
  expiração**.
- **Entidades:** `RefreshToken`, `EmailVerificationToken`,
  `PasswordResetToken`, `Invitation` (todas guardam só o hash do token).
- **Depende de:** `iam` (usuários/papéis), `tenant` (signup cria o tenant),
  `infra/email` (envio real via SMTP).
- **Expõe (público):** `POST /auth/login`, `/auth/refresh`, `/auth/signup`,
  `/auth/verify-email`, `/auth/resend-verification`, `/auth/forgot-password`
  (resposta neutra), `/auth/reset-password`, `/auth/accept-invite`.
  **Autenticado:** `/auth/logout`, `GET/PATCH /me`, `/me/change-password`.
  signup/forgot/resend têm rate limit por IP mais estrito.
- **Segurança/auditoria:** audita `auth.signup`, `auth.verify_email`,
  `auth.password_reset(_requested)`, `auth.password_changed`, `user.invited`,
  `auth.invite_accepted`. Reset de senha revoga todas as sessões.

### `iam`
- **Responsabilidade:** *Identity & Access Management*. Usuários
  (agentes/supervisores/admins), **papéis** e **permissões** (RBAC), atribuição
  de papéis, vínculo de usuário a setores/filas.
- **Entidades:** `User` (com `status` active|disabled|pending_verification e
  `avatar_attachment_id`), `Role`, `Permission`, `RoleAssignment`.
- **Depende de:** `tenant`.
- **Expõe:** REST (`/users`, `/users/invite`, `/roles`); fonte de verdade do
  `Authorizer`. O avatar do usuário reusa o fluxo de `attachments` (URL
  assinada): o cliente sobe o arquivo e envia o `avatar_attachment_id`.
- **Nota:** catálogo de permissões em
  [security-permissions.md](security-permissions.md).

---

## Atendimento

### `contacts`
- **Responsabilidade:** pessoas que conversam (clientes). Perfil, identidades
  por canal (telefone, @telegram, e-mail), campos customizados, deduplicação,
  merge.
- **Entidades:** `Contact`, `ContactIdentity`, `CustomField`.
- **Depende de:** `tenant`, `privacy` (anonimização/retenção).
- **Expõe:** REST (`/contacts`); WS (`contact.updated`).

### `sectors`
- **Responsabilidade:** setores/departamentos (ex.: Vendas, Suporte). Agrupam
  filas e agentes; base para roteamento e visibilidade.
- **Entidades:** `Sector`, `SectorMembership`.
- **Depende de:** `tenant`, `iam`.
- **Expõe:** REST (`/sectors`).

### `queues`
- **Responsabilidade:** filas de atendimento dentro de um setor. Capacidade,
  estratégia de distribuição, agentes elegíveis, conversas aguardando.
- **Entidades:** `Queue`, `QueueMembership`.
- **Depende de:** `sectors`, `iam`.
- **Expõe:** REST (`/queues`); WS (`queue.stats`).

### `presence`
- **Responsabilidade:** presença/disponibilidade dos agentes em tempo real
  (online/away/busy/offline), capacidade corrente, “digitando”. Estado volátil.
- **Entidades:** `AgentPresence` (em **Redis**, com TTL/heartbeat).
- **Depende de:** `iam`, `realtime`.
- **Expõe:** WS (`presence.changed`, `typing`); REST de leitura agregada.
- **Nota:** **não** persiste em Mongo; é Redis-only por design.

### `conversations`
- **Responsabilidade:** coração do sistema. Conversas, **mensagens**,
  participantes, ciclo de vida (open → assigned → pending → resolved → closed),
  transferências, notas internas, leitura/entrega.
- **Entidades:** `Conversation`, `Message`, `Participant`.
- **Depende de:** `contacts`, `channels`, `routing`, `queues`, `realtime`,
  `attachments`.
- **Expõe:** REST (`/conversations`, `/conversations/{id}/messages`); WS
  (`conversation.*`, `message.*`); jobs (entrega outbound).

### `conversationtools`
- **Responsabilidade:** ferramentas de produtividade — **tags**, **respostas
  prontas** (canned responses) e **motivos** de encerramento/categorização.
- **Entidades:** `Tag`, `CannedResponse`, `Reason`.
- **Depende de:** `tenant`, `iam`.
- **Expõe:** REST (`/tags`, `/canned-responses`, `/reasons`).

### `routing`
- **Responsabilidade:** roteamento/distribuição de conversas para
  setor/fila/agente. Regras (por canal, palavra-chave, horário), estratégias
  (round-robin, menos ocupado, manual), reatribuição, balanceamento.
- **Entidades:** `RoutingRule`, `Assignment`.
- **Depende de:** `queues`, `sectors`, `presence`, `businesshours`,
  `conversations`.
- **Expõe:** REST (`/routing/rules`); jobs (`routing.assign`).

---

## Canais e integrações externas

### `channels`
- **Responsabilidade:** abstração de canais (canal genérico **API**, WhatsApp,
  webchat). Configuração de instâncias, normalização inbound, **entrega
  outbound** (POST assinado em HMAC-SHA256 no `outbound_url`), status de saúde,
  mídia.
- **Canal `api` (genérico):** qualquer sistema externo integra por HTTP —
  inbound autenticado por `inbound_token` (header) + assinatura; outbound
  entregue por POST assinado no `outbound_url` (headers `X-Chat-Event`,
  `Timestamp`, `Signature`, `Delivery-Id`); recibos idempotentes por
  `external_message_id`. É o substituto real do antigo adapter mock.
- **Entidades:** `ChannelConnection` (instância/config), `OutboundDelivery`,
  `InboundRecord`.
- **Depende de:** `conversations`, `contacts`, `secrets`, `attachments`,
  `providerhub`.
- **Expõe:** REST (`/channels` CRUD + `/channels/{id}/test`); webhooks de inbound
  (`/inbound/channel/{channel}/messages` e `/delivery-receipts`); jobs
  (`channel.deliver`, `channel.retry`, `channels.health_check`).
- **Adapters:** `infra/channels/{api,whatsapp,webchat}` (HMAC compartilhado em
  `infra/channels/sign`). Sem adapter mock em produção.

### `automation`
- **Responsabilidade:** **integração com o sistema de flow externo já
  existente** (não há flow builder nem chatbot aqui). Dispara execuções no flow,
  recebe **callbacks**, registra **logs/execuções**, correlaciona com a conversa.
- **Entidades:** `AutomationBinding`, `AutomationExecution`, `AutomationLog`.
- **Depende de:** `conversations`, `infra/automation` (HTTP client + callbacks).
- **Expõe:** REST (config de bindings + endpoint de callback); jobs
  (`automation.invoke`, `automation.callback`).
- **Nota:** o flow é externo; aqui só orquestramos chamadas, callbacks e logs.

### `providerhub`
- **Responsabilidade:** cliente da **API smsnet-integrations** (a API
  padronizada). **Consulta sob demanda**: consultar cliente, listar planos,
  dados da empresa, e ações com efeito (liberar acesso, abrir chamado). A cada
  chamada o chat monta `{ botId, <campos da rota>, config: { type, <isp_credentials> } }`
  e envia `x-api-key`; nunca fala direto com IXC/SGP/MK/Voalle.
- **Config (`ProviderIntegrationConfig`):** `smsnet_base_url`, `smsnet_api_key`
  (cifrado), `isp_type` (slug: hubsoft/sgpnet/ixcsoft/…), `isp_credentials`
  (mapa cifrado), `options`, `bot_id`, `enabled`, `timeout_ms`. Mantém o
  `ProviderQueryLog` mínimo (sem `response_body`).
- **Envelope:** `success` → dados normalizados; `not_found` → erro amigável
  "não localizado"; `needs_input` → `ClienteResult{NeedsSelection, Options}` para
  o atendente escolher o contrato (próxima chamada com `idCliente`); `fallback` →
  `integration_unavailable` (não quebra a tela).
- **Depende de:** `infra/providerhub`, `secrets`, `conversations`, `contacts`.
- **Expõe:** REST sob a conversa (`/conversations/{id}/external/*`) + config.
- **Nota:** **não** sincroniza, **não** persiste payload externo, **não** faz
  ingestão em tempo real. `liberacao`/`chamado` são auditadas.

### `copilot`
- **Responsabilidade:** assistente de IA para o agente — **suggest_reply**,
  **summarize**, **classify**, **next_action**. Adapters REAIS por HTTP em
  `infra/copilot/provider`: `openai`, `mistral`, `deepseek`, `perplexity`
  (compatíveis OpenAI, derivados de `openAICompatible`), `anthropic` (Messages
  API) e `gemini` (generateContent). Adapters stateless: a chave/`base_url` por
  tenant viajam na requisição. O `echo` é mock **só para testes** — fora do
  wiring de produção. Suporte ao loop de **tool calling** (ferramentas vêm do
  registry MCP; leitura a IA chama, escrita só PROPÕE).
- **Config por tenant (`AIConfig`):** `provider`, `model`, `temperature`,
  `max_tokens`, `api_key` **cifrada (AES-GCM, nunca retornada — só `has_key`)**,
  `base_url` opcional, `enabled`, gates `allow_*_data`, `human_approval_required`.
- **Privacidade:** os gates `allow_*_data` filtram o contexto **antes** do
  provider (dado bloqueado nunca é enviado); o `AILog` guarda só flags + tokens +
  custo estimado + status, **sem dado bruto**. Falha do provider vira erro
  amigável (não derruba a rota).
- **Entidades:** `AIConfig`, `AILog` (auditável).
- **Depende de:** `conversations`, `infra/copilot/provider`, `infra/secrets`.
- **Expõe:** REST (`/copilot/*`); jobs (`ai.suggest`, `ai.summarize`,
  `ai.classify`); WS (`copilot.suggestion`).
- **Nota:** assíncrono quando a latência permitir; síncrono para sugestão
  interativa curta.

### `mcp`
- **Responsabilidade:** camada genérica para o copiloto e o atendente usarem
  ferramentas de sistemas externos via **MCP (Model Context Protocol)**. Os MCP
  do smsnet-integrations (consultas + operações) e, no futuro, o de
  monitoramento/OLT conectam pela MESMA camada — adicionar um sistema é só
  **registrar o servidor MCP**, sem mexer em código. Nenhum nome de ferramenta é
  hard-coded; tudo é descoberto dinamicamente (`tools/list`).
- **Entidades:** `ServerConnection` (por tenant: `transport=streamable_http`,
  `base_url`, `auth_header`+token **cifrado**, `kind=read|write`, `enabled`),
  `Tool` (descoberta, anotada write/read pelo *kind* do servidor), `Approval`
  (ação write proposta, aguardando confirmação), `CallLog` (mínimo, **sem
  payload sensível** — só quem/onde/tool/status/latência).
- **Cliente:** `infra/mcp` fala Streamable HTTP (JSON-RPC 2.0): `list_tools` e
  `call_tool`. `ToolRegistry` por tenant agrega as ferramentas dos servidores
  habilitados.
- **Copiloto agêntico:** o provider de IA recebe as ferramentas; **read** são
  executadas em loop (chat→MCP→modelo até a resposta final); **write** NUNCA
  executam automaticamente — o modelo só **PROPÕE** e o chat devolve um card de
  confirmação. `human_approval_required` vale para TODA ferramenta write.
- **Atendente:** `GET /v1/conversations/{id}/mcp/tools`;
  `POST /v1/conversations/{id}/mcp/run` (read direto; write vira aprovação
  pendente); `POST /v1/conversations/{id}/copilot/approvals/{id}` aprova/recusa
  (aprovação dispara a execução, **auditada** com actor/ip/params mascarando
  segredos).
- **Config:** `GET/POST/PATCH/DELETE /v1/mcp/servers`,
  `POST /v1/mcp/servers/{id}/test` (lista as tools do servidor).
- **Permissões:** `copilot.use` (IA usar read), `integration.read` (atendente
  rodar read), `integration.execute_action` (qualquer write — IA propõe, humano
  aprova).
- **Depende de:** `conversations` (visibilidade), `copilot` (porta `ToolBroker`),
  `infra/mcp`, `infra/secrets`.

---

## Qualidade de serviço e engajamento

### `sla`
- **Responsabilidade:** acordos de nível de serviço — políticas (primeira
  resposta, resolução), relógios por conversa, alertas de violação/risco.
- **Entidades:** `SlaPolicy`, `SlaTracker`.
- **Depende de:** `conversations`, `businesshours`, `notifications`.
- **Expõe:** REST (`/sla/policies`); jobs (`sla.check` no scheduler); WS
  (`sla.breached`, `sla.warning`).

### `csat`
- **Responsabilidade:** pesquisa de satisfação pós-atendimento. Disparo,
  coleta de respostas, expiração, agregação.
- **Entidades:** `CsatSurvey`, `CsatResponse`.
- **Depende de:** `conversations`, `channels`, `notifications`.
- **Expõe:** REST (`/csat/*`); jobs (`csat.send`, `csat.expire`).

### `businesshours`
- **Responsabilidade:** horários de funcionamento por tenant/setor, feriados,
  fuso. Base para roteamento, SLA e mensagens automáticas fora de expediente.
- **Entidades:** `BusinessHours`, `Holiday`.
- **Depende de:** `tenant`, `sectors`.
- **Expõe:** REST (`/business-hours`); consultado por `sla` e `routing`.

---

## Plataforma

### `webhooks`
- **Responsabilidade:** **webhooks de saída** — assinaturas por evento, entrega
  com **assinatura HMAC**, retries, dead-letter, histórico de entregas.
- **Entidades:** `WebhookSubscription`, `WebhookDelivery`.
- **Depende de:** `infra/webhooks`, eventos de domínio.
- **Expõe:** REST (`/webhooks`); jobs (`webhook.deliver`, `webhook.retry`).

### `notifications`
- **Responsabilidade:** notificações ao operador — in-app (WS), **e-mail**,
  (push opcional). Preferências, templates, dedupe.
- **Entidades:** `Notification`, `NotificationPreference`.
- **Depende de:** `iam`, `infra/email` (SMTP real), `realtime`.
- **Expõe:** REST (`/notifications`); jobs (`notification.send`,
  `notification.email` → envio por SMTP respeitando as preferências do
  usuário); WS (`notification.created`).

### `search`
- **Responsabilidade:** busca de conversas, contatos e mensagens (full-text +
  filtros). Read-model/índices; sem collection própria além de índices Mongo.
- **Entidades:** —
- **Depende de:** `conversations`, `contacts` (índices), Mongo text/Atlas Search.
- **Expõe:** REST (`/search`).
- **Nota:** começar com índices Mongo; evoluir para mecanismo dedicado se
  necessário (ver dúvidas no roadmap).

### `privacy`
- **Responsabilidade:** LGPD/GDPR — anonimização, **retenção** e expurgo,
  **export** de dados do titular, *right to be forgotten*, consentimento.
- **Entidades:** `PrivacyRequest`, `RetentionPolicy`.
- **Depende de:** `contacts`, `conversations`, `attachments`, `audit`.
- **Expõe:** REST (`/privacy/*`); jobs (`privacy.export`, `privacy.erase`,
  retenção via scheduler).

### `audit`
- **Responsabilidade:** trilha de auditoria imutável de ações sensíveis (quem,
  o quê, quando, antes/depois). Compactação/retenção.
- **Entidades:** `AuditLog`.
- **Depende de:** todos (consumidor de eventos).
- **Expõe:** REST (`/audit`, leitura); jobs (`audit.compact` no scheduler).

### `attachments`
- **Responsabilidade:** anexos/mídia — upload, validação (tipo/tamanho/AV),
  armazenamento (S3-compatible/local), thumbnails, URLs assinadas.
- **Entidades:** `Attachment`.
- **Depende de:** `infra/storage`, `conversations`, `messages`.
- **Expõe:** REST (`/attachments`); jobs (`attachment.process`).

### `realtime`
- **Responsabilidade:** infraestrutura de tempo real — **hub** de conexões WS
  por tópico (tenant-scoped) e **fan-out** entre nós via **Redis Pub/Sub**.
- **Entidades:** `Client`, `Topic`, `Message` (efêmeros).
- **Depende de:** `infra/redis`.
- **Expõe:** endpoint WS (`/ws`); API interna `Publish(topic, payload)` usada
  por todos os domínios.

---

## Matriz de dependências (resumo)

```
tenant ◄── iam ◄── auth
   ▲         ▲
   │         ├── sectors ◄── queues ◄── routing ◄── conversations ──► channels ──► providerhub
   │         │                              ▲            │   ▲             │
   │         │        presence ─────────────┘            │   │             └── automation (flow externo)
   │         │        businesshours ──► sla ◄────────────┘   │
   │         │        conversationtools ──────────────────────┤
   │         │        attachments ───────────────────────────►│
contacts ◄───┘        copilot / csat / notifications / webhooks / search / privacy / audit ◄─ eventos
                      realtime ◄── (publish) ── todos
```
