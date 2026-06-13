# Roadmap de MVP

Sequência de implementação priorizando o **caminho crítico do atendimento em
tempo real** e adicionando plataforma/QoS em camadas. Cada fase é entregável e
testável. A **fundação** (camadas, config, container, start_routines por papel,
padrões transversais, Asynq/Redis/Mongo bootstrap, health) já está implementada.

> Legenda de domínios por fase. Cada domínio segue o molde
> `contracts/entity/repository/service` + impl Mongo + controller + rotas +
> (jobs/eventos quando aplicável).

## Fase 0 — Fundação ✅ (feita)
- Camadas, `RUN_ROLE`, config, DI, server, health.
- Padrões: cursor pagination, error envelope, idempotency, request_id, rate
  limit, tenant scope.
- Bootstrap: Mongo, migrations/índices, seed, Asynq worker/scheduler, realtime
  (hub + pub/sub).

## Fase 1 — Identidade e acesso
**Domínios:** `tenant`, `auth`, `iam`.
- Tenant + settings; seed de owner/papéis/permissões.
- Login/refresh/logout, sessões, API keys.
- Usuários, papéis, permissões, `Authorizer` real ligado ao middleware `authz`.
- **Entregável:** autenticar e autorizar; base multi-tenant funcional.

## Fase 2 — Núcleo de atendimento (tempo real)
**Domínios:** `contacts`, `sectors`, `queues`, `conversations`, `realtime`,
`presence`.
- Contatos + identidades por canal.
- Setores/filas + membership.
- Conversas + mensagens + ciclo de vida + atribuição manual.
- Eventos WS (`conversation.*`, `message.*`, `typing`, `presence.changed`).
- Presença Redis + heartbeat.
- **Entregável:** atender uma conversa fim a fim **com um canal mock**, em tempo
  real, com inbox e atribuição manual.

## Fase 3 — Canais reais e roteamento
**Domínios:** `channels`, `routing`, `conversationtools`, `businesshours`.
- Adapter de 1 canal real (decidir WhatsApp **ou** Telegram primeiro) + webchat.
- Inbound (webhook assinado) + outbound (job `channel.deliver`/retry) + status.
- Regras de roteamento + estratégias de distribuição (`routing.assign`).
- Tags, respostas prontas, motivos; horários de funcionamento.
- **Entregável:** receber/enviar por canal real; distribuir automaticamente.

## Fase 4 — Anexos e produtividade
**Domínios:** `attachments`, `notifications`, `search`.
- Upload/validação/AV/thumbnail (`attachment.process`) + storage.
- Notificações in-app (WS) + e-mail.
- Busca de conversas/contatos/mensagens (índices Mongo).
- **Entregável:** mídia em conversas; avisos ao agente; busca.

## Fase 5 — QoS e engajamento
**Domínios:** `sla`, `csat`.
- Políticas de SLA + relógios + `sla.check` + eventos `sla.*`.
- CSAT (`csat.send`/`expire`) + coleta + agregação.
- **Entregável:** SLA monitorado; satisfação coletada.

## Fase 6 — Integrações externas
**Domínios:** `automation`, `providerhub`, `copilot`.
- `automation`: invoke + callbacks + logs do **flow externo**.
- `providerhub`: consulta/ação sob demanda à API **smsnet-integrations**
  (consultar cliente, planos, empresa; liberar acesso; abrir chamado) —
  sem persistir payload. `monitoring` foi removido.
- `copilot`: adapters de IA REAIS (openai/anthropic/gemini/mistral/deepseek/
  perplexity), chave por tenant cifrada, gates de privacidade; echo só em testes.
- **Entregável:** automações externas, consultas sob demanda e copiloto.

## Fase 7 — Plataforma e conformidade
**Domínios:** `webhooks`, `audit`, `privacy`.
- Webhooks de saída (HMAC, retries, dead-letter, histórico).
- Trilha de auditoria + `audit.compact`.
- LGPD: export/erase + retenção (`privacy.*`).
- **Entregável:** extensibilidade externa + conformidade.

## Cortes de escopo do MVP (deliberados)
- Sem chatbot/flow builder próprios (flow é externo, via `automation`).
- `search` com índices Mongo (não mecanismo dedicado) no MVP.
- Push mobile fica para depois (notifications cobre in-app + e-mail).
- Relatórios = `reports.snapshot` simples; BI dedicado fora do MVP.

## Backlog pós-piloto

- **Merge de contatos** (`POST /v1/contacts/{id}/merge`): empresas importam muitos
  duplicados, então merge será necessário — mas só DEPOIS do piloto. Quando for a
  hora, especificar: unir `phones`/`tags`/`external_ids` (sem duplicar), reapontar
  as conversas do contato source para o target, soft-delete do source, e operação
  **transacional e idempotente** (re-merge não duplica nem perde dados). Por ora a
  rota NÃO existe.

---

## Dúvidas que BLOQUEIAM implementação

Itens que mudam contratos/modelo e precisam de decisão antes de codar o domínio
correspondente:

1. **Canais prioritários e provedores.** Qual canal real primeiro (WhatsApp via
   qual BSP/Cloud API? Telegram? webchat próprio?). Define o contrato do
   `channels` adapter, webhooks inbound e modelo de mídia/templates.
2. **Contrato do flow externo (`automation`).** Endpoints, autenticação,
   formato de invoke, **callbacks** (URLs, assinatura HMAC, payload) e o que
   exatamente logar. Sem isso o `automation` não fecha.
3. **Contrato do `providerhub` (API smsnet-integrations).** Resolvido: corpo
   `{ botId, <campos>, config: { type, <isp_credentials> } }` + `x-api-key`;
   envelope `success|not_found|needs_input|fallback`.
4. **Copilot — provider e modo.** Qual provider no MVP (OpenAI/Gemini/Anthropic
   ou só echo/mock)? Sugestão **síncrona** (latência interativa) vs.
   **assíncrona** (job `ai.*`)? Onde ficam as chaves (tenant ou global)?
5. **Estratégia de tokens JWT.** HS256 (segredo único) vs. RS256 (par de
   chaves/rotação)? TTLs de access/refresh? Logout = revogação de sessão server-
   side (lista no Mongo/Redis) confirmada?
6. **Leitura/`unread` de mensagens.** Por-participante (read receipts) vs.
   `unread_count` simples? Multi-agente numa conversa muda o cálculo.
7. **Regra de “conversa inativa”.** Tempo e condições de auto-close
   (`chat.close_inactive_conversations`) — por tenant? por canal?
8. **SLA — base de tempo.** Considerar `businesshours` (pausar relógio fora de
   expediente) já no MVP ou só 24/7? Metas (first response/resolution) default?
9. **Retenção/LGPD.** Períodos padrão de retenção por entidade e definição de
    “anonimização” aceitável (o que apaga vs. o que mantém para métricas).

## Decisões pragmáticas já assumidas (não bloqueiam)

- **IDs** UUIDv4 string; **base** com `tenant_id/created_at/updated_at`.
- **Paginação** keyset; **erros** no envelope padrão; **idempotência** via
  `Idempotency-Key`.
- **Presence** Redis-only; **realtime** via Redis Pub/Sub + Hub.
- **Permissões** como catálogo em código (seed); `roles.permissions` por string.
- **Eventos do sistema** aparecem na timeline (`message.type=event`) **e** em
  `audit_log`.
- **Search** começa em índices Mongo (text) com caminho de evolução.
- **Canal genérico `api`** (estilo Chatwoot API channel): qualquer sistema
  externo integra por HTTP (inbound com `inbound_token` + assinatura; outbound
  por POST assinado HMAC-SHA256 no `outbound_url`). Substituiu o antigo adapter
  mock; WhatsApp/Telegram entram depois como outros adapters da mesma interface.
- **Copilot echo(mock)** primeiro, para destravar a Fase 6 sem provider final.
- **Scheduler** singleton + handlers idempotentes.
