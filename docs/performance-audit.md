# Auditoria de performance e eficiência de I/O — chat-smsnet-omnichannel

> Auditoria de **performance e eficiência de I/O** (não de corretude/segurança —
> essa vive em `docs/audit-report.md`). Foco: padrões que geram requisições
> desnecessárias, N+1 (no cliente e no backend), amplificação de carga e
> varreduras sem índice.
> Metodologia: leitura/grep do código real — a documentação **não** foi tomada
> como verdade; onde diverge, o código prevalece e a divergência virou achado.
> Data da auditoria: 2026-06-13. Branch: `claude/admiring-knuth-2wk8xd`.
>
> **Atualização de remediação (2026-06-13, mesma branch):** os achados **P0/P1**
> foram **corrigidos em código** após a auditoria — ver `## 1b. Status de
> remediação` para os commits. Este doc agora reflete o estado pós-correção; as
> seções de achados originais mantêm a evidência e ganharam marcação
> ✅ **RESOLVIDO (commit)**. Restam apenas **P2 defensivos** (ver `## 1c`).

Severidades: **P0** (catastrófico/escala explosiva imediata) · **P1** (degrada
caminho quente ou escala mal com tenants×entidades) · **P2** (defensivo/médio,
sem impacto agudo hoje).

---

## 1. Veredito executivo

| # | Eixo | Status | Pendência |
|---|------|--------|-----------|
| 1 | Payloads de lista com "id solto" → N+1 no cliente | **OK** ✅ | — |
| 2 | N+1 dentro do backend (Mongo) em caminho de lista/job | **OK** ✅ | — |
| 3 | Assinatura/storage em caminho quente (avatares, presigned) | **OK** | — |
| 4 | Paginação e limites | **OK (índices)** ✅ | paginações **P2** (presence/agents/copilot, ListBySector 1000) |
| 5 | Cache / leituras redundantes | **OK (cache HTTP)** ✅ | Redis read-cache **P2** (documentado) |
| 6 | Tempo real vs polling (sobreposição WS×REST) | **PARCIAL** | contrato anti-polling **P2** |
| 7 | Fan-out multi-tenant em jobs periódicos | **OK (sla)** ✅ | health-check serial **P2** |
| — | Build / vet / test / lint verdes | **OK** | — |

**Resumo (pós-remediação):** todos os achados **P0/P1** foram corrigidos —
os três N+1 de backend (`LastMessages` no inbox, `presence.List`, `sla.RunCheck`)
viraram **agregações/batch em 1 query**; a conversa passou a **embutir
`contact_name`/`agent_name`/`agent_avatar_url`** (mata o N+1 de tradução do
cliente); as **lacunas de índice** (`assigned_to`, `tag`, fechamento inativo)
foram fechadas pela **migração 0028**; e os catálogos quase-estáticos ganharam
**cache HTTP condicional (ETag + 304)**. Restam apenas **P2 defensivos**
(`## 1c`): paginação de algumas listas pequenas, read-cache de Redis, contrato
anti-polling e paralelização do health-check.

---

## 1b. Status de remediação (RESOLVIDO + commit)

| Achado | Sev. | Correção | Commit |
|---|---|---|---|
| **1.1** — Conversa não embute nome do contato/agente | P1 | `ConversationResponse` ganhou `contact_name`/`agent_name`/`agent_avatar_url` (e `contact_avatar_url`), resolvidos em **batch** no controller (ports `ContactDirectory`/`AgentDirectory`); detalhe/create idem | `97cfe53` (avatar base: `a46d412`, `257b579`) |
| **2.1** — `LastMessages` 1 query por conversa | P1 | `MessageRepository.LatestByConversations`: **1 agregação** `$match $in → $sort → $group $first`; `LastMessages` só troca a impl | `97cfe53` |
| **2.2** — `presence.List` 1 count por agente | P0 | `LoadCounter.OpenAssignedLoads`: **1 agregação** `$group $sum` p/ todos os agentes; reusada em `routing.eligibleAgents`; `?sector_id` no servidor | `d24d4dd` |
| **2.3** — `sla.RunCheck` 1 `FindByID` por tracking | P1 | agrupa por tenant + `conversations.FindByIDs($in)` por tenant (novo `FindByIDs`); avaliação inalterada | `d24d4dd` |
| **Índices §4/§9** — `assigned_to`/`tag`/fechamento inativo | P1/P2 | **migração 0028** idempotente: `{tenant,assigned_to,updated_at,_id}`, `{tenant,tags,updated_at,_id}`, `{tenant,status,last_message_at}` | `d24d4dd` |
| **Eixo 5** — zero cache em quase-estáticos | P2 | `middleware.ConditionalCache` (ETag forte = hash do corpo + `Cache-Control: private, max-age=45` + **304**) em `/tags`,`/canned-responses`,`/close-reasons`,`/sectors`,`/queues`,`/me` | `706903d` |

Cada correção tem teste (batch/constant-queries, 304/ETag tenant-scoped, índice).
Build/vet/lint/test seguiram verdes em cada commit.

## 1c. Pendências P2 (defensivas)

Nenhuma é P0/P1; não há amplificação aguda hoje. Não aplicadas.

| # | Pendência | Evidência | Recomendação |
|---|---|---|---|
| P2-a | **Paginação** de `GET /v1/agents/presence`, `/v1/agents`, copilot `tool-calls`/`approvals` (arrays crus) | `presence_controller.go`, `agents_controller.go:98`, `mcp/tool_controller.go:63,74` | cursor keyset; `agents/presence` já filtra por `?sector_id` |
| P2-b | **`ListBySector` com `SetLimit(1000)`** fixo (sem cursor) | `infra/.../iam/user_repository.go:148` | trocar por keyset paginado |
| P2-c | **Read-cache de Redis** com invalidação por escrita p/ catálogos | §7 (só cache HTTP hoje) | evolução documentada em `middleware.ConditionalCache`; opcional |
| P2-d | **Contrato anti-polling** quando o WS está conectado | §8 / `docs/realtime-events.md` | documentar "WS conectado ⇒ não pollar `/conversations`/`/messages`/`/agents/presence`" |
| P2-e | **Health-check serial** (1 HTTP GET bloqueante por conexão) | `channel_service.go:68` → `infra/channels/health.go:39` | worker pool com timeout agregado |
| P2-f | **`tenant.ListActive`** e `search.conversationIDsBySLA` sem `SetLimit` | `tenant_repository.go:61`, `search/index.go:221` | limite defensivo |
| P2-g | **`FindByIDs/$in`** sem `SetLimit` | contacts/attachments/tags/roles/users repos | já limitados a ≤100 ids nos caminhos de lista; limite defensivo opcional |

---

## 2. Saída literal de build / vet / test / lint

```
$ go build ./...
(sem saída — exit 0)

$ go vet ./...
(sem saída — exit 0)

$ golangci-lint run
0 issues.

$ go test ./...
(sem FAIL; sem panic)
pacotes com teste OK : 56
pacotes com FAIL     : 0
```

Constantes de paginação: `DefaultPageSize = 25`, `MaxPageSize = 100`
(`domain/shared/pagination.go:12-13`), aplicadas por `PageRequest.Normalize()`.

---

## 3. Eixo 1 — Payloads que forçam N+1 no cliente

Campos de relacionados expostos **só como id**, que previsivelmente viram uma
chamada extra por item numa lista. O **inbox** (`GET /v1/conversations`) é o pior
caso porque renderiza nome/avatar do contato e do agente por linha.

| Campo (DTO) | Arquivo:linha | Resolvido em batch na lista? | Veredito |
|---|---|---|---|
| `Conversation.contact_id` (sem `contact_name`) | `presenter/contracts/conversations/dto.go:16` | **Não** — só `contact_avatar_url` é resolvido (dto.go:25) | **N+1 no cliente p/ o NOME do contato** |
| `Conversation.assigned_to` (sem nome/avatar do agente) | `dto.go:20` | **Não** | **N+1 no cliente p/ o agente** |
| `Conversation.sector_id` / `queue_id` (sem rótulo) | `dto.go:18-19` | **Não** | N+1 ou resolvido via cache de setores/filas |
| `Conversation.tags` (ids, sem nome/cor) | `dto.go:22` | **Não** (por contrato — front resolve via `GET /v1/tags`) | aceitável **se** `/tags` for cacheado (ver Eixo 5) |
| `Contact.tags` (ids, sem nome/cor) | `presenter/contracts/contacts/dto.go:96` | **Não** (por contrato) | idem |
| `User.role_ids` / `sector_ids` (ids) | `presenter/contracts/iam/dto.go:67-68` | **Não** (front resolve via `/roles`, `/sectors`) | aceitável se cacheado |
| `Message.sender_id` (sem nome do agente) | `dto.go:263` | **Não** | menor (poucos agentes; cache de `/agents`) |

**Já resolvido (bom):** `Contact.avatar_url`
(`presenter/contracts/contacts/dto.go:101`) e `Conversation.contact_avatar_url`
(`dto.go:25`) vêm assinados e em batch; `User.avatar_url` e
`AssignableAgent.avatar_url` idem. A hidratação de anexos de mensagem
(`url/content_type/filename/size`) também vem resolvida.

**Achado 1.1 (P1) — Conversa não embute nome do contato nem do agente.** ✅ **RESOLVIDO (`97cfe53`).**
`ConversationResponse` (`presenter/contracts/conversations/dto.go:14-26`) tem
`contact_id` e `assigned_to` mas **nenhum** `contact_name`/`agent_name`. O inbox
precisa desses rótulos por linha, então o front faz uma busca por contato e por
agente por conversa visível → N+1 multiplicado pelo tamanho da página.
**Recomendação (não aplicada):** embutir `contact_name` (e opcionalmente
`agent_name`/`agent_avatar_url`) na `ConversationResponse`, resolvidos em batch no
controller (mesmo padrão de `LastMessages`/`ContactAvatarURLs` já presente em
`conversation_controller.go:48-55`). Para `tags`, manter ids mas garantir que
`GET /v1/tags` seja cacheável (Eixo 5).

---

## 4. Eixo 2 — N+1 dentro do backend (Mongo)

**Achado 2.1 (P1) — `LastMessages` faz 1 query por conversa no inbox.** ✅ **RESOLVIDO (`97cfe53`).**
`domain/conversations/service/conversation_service.go:274-289`:

```go
func (s *Service) LastMessages(ctx, conversationIDs []string) (...) {
    for _, id := range conversationIDs {
        m, err := s.messages.LatestByConversation(ctx, id) // 1 FindOne por conversa
        ...
    }
}
```

Chamado em `presenter/controller/conversations/conversation_controller.go:48`
para **toda** página do inbox. Uma página de 100 conversas dispara **100 queries**
adicionais só para os previews. O inbox por página hoje custa:
`conversations.List` (1) + `LastMessages` (**N**) + `contacts.FindByIDs` (1, batch)
+ `attachments.FindByIDs` (1, batch). O termo **N** domina.
**Recomendação (não aplicada):** substituir o loop por **uma** agregação
`$match {tenant_id, conversation_id: {$in}}, $sort {created_at desc}, $group {_id:$conversation_id, doc:{$first}}`
(ou `$lookup` no list de conversas), retornando o mapa em 1 round-trip. O índice
`tenant_conversation_created` (`migrations/0004:41-42`) já cobre o `$sort`.

**Achado 2.2 (P0) — `presence.List` faz 1 query Mongo por agente.** ✅ **RESOLVIDO (`d24d4dd`).**
`domain/presence/service/presence_service.go:112-126`:

```go
items, _ := s.store.List(ctx)            // N agentes do Redis (N HGetAll — store.go:70)
for _, p := range items {
    if load, err := s.load.CountOpenAssigned(ctx, p.UserID); err == nil { // 1 count Mongo/agente
        p.CurrentLoad = load
    }
}
```

`GET /v1/agents/presence` (sidebar do inbox, alta frequência) =
**N HGetAll no Redis + N counts no Mongo**, sem paginação
(`presenter/controller/presence/presence_controller.go:24-33` devolve
`{"data": ...}` cru). O mesmo `CountOpenAssigned`-por-agente aparece no roteamento
(`domain/routing/service/routing_service.go` em `eligibleAgents`).
**Recomendação (não aplicada):** um único `aggregate` em `messages`/`conversations`
agrupando `open assigned` por `assigned_to` (`$group`), devolvendo o load de todos
os agentes em 1 query; opcionalmente paginar/filtrar a presença por setor no
servidor.

**Achado 2.3 (P1) — `sla.RunCheck` faz 1 `FindByID` por tracking, a cada minuto.** ✅ **RESOLVIDO (`d24d4dd`).**
`domain/sla/service/sla_service.go:172-186` busca até `checkBatchLimit` (1000)
trackings em 1 query (`tracking.ListRunningAcrossTenants`) e então:

```go
for _, t := range items { s.evaluate(ctx, t, now) }
// evaluate -> conv, _ := s.conversations.FindByID(tctx, t.ConversationID)  // sla_service.go:186
```

Job cron `* * * * *` (minuto): até **1000 `FindByID` serial por execução**.
**Recomendação (não aplicada):** agrupar `ConversationID` por tenant e usar
`conversations.FindByIDs($in)` por tenant (já existe `FindByIDs` no padrão);
hidratar o lote em poucas queries em vez de 1k.

**Índices — lacunas em filtros de lista de conversas.** O `List`
(`infra/.../conversations/conversation_repository.go:146-176`) **sempre** ordena
por `updated_at desc, _id desc`, mas combina filtros arbitrários
(`status/sector_id/queue_id/assigned_to/contact_id/tag` + visibilidade `$or`).
Índices existentes (`migrations/0004:18-36`, `0026`):

| Filtro de lista | Índice que cobre filtro **e** ordenação `updated_at` | Severidade |
|---|---|---|
| nenhum (inbox base) | `tenant_updated_keyset` ✅ | — |
| `status`+`sector_id` | `tenant_status_sector_updated` ✅ | — |
| `contact_id` | `tenant_contact_updated_keyset` (0026) ✅ | — |
| **`assigned_to`** (aba "Minhas", quente) | `tenant_assignee_status` **não tem `updated_at`** → **sort em memória** | **P1** |
| **`tag`** | sem índice `{tenant, tags, updated_at}` (contatos ganharam em 0027; conversas não) | **P2** |
| **`queue_id`** | sem índice com `queue_id` → varredura no tenant | **P2** |
| visibilidade `$or {assigned_to, sector_id:$in}` + sort | difícil de cobrir; merge/sort em memória p/ agentes não-admin | **P1** |

`data-model.md` afirma "todo filtro de lista tem índice que o cobre"; o código
**diverge** nesses casos. **Recomendação (não aplicada):** índices
`{tenant_id, assigned_to, updated_at desc, _id desc}` e
`{tenant_id, tags, updated_at desc, _id desc}`.

**Nota (não é N+1):** os `FindByIDs`/`$in`
(`contacts:89`, `attachments:113`, `conversationtools/tag_repository.go:123`,
`iam/role_repository.go:97`) **não** têm `SetLimit`, mas nos caminhos de lista são
alimentados por ≤`MaxPageSize` (100) ids — **limitados na prática**. Severidade
**P2** (limite defensivo seria bom, não há amplificação real hoje).

---

## 5. Eixo 3 — Assinatura/storage em caminho quente

**OK (melhoria recente).** A geração de `avatar_url`
(`domain/attachments/service/service.go` `SignedAvatarURLs`) é **HMAC local puro**
(reusa o token `/v1/channel-media/{token}`), **sem IO por item**: uma query
`FindByIDs` por página + assinatura em memória. O inbox e as listas de
contatos/usuários/agentes resolvem avatares **em batch** sem `HeadObject` nem
presigned por item. O download JWT-gated `/v1/attachments/{id}/download` **deixou
de ser** o caminho de avatar em massa (era o amplificador anterior).

Ressalva **P2:** `contact_avatar_url` no inbox adiciona 2 queries batch por página
(`contacts.FindByIDs` + `attachments.FindByIDs`) — barato e aceitável, mas some
no overhead só quando o Achado 2.1 (LastMessages N+1) for resolvido. Presigned S3
(`infra/storage/attachments_s3.go`) é gerado sob demanda 1×/objeto na rota de
download (não em loop de lista) — OK.

---

## 6. Eixo 4 — Paginação e limites

**PARCIAL.** ~27 endpoints de lista usam keyset com default 25 / máx 100
(`shared.NewPage` + `PageRequest.Normalize`). Exceções (arrays crus sem
paginação):

| Endpoint | Arquivo:linha | Limite | Severidade |
|---|---|---|---|
| `GET /v1/agents/presence` | `presenter/controller/presence/presence_controller.go:24` (`{"data": ...}`) | **sem paginação** (todo o time) + N+1 (2.2) | **P1** |
| `GET /v1/agents` | `presenter/controller/agents/agents_controller.go:98` (`{"data": out}`) | `users.List(MaxPageSize=100)` + `presence.List` cru | **P2** |
| `GET /v1/agents?sector_id=` | `infra/.../iam/user_repository.go:148` `ListBySector` | **`SetLimit(1000)`** fixo (10×) sem cursor | **P2** |
| `GET /v1/conversations/{id}/mcp/tools` | `presenter/controller/mcp/tool_controller.go:26` | sem paginação (por conversa) | **P2** |
| `GET /v1/conversations/{id}/copilot/tool-calls` | `tool_controller.go:63` | sem paginação (cresce com atividade) | **P2** |
| `GET /v1/conversations/{id}/copilot/approvals` | `tool_controller.go:74` | sem paginação | **P2** |
| `GET /v1/providerhub/...` (listas) | `presenter/controller/providerhub/providerhub_controller.go` | `{"data": ...}` cru | **P2** |

**Varreduras sem `SetLimit` no backend:**
- `tenant/tenant_repository.go:61` `ListActive` — sem limite; alimenta o fan-out
  dos jobs (Eixo 7). Cresce com nº de tenants. **P2.**
- `search/index.go:221` `conversationIDsBySLA` — `Find` com projeção **sem
  `SetLimit`**, devolve **todos** os `conversation_id` num status de SLA e usa como
  `$in` na query seguinte. Com muitas conversas em risco, o `$in` explode. **P2.**

Cursor keyset é consistente (sem `skip`/offset caro detectado). **Recomendação
(não aplicada):** paginar `presence`/`agents`/copilot-logs; trocar
`ListBySector(1000)` por cursor; pôr `SetLimit` defensivo em `ListActive` e
`conversationIDsBySLA`.

---

## 7. Eixo 5 — Cache / leituras redundantes  ✅ **cache HTTP RESOLVIDO (`706903d`); Redis read-cache pendente (P2-c)**

**GAP.** Não há cache HTTP nem read-cache de Redis em nenhuma leitura de domínio.

- **HTTP caching:** o único `Cache-Control` do projeto está no spec OpenAPI
  (`presenter/openapi/handler.go:56`, `public, max-age=300`). **Nenhum** `ETag`,
  `Last-Modified` ou `If-None-Match` em qualquer endpoint REST de dados (grep em
  `presenter/` retorna só o handler do spec). Toda navegação re-busca tudo.
- **Redis:** usado só para **locks**, **pub/sub** (realtime), **broker Asynq**,
  **rate-limit** do providerhub e **store de presença** — **não** há read-through
  cache de Mongo (`infra/redis/`).
- **Quase-estáticos sempre batem no Mongo:** `sectors` (`sector_service.go:110`),
  `queues` (`queue_service.go:187`), `channels` (`channel_service.go:223`), além de
  `tags`/`canned-responses`/`close-reasons` e `GET /v1/me`
  (`auth_controller.go:88` faz `users.Get` direto). Esses são exatamente os dados
  que o front (Eixo 1) precisa re-resolver para traduzir ids→nomes.
- **Alta frequência sem cache:** inbox (`GET /v1/conversations`) e
  `GET /v1/agents/presence` re-computam a cada request, sem `ETag`/`max-age`.

**Recomendação (não aplicada, sem prescrever solução de front):** expor `ETag`
(hash do conteúdo) + `Cache-Control: private, max-age` curto nos quase-estáticos
(`/tags`, `/sectors`, `/queues`, `/close-reasons`, `/canned-responses`, `/me`),
permitindo `304 Not Modified` e cortando refetch em cada navegação; opcionalmente
um read-cache de Redis com invalidação por escrita para esses catálogos.

---

## 8. Eixo 6 — Tempo real vs polling

**PARCIAL.** O WS já entrega o delta que o REST recomputaria:
`conversation.updated`/`message.created`/`queue.stats`/`agent.presence_changed`
(catálogo em `docs/realtime-events.md:92-213`). O doc orienta corretamente
reconciliar via REST **na (re)conexão** com `cursor`/`last_message_at`
(`realtime-events.md:227-236`) — isso é certo. **Lacuna:** não há orientação
explícita contra **polling simultâneo** enquanto o WS está conectado. Um cliente
que assina `inbox:{sector}` para `conversation.updated` **e** também faz
`GET /v1/conversations` periódico gera leituras duplicadas do mesmo dado —
amplificado pelo Achado 2.1 (cada refetch do inbox = 1 + N queries).
**Recomendação (não aplicada):** documentar o contrato "WS conectado ⇒ não pollar
`/conversations`/`/messages`/`/agents/presence`; só refetch no reconnect via
cursor"; expor `queue.stats`/presença preferencialmente via WS.

---

## 9. Eixo 7 — Amplificação multi-tenant / fan-out (jobs)

**PARCIAL.** O helper `eachTenant` (`app/start_routines/bootstrap_workers.go:232`)
itera **todos** os tenants ativos (`tenant.ListActive`, sem limite) e chama a
operação por tenant. A maioria dos jobs é batch dentro do tenant (OK); dois
escalam mal:

| Job (cron) | Padrão | Escala | Sev. |
|---|---|---|---|
| `sla.check` (`* * * * *`) | global 1000 trackings + **`FindByID` por tracking** (Achado 2.3) | O(trackings) serial/minuto | **P1** |
| `chat.close_inactive` (`*/5`) | `eachTenant` → `ListInactiveOpen` filtra `{tenant_id, status:$nin, last_message_at:$lte}` ordenado por `last_message_at` — **sem índice composto** (`conversation_repository.go:106-119`; `migrations/0004` não cobre `last_message_at`) | O(tenants) + varredura por tenant | **P2** |
| `channels.health_check` (`*/5`) | `eachTenant` → loop de conexões com **1 HTTP GET bloqueante por conexão** (`channel_service.go:68` → `infra/channels/health.go:39`, timeout 5s) | O(tenants × conexões) serial | **P2** |
| `notifications.cleanup` / `audit.compact` / `privacy.retention` | `eachTenant` → delete em batch por coleção | O(tenants) + deletes em lote | OK |
| `reports.snapshot` | `eachTenant` → 1 agregação/upsert por tenant | O(tenants) | OK |

**Recomendações (não aplicadas):** (a) `sla.check` em batch por tenant via `$in`;
(b) índice `{tenant_id, status, last_message_at}` para `ListInactiveOpen`;
(c) paralelizar/limitar concorrência dos health-checks HTTP (worker pool com
timeout agregado) em vez de serial; (d) `SetLimit` em `ListActive`.

---

## 10. Tabela — endpoints de lista × batch de relacionados × paginado × cacheável

| Endpoint | Resolve relacionados em batch? | Paginado (25/100)? | Cacheável hoje? |
|---|---|---|---|
| `GET /v1/conversations` | **Parcial** (avatar do contato sim; **nome do contato/agente não**) + **N+1 de preview** | Sim | Não (sem ETag) |
| `GET /v1/conversations/{id}/messages` | **Sim** (anexos hidratados em batch) | Sim | Não |
| `GET /v1/contacts` | **Sim** (`avatar_url` em batch); tags ids | Sim | Não |
| `GET /v1/users` | **Sim** (`avatar_url`); roles/sectors ids | Sim | Não |
| `GET /v1/agents` | **Sim** (`avatar_url`) | **Não** (array cru, cap 100/1000) | Não |
| `GET /v1/agents/presence` | n/a | **Não** + **N+1 de load** | Não |
| `GET /v1/tags` `/sectors` `/queues` `/close-reasons` `/canned-responses` | n/a | Sim | **Não** (quase-estático, deveria) |
| `GET /v1/me` | `avatar_url` sim; roles/sectors ids | n/a | **Não** (deveria) |
| `GET /v1/sla/at-risk` `/notifications` `/audit` `/webhooks` `/automation/*` `/csat/*` | n/a | Sim | Não |
| `GET /v1/conversations/{id}/copilot/{tool-calls,approvals}` | n/a | **Não** | Não |

---

## 11. Top 5 ofensores de requisição (em escala)

1. **`LastMessages` N+1 no inbox** (`conversation_service.go:274-289`, chamado em
   `conversation_controller.go:48`). Cada listagem de conversas dispara
   **1 query por conversa** para o preview. → *agregação `$group $first` em 1
   round-trip.* **(P1)**
2. **Conversa sem nome do contato/agente** (`conversations/dto.go:16,20`). Força o
   front a N+1 por linha do inbox para traduzir `contact_id`/`assigned_to`. →
   *embutir `contact_name` (e `agent_name`/`agent_avatar_url`), resolvidos em batch
   no controller.* **(P1)**
3. **`presence.List` N+1 + sem paginação** (`presence_service.go:112-126`;
   `presence_controller.go:24`). Sidebar de agentes faz N HGetAll + N counts Mongo
   por request. → *1 agregação de load por `assigned_to`; paginar/filtrar por
   setor.* **(P1)**
4. **`sla.check` N+1 por minuto** (`sla_service.go:173-186`). Até 1000 `FindByID`
   seriais/minuto. → *`FindByIDs($in)` por tenant.* **(P1)**
5. **Zero cache em quase-estáticos** (`tags/sectors/queues/close-reasons/me`,
   §7). Cada navegação re-busca os catálogos que o front usa para traduzir os
   "ids soltos" do Eixo 1. → *`ETag` + `Cache-Control` curto → `304`.* **(P2,
   mas multiplicador de todo o Eixo 1)**

---

## 12. O que já está bom (não-achados)

- Keyset/cursor consistente, default 25 / máx 100, em ~27 endpoints.
- Hidratação de anexos de mensagem **em batch** (`attachments.FindByIDs` `$in`).
- `avatar_url`/`contact_avatar_url` **resolvidos e assinados em batch** (HMAC
  local, sem IO por item) — N+1 de avatar **eliminado**.
- Jobs de limpeza/retenção/relatório usam deletes/agregações em lote por tenant.
- Multi-tenant correto em todas as queries de lista (`tenant_id` sempre no filtro).
</content>
