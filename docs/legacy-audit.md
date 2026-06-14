# Legacy Audit — pós-refatoração (Fases 0–3 + PR #1/#3)

**Data:** 2026-06-14 · **Escopo:** auditoria de LEITURA (nenhuma alteração de código aplicada).
**Premissa:** sistema NÃO está em produção; o banco será ZERADO. Objetivo: zero legado em
código, banco, docs e seed.

Convenção de severidade: **HIGH** = legado ativo/quebra ou caminho morto claro · **MED** =
resíduo provável / cobertura ausente · **LOW** = cosmético.

Legenda de recomendação: **Remover** · **Atualizar** · **Simplificar** · **Manter (ok)**.

> Ordem de aplicação sugerida pelo dono: (1) código morto → (2) docs → (3) seed, cada commit verde.

---

## 1) Código morto / órfão

### 1.1 ProviderHub — `ConfigRepository` (singleton legado) órfã — **HIGH** → Remover
Resíduo do PR #1 (config singleton → perfis). A interface e a implementação do repositório
singleton (coleção `providerhub_configs`) não são mais consumidas por nenhum service/factory.

- Interface: `domain/providerhub/repository/repository.go:12-20` (`ConfigRepository`). *(o
  `ProfileRepository` no mesmo arquivo é vivo.)*
- Impl (arquivo inteiro, ~220 linhas): `infra/database/mongodb/repositories/providerhub/config_repository.go`
- Modelo BSON de storage: `infra/database/mongodb/models/providerhub_models.go:7` (`ProviderIntegrationConfig`)
  e `:21` (`ProviderConfigOptions`) — usados **só** por `config_repository.go:162`.
- Única construção viva: `app/start_routines/seed_demo.go:509` (`phRepo := phrepo.NewConfigRepository(...)`)
  — seed legado (ver §5.3). A migração `0030` lê a coleção para converter (ver §2).
- Factory só liga `ProfileRepository`: `app/factories/providerhub.go` (nenhuma ref a `ConfigRepository`).

**Distinção importante (NÃO remover):** a *entity* `domain/providerhub/entity/config.go`
`ProviderIntegrationConfig` continua VIVA — é a forma de chamada do gateway, construída em
runtime a partir de um `ISPProfile` por `resolver.go:110 buildCallConfig(...)` e consumida por
`infra/providerhub/http_gateway.go`. Só o *repositório/coleção/model de storage* é morto.

**Recomendação:** remover a interface `ConfigRepository`, `config_repository.go`, os structs
`ProviderIntegrationConfig`/`ProviderConfigOptions` de `providerhub_models.go`, o teste vivo
associado (§3.2) e a criação no seed (§5.3). Em seguida a migração `0030` vira no-op (§2).

### 1.2 `FindEnabledByType` — só sobrevive como fallback sem `channel_id` — **MED** → Simplificar (decisão)
Após a Fase 0 o outbound resolve por `conv.ChannelID`. O método antigo por TIPO continua vivo
apenas no ramo de fallback para conversas sem `channel_id`:

- Interface: `domain/channels/repository/repository.go:18-20`
- Impl: `infra/database/mongodb/repositories/channels/connection_repository.go:105`
- Único chamador: `domain/channels/service/outbound_service.go:118` (ramo `else` de `conv.ChannelID == ""`)

Como a meta é "sem legado" e toda conversa deve carregar `channel_id` (depois de corrigir o seed,
§5.2), esse `else` é um caminho de compatibilidade legado. **Não é morto** (tem 1 chamador), por
isso é decisão de produto: manter como rede de segurança defensiva, ou remover o `else` +
`FindEnabledByType` (interface, impl e os 3 fakes de teste) e tratar `channel_id` vazio como
"sem conexão utilizável". Recomendo remover para fechar o legado.

### 1.3 `FindByChannelID` — nome de parâmetro residual em fake de teste — **LOW** → Atualizar
A impl real está correta (`assistant_repository.go:108`, param `channelID`). Só o fake de teste
ainda usa o nome da era "channel type":
- `domain/copilot/service/isp_bridge_test.go:42` (`func (r *fakeAssistantRepo) FindByChannelID(_ context.Context, ct string)`).

**Recomendação:** renomear `ct` → `channelID` no fake (cosmético).

### 1.4 Verificados e LIMPOS (sem resíduo) — sem ação
- **business_hours no Sector:** `domain/sectors/entity/sector.go` sem o campo; sem refs residuais
  em models/DTO/service de sector. (Resíduo só intencional na migração `0036`.)
- **Holiday sector-scope:** `ScopeAllSectors`/`ScopeSectors`/`SectorIDs` não existem mais em código
  (só strings de conversão na migração `0037:23,28` — esperado).
- **CopilotAssistant `ChannelTypes`:** não há `ChannelTypes`/`channel_types` em código (só nos
  índices das migrações `0031`/`0035` — ver §2).
- **`FindOpenByContactChannelID`** (renomeado): vivo — `conversation_repository.go:111` ←
  `inbound_service.go:188`.
- **Domínio `automation` (flow externo):** VIVO e distinto de `automationrules` — ligado em
  `app/factories/automation.go`, `app/start_routines/bootstrap_workers.go:59,80`,
  `presenter/controller/automation/automation_controller.go`. Não é dead code.

### Sobre o linter `unused`
O `unused` (staticcheck) só pega símbolos não exportados sem uso **dentro do pacote**; ele NÃO
acusa exportados sem chamador cross-package — exatamente o caso de §1.1/§1.2 (interface+impl
exportadas, "usadas" pela assinatura `var _ repository.ConfigRepository = ...` em
`config_repository.go:219` e pelo seed). Por isso a varredura de chamadores acima foi manual.

---

## 2) Migrações — coerência para banco ZERADO (até 0037)

O runner (`infra/database/mongodb/migrations/migrations.go:50`) ordena por `Version` e aplica as
não-aplicadas; versões devem ser **únicas e estritamente crescentes**, *não* contíguas. Como o
banco (e o ledger `schema_migrations`) será zerado, dá para **renumerar/colapsar livremente**.

**Migrações de conversão de dados = no-op em banco novo** (não dependem de dado legado que não
existirá; podem ser removidas/fundidas no baseline):

| Migração | O que faz | Em banco zerado | Recomendação |
|---|---|---|---|
| `0024_normalize_tag_ids` | troca NOMES de tag por IDs em conversations/contacts (corrige seed antigo) | nada a converter | Remover (ou corrigir seed e dropar) |
| `0025_clean_user_sector_ids` | limpa `sector_ids:[""]`/null e re-liga agentes demo (corrige bug de ordem do seed) | nada a limpar | Remover (corrigir ordem do seed em vez disso) |
| `0030_migrate_single_config_to_profile` | `providerhub_configs` → `isp_profiles` default | sem configs legadas | Remover (junto de §1.1/§5.3) |
| `0036_drop_sector_business_hours` | `$unset business_hours` em sectors | nenhum sector com o campo | Remover (e parar o seed de gravá-lo, §5.1) |
| `0037_holiday_scope_sector_to_channel` | `all_sectors→all_channels`, deleta `sectors`, `$unset sector_ids` | nenhum holiday legado | Remover |

**Churn de índice (cria-e-dropa inútil em banco novo) — Simplificar:**
- `0031_copilot_assistants_indexes.go:26-27` cria o índice `tenant_channel_type` sobre o campo
  **legado `channel_types`**; `0035_copilot_assistants_channel_ids_index.go:21-28` dropa esse
  índice e cria o de `channel_ids`. Em banco zerado isso cria um índice sobre um campo que nunca
  existirá e logo o remove.
  **Recomendação:** fazer `0031` indexar `channel_ids` direto e **remover `0035`** (ou manter
  `0035` como único criador e tirar o `channel_types` de `0031`).

**Manter (índices de baseline, sempre necessários):** todas as `*_indexes.go` (0001–0023, 0026–0029,
0032–0034 e a parte de criação de 0035), `0023_channel_inbound_token_hash`, `0034_conversations_channel_id_index`.

**Nota de ordenação:** se as migrações de conversão forem mantidas, são no-ops idempotentes baratos
(coleções vazias) — manter é inofensivo. A recomendação de remoção vale **só** porque o dono pediu
baseline limpo de sistema nunca implantado. Se remover, **renumerar** para manter a sequência limpa
(ex.: baseline 0001.. sem buracos), já que o ledger será recriado.

---

## 3) Testes

### 3.1 Gap fechado ✓ — sem ação
`TestStatus_SectorScopedHoliday` (dropado na Fase 2) foi **restaurado** como
`TestStatus_ChannelScopedHoliday` na Fase 3 — `domain/businesshours/service/businesshours_service_test.go:224`
(escopo agora por channel: fecha só o channel listado). Cobertura recomposta.

### 3.2 Teste de modelo legado — **HIGH** → Remover (junto de §1.1)
`infra/database/mongodb/repositories/providerhub/config_live_test.go:35`
(`TestConfigEncryptionRoundTripLive`) exercita `entity.ProviderIntegrationConfig` round-trip pela
coleção morta `providerhub_configs` (`:45,:85`). Atrelado ao repositório singleton morto — remover
junto com §1.1.

### 3.3 Gaps de cobertura criados pelas trocas
- **MED** — `business_hours` no write-path do channel **sem teste de service**: a validação
  `bhentity.ValidateSchedule` em `domain/channels/service/channel_service.go` (Create/Update) não é
  exercida em `domain/channels/service/channel_service_test.go` (grep: zero refs a `business_hours`/
  `ValidateSchedule`). O parser/entity está bem coberto (`schedule_test.go`, incl. `TestValidateSchedule`,
  `TestStatus_LunchBreakClosed`), mas a validação+persistência no service, não.
  **Recomendação:** adicionar caso de Create/Update com `business_hours` inválido → `apperror.Validation`,
  e válido → persiste.
- **MED** — sem teste HTTP para `GET /v1/channels/{id}/business-status` nem para `PATCH` com
  `business_hours` em `presenter/controller/channels/` (controller test cobre conexões, não o status/horário).
- **OK (já coberto):** SLA por channel sem o guard — `domain/sla/service/sla_service_test.go:250`
  (`TestOnCreated_BusinessHoursOnly_UsesBusinessClock` + `recordingBizClock`); copilot `channel_id`
  vazio → sem assistente — `isp_bridge_test.go:126`.

---

## 4) Docs — divergências vs. código/OpenAPI

`docs/openapi.yaml` é o contrato gerado (fonte de verdade). Divergências dos `.md`:

- **HIGH** — `docs/api-design.md:246-249` descreve endpoints **singleton de providerhub que não
  existem**: `GET/POST/PATCH /providerhub/config` + `POST /providerhub/config/test`. Realidade
  (`app/routes/http/providerhub_routes.go:26,30-36` e openapi): só `GET /v1/providerhub/config`
  (status do gateway, re-significado) + CRUD `/v1/providerhub/profiles` + `/profiles/{id}/test` +
  `/profiles/{id}/default`. **Atualizar** para o modelo de perfis.
- **HIGH** — `docs/api-design.md` **não tem seção `automation-rules`**, embora os endpoints existam
  (`GET/POST/PATCH/DELETE /v1/automation-rules[/{id}]` + `GET /v1/automation-rules/{id}/logs`,
  documentados em `docs/automation-rules.md:7` e no openapi). **Adicionar** seção (incl. permissão
  `automation.manage`).
- **MED** — `docs/realtime-events.md:114-116` lista `ConversationPayload` **sem `channel_id`**, que
  já existe no schema `Conversation` (openapi) e no modelo
  (`infra/database/mongodb/models/conversations_models.go`, campo `ChannelID`). **Atualizar** a lista.

**Já atualizados nas Fases 2/3 (conferidos, OK):** `docs/backend-modules.md` (businesshours por
channel), `docs/data-model.md` (business_hours no channel + holidays `all_channels|channels`/`channel_ids`
+ `channel_id` em conversations), `docs/security-permissions.md` (horário do channel sob `channel.manage`),
`docs/api-design.md` (holidays + `/channels/{id}/business-status`).

**Nota:** `docs/audit-report.md` é um relatório histórico anterior; seu item L-3
(`MONITORING_RATE_PER_MINUTE`) já está resolvido (ver §6). Pode ser anotado como "resolvido" ou
arquivado.

---

## 5) Seed demo (`app/start_routines/seed_demo.go`)

Flags lidas em `app/config/config.go` (`SEED_DEMO_DATA`/`SEED_DEMO_RESET`). O seed está em modelo
**MISTO**: parte novo, parte legado.

### 5.1 business_hours no SETOR + shape antigo — **HIGH** → mover p/ channel + novo shape
`seed_demo.go:360-380` (`seedBusinessHours`) grava `business_hours` em **sectors** via
`db.Collection("sectors").UpdateOne(... $set business_hours ...)` usando o shape **antigo por NOME**
(`"weekly": {"monday":[...], ...}`). Duplo legado: lugar errado (sector não tem o campo;
`ParseSchedule` espera lista de intervalos `{day,intervals}`) e shape errado — nunca será lido.
**Recomendação:** gravar `BusinessHours` (novo shape `{timezone, weekly:[{day,intervals}]}`) na
`CreateConnection` de pelo menos um channel (o contrato já aceita: `domain/channels/contracts/connection.go`).

### 5.2 Conversas sem `channel_id` — **HIGH** → setar ChannelID
`seed_demo.go:766` cria conversas com `Channel: channel` (TIPO; `d.channels` é `[]string` de tipos,
populado em `:557` com `string(ch.typ)`) e **sem `ChannelID`**. Quebra o reuso de inbound
(`FindOpenByContactChannelID`, que casa por `channel_id`) e a resolução determinística de outbound.
**Recomendação:** guardar o `conn.ID` ao criar conexões (`:549-560`) e atribuir `ChannelID` a cada
conversa.

### 5.3 ProviderHub: cria config singleton legada — **MED** → criar ISPProfile
`seed_demo.go:508-520` cria `ProviderIntegrationConfig` pela `ConfigRepository` morta (`:509`,
coleção `providerhub_configs`) em vez de um `ISPProfile` (coleção `isp_profiles`, modelo novo).
**Recomendação:** criar um `ISPProfile` default via `ProfileRepository`; remover a criação legada
(casado com §1.1).

### 5.4 Sem CopilotAssistant — **MED** → criar 1 assistant (novo modelo)
O seed não cria nenhum `CopilotAssistant` (`domain/copilot/entity/assistant.go` com `ChannelIDs[]` +
`ISPProfileID`). O modelo novo (assistente por channel, ligado a perfil ISP) não é exercitado.
**Recomendação:** criar ≥1 assistant com `ChannelIDs` reais e `ISPProfileID` do perfil de §5.3.

### 5.5 OK — sem ação
Holiday do seed já usa `Scope: bhentity.ScopeAllChannels` (`seed_demo.go:384`) — modelo novo ✓
(corrigido na Fase 3).

---

## 6) Env / config

- **CLEAN** — sem variáveis órfãs. Os 85 keys de `.env.example` têm leitor em `app/config/config.go`
  (`getString/getInt/getBool/getDuration/getList`). O antigo órfão `MONITORING_RATE_PER_MINUTE`
  **não está mais** no `.env.example` (já removido; domínio monitoring extinto). Nenhuma flag aponta
  para coisa removida.
- **LOW (completude, não-legado)** — alguns keys LIDOS pelo `config.go` estão **ausentes** do
  `.env.example` (alguns com default seguro): `PRIVACY_SIGNING_SECRET`, `PRIVACY_DOWNLOAD_BASE_URL`,
  `PRIVACY_DOWNLOAD_TTL`, `PRIVACY_STORAGE_DIR`, `ATTACHMENTS_SIGNING_SECRET`, `ATTACHMENTS_PROVIDER`.
  **Recomendação:** documentá-los no `.env.example` (completude; não é resíduo de remoção).

---

## Resumo priorizado

| # | Item | Severidade | Recomendação |
|---|---|---|---|
| 1.1 | ProviderHub `ConfigRepository` (interface+impl+model storage) órfã | HIGH | Remover (+ teste §3.2, seed §5.3, migração 0030 §2) |
| 5.1 | Seed grava business_hours em sector + shape antigo | HIGH | Mover p/ channel, novo shape |
| 5.2 | Seed cria conversas sem `channel_id` | HIGH | Setar `ChannelID` |
| 3.2 | `config_live_test` exercita coleção morta | HIGH | Remover com 1.1 |
| 4 (a) | api-design.md: endpoints singleton providerhub inexistentes | HIGH | Atualizar p/ perfis |
| 4 (b) | api-design.md: falta seção automation-rules | HIGH | Adicionar |
| 1.2 | `FindEnabledByType` só no fallback sem channel_id | MED | Remover o `else` + método (decisão) |
| 2 | Migrações de conversão = no-op em banco novo (0024/0025/0030/0036/0037) + churn 0031↔0035 | MED | Remover/fundir e renumerar baseline |
| 5.3 | Seed cria config ISP singleton legada | MED | Criar `ISPProfile` |
| 5.4 | Seed não cria CopilotAssistant (novo modelo) | MED | Criar 1 assistant |
| 3.3 | Sem teste de service/HTTP para business_hours do channel | MED | Adicionar testes |
| 4 (c) | realtime-events.md: `ConversationPayload` sem `channel_id` | MED | Atualizar |
| 1.3 | Fake de teste `FindByChannelID(ct ...)` | LOW | Renomear `ct`→`channelID` |
| 6 | `.env.example` sem alguns keys de privacy/attachments | LOW | Documentar keys |

**Nenhuma alteração de código foi aplicada.** Próximo passo (após revisão): commits separados —
(1) código morto (§1.1/§3.2 + decisão §1.2), (2) docs (§4), (3) seed (§5) — e, opcionalmente,
limpeza de migrações (§2), cada um verde.
