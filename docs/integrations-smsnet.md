# Integração SMSNET (HTTP + MCP)

Como o backend fala com o SMSNET Integrations: um **gateway HTTP** (ProviderHub,
Customer 360) e **dois servidores MCP** (consultas/leitura e operações/escrita).
Configuração por **env default** com **override por tenant** no banco. Nada disso
é exposto ao front.

## Variáveis de ambiente

| Env | Uso | Lido por |
|---|---|---|
| `ISP_GATEWAY_API_HOST` | host do gateway HTTP do ProviderHub | api, worker |
| `ISP_GATEWAY_API_KEY` | chave do gateway (somente backend; nunca retornada) | api, worker |
| `SMSNET_MCP_CONSULTAS_URL` | MCP de **leitura** (tools sem aprovação) | api, worker |
| `SMSNET_MCP_OPERACOES_URL` | MCP de **escrita** (tools sempre com aprovação humana) | api, worker |

## Resolução de configuração

- **ProviderHub (HTTP):** o **gateway SMSNET é infra** — host/chave vêm **sempre**
  de env (`ISP_GATEWAY_API_HOST/KEY`), lidos só no backend e nunca retornados. O
  **ISP ativo** vem de um **perfil de ISP do tenant** (ver abaixo), não mais de uma
  config única. `GET /v1/providerhub/config` reporta o **status do gateway** +
  resumo dos perfis: `source` (`env|none`), `configured` (bool), `has_profiles`,
  `default_profile_id`, `profiles_count`. Não devolve host/chave.

### Perfis de ISP (múltiplos por tenant)

Um tenant tem **vários perfis de ISP** endereçáveis por id (coleção `isp_profiles`),
cada um com `label`, `isp_type` (validado contra o catálogo dos 19 slugs),
credenciais (**cifradas em repouso AES-GCM**; só as **keys** são expostas via
`credential_keys`), `is_default`, os dois toggles `usa_*`, `timeout_ms`, `enabled`.
O perfil **não** guarda host/chave do gateway (são infra). No máximo **um** perfil
por tenant é `is_default=true` (garantido por índice **único parcial**); o primeiro
perfil criado vira default automaticamente.

CRUD em `/v1/providerhub/profiles` (`integration.read` para ler, `integration.configure`
para escrever): `GET` (lista), `POST` (cria), `GET/PATCH/DELETE /{id}`,
`POST /{id}/default` (define default), `POST /{id}/test` (testa contra o gateway).
Credenciais **nunca** retornam em claro; as `credentials[].key` devem casar **1:1**
com o catálogo (`GET /v1/providerhub/catalog`) — ex.: `rbxsoft` exige
`rbxsoft_host/rbxsoft_token/rbxsoft_appkey`. Cada perfil expõe `actions[]` (derivado
do catálogo) para o front fazer o gating de ações por ISP. Não há toggles
`use_smsnet/use_email/use_whatsapp` — os únicos toggles são `enabled` e os dois `usa_*`.

> **Migração:** a config única legada (`providerhub_configs`) é migrada para **um**
> perfil `is_default=true` (migration idempotente), preservando o comportamento. O
> host/chave que a config legada porventura tivesse **deixam de ser usados** — o
> gateway passa a ser sempre env. Sem perfil → sem ISP ativo (ações externas
> indisponíveis, resposta clara, não 500).

> **Deletar perfil:** se o delete deixar o tenant **sem default** e sobrar
> **exatamente 1** perfil, ele é promovido a default automaticamente. Com **2+**
> restantes não há chute: o tenant fica sem default (`GET /config` →
> `default_profile_id: null`) e a UI pede para definir um ISP padrão.

### Busca manual na conversa (resolvedor tri-modal)

As consultas por conversa (`/v1/conversations/{id}/external/*`) são **POST** e
aceitam `isp_config_id?` no corpo. O **resolvedor** escolhe o perfil nesta ordem:
**explícito (`isp_config_id`) > default do tenant > (sem default e 1 elegível) esse
único > (sem default e 2+) `needs_isp_selection` > nenhum perfil → 409 claro**.
Quando ambíguo, a resposta é **HTTP 200** com
`{ "needs_isp_selection": true, "eligible": [ {id,label,isp_type,actions[]} ] }`
(não é erro): o agente escolhe e reenvia com `isp_config_id`. O `needs_input`
multi-contrato do gateway continua funcionando por cima disso.

`liberacao`/`chamado` (efeito colateral) aceitam o header `Idempotency-Key`
(replay via middleware + **propagado ao gateway** para dedup upstream; gerado no
backend quando omitido) e exigem `integration.execute_action`. As credenciais do
ISP nunca passam por camada de decisão de IA — o config é montado na borda.

### CopilotAssistant (transporte MCP)

`CopilotAssistant` (coleção `copilot_assistants`, vários por tenant) reusa o
`AIConfig` do tenant (provider/key/políticas) e adiciona roteamento: `ChannelIDs[]`
(ids de ChannelConnection específicas, casados com `conv.channel_id`) + a **FONTE DE
TOOLS EXTERNAS**, que é `ISPProfileID` **XOR** `MCPServerID` — **mutuamente
exclusivos** (ambos preenchidos → **422** "choose an ISP profile OR an MCP server,
not both"); os dois vazios = sem tools externas. CRUD em `/v1/copilot/assistants`
(`copilot.configure`), validando que `channel_ids`, o perfil e o servidor MCP
existem. Conversa com `channel_id` vazio → nenhum assistente resolve (sem fallback
por tipo).

**Fonte de tools no `OpenToolSession`** (caminho do copiloto), decidida pelo
assistente da conversa (`ISPToolBridge.ToolSource(channel_id)`):
- **ISP** (`isp_profile_id`): comportamento de sempre — tools **SMSNET** liberadas
  (gate por servidor: read sempre; write OPERACOES só se o perfil suporta
  liberacao/chamado) + `config{type+creds}` injetado server-side em
  `ToolService.invoke` (o modelo nunca vê credencial; em write, `idempotency_key` =
  `approval.ID`).
- **MCP** (`mcp_server_id`): o copiloto vê as tools **só daquele** servidor MCP do
  tenant, respeitando o `Kind` read/write (read executa; write cria `mcp_approval` /
  exige aprovação — mesmo gating).
- **nenhum**: sem tools externas.

> **Mudança de comportamento:** um servidor MCP registrado pelo tenant **só é
> exposto ao copiloto se um assistente o referenciar** via `mcp_server_id`. Servidor
> cadastrado e **não vinculado** a nenhum assistente **não aparece** para o copiloto
> (antes somava globalmente na união de tools). A lista **manual** do agente
> (`GET /v1/conversations/{id}/mcp/tools`) segue agregando todos os servidores
> habilitados — a mudança é escopada ao copiloto.

> **Integridade referencial:** deletar um perfil de ISP **vinculado a um
> CopilotAssistant** é **bloqueado** (409 "ISP em uso pelo assistente X"); deletar um
> **servidor MCP vinculado** é **bloqueado** (409 "servidor MCP em uso pelo
> assistente X") — nunca anula o vínculo silenciosamente.

> **Roadmap (F4):** a **automação** (transporte HTTP :8085, seleção de
> `isp_config_id` na regra, efeito colateral com idempotency key, estilo Chatwoot)
> permanece roadmap, ainda não implementada.
- **MCP:** os servidores `SMSNET_CONSULTAS` (read) e `SMSNET_OPERACOES` (write)
  entram no substrato MCP genérico via env default; um servidor **registrado pelo
  tenant com o mesmo nome** sobrescreve a URL (tenant DB override → env default).
  - `CONSULTAS` = **read**: a IA chama no loop **sem aprovação**.
  - `OPERACOES` = **write**: **sempre** `human_approval_required` → cria um
    `mcp_approval` pendente; só executa após
    `POST /v1/conversations/{id}/copilot/approvals/{id}` (gate
    `integration.execute_action`), **auditado**.

> **Path do endpoint MCP (streamable_http):** o transporte Streamable HTTP serve em
> **`/mcp`** por convenção (default do `NewStreamableHTTPServer` do mark3labs). O
> cliente do backend **acrescenta `/mcp` automaticamente** quando a `base_url` não
> tem path (ex.: `http://127.0.0.1:8086` → `…:8086/mcp`); se a `base_url` já incluir
> um path (ex.: `…/mcp` ou um mount custom), ele é respeitado como está. Uma falha de
> descoberta (`tools/list`) registra a **causa concreta** (status/path/handshake) no
> log `mcp tools/list failed` (server_id/name/base_url/cause), mesmo que o cliente
> devolva o 502 amigável "could not list tools".

## Segurança / deploy

- Os hosts SMSNET (ex.: portas **8085/8086/8087**) ficam em **rede privada** e
  **nunca** são expostos à internet. Por isso os **MCP não têm autenticação**
  (sem credencial/cliente); a barreira é a rede. Valide no deploy que essas URLs
  são internas.
- **Nada** disso vaza para o front: nenhuma rota `/v1` retorna `ISP_GATEWAY_API_HOST`,
  as `*_MCP_*_URL` ou a chave em claro. O front só vê flags de
  configurado/ligado, a **lista de tools** (nomes) via
  `GET /v1/conversations/{id}/mcp/tools` e resultados via `POST .../mcp/run`.
- **Health no startup:** o backend faz um probe best-effort (não-fatal) de cada
  endpoint configurado e loga em INFO `reachable=true|false` (apenas o nome da env,
  nunca a URL). Inalcançável não impede o boot.

---

# Copiloto (provedor de IA)

Inferência do copiloto (`POST /v1/copilot/suggest-reply` etc.) contra um provedor
OpenAI-compatível (OpenAI/Mistral/DeepSeek/Perplexity), Anthropic ou Gemini.

## Infra global vs. comportamento por assistente (modelo híbrido)

A **`copilot_config`** (tenant, `PATCH /v1/copilot/config`) carrega **só a infra de
IA compartilhada**: `provider`, `model`, `api_key`, `base_url`, `enabled`. Uma
**única chave** serve todos os segmentos.

O **comportamento** desceu para o **`CopilotAssistant`** (por assistente/canal):
`allow_customer_data` (único gate de pré-injeção), `human_approval_required`,
`temperature` (0–2), `max_tokens` (>0) e `system_instructions` (persona/conduta,
texto livre). Dados financeiros/monitoramento **não têm gate** — são consultados
sob demanda via **tool do ISP**, nunca pré-injetados. Assim canais diferentes (ex.:
WhatsApp de ISP vs. de loja) têm gate, persona e temperatura próprios sem replicar
a chave de IA.

**Resolução na inferência** (`Service.run`): resolve o assistente pela
`conv.channel_id` (`FindByChannelID`) e monta o `Request` com **provider/model/key
da config global** + **gates/temperature/max_tokens/system_instructions do
assistente**. O `system_instructions` é **concatenado** ao system prompt fixo da
ação: `systemPrompt(action)` + `"\n\n"` + persona (o fixo garante o comportamento
base — idioma, concisão, formato; a persona adiciona segmento).

**Sem assistente resolvido** (`channel_id` vazio ou nenhum assistente serve o
canal): o copiloto roda com **`DefaultBehavior` conservador** — **todos os gates
OFF** (nada sensível no prompt), **sem persona**, `temperature=0.7`,
`max_tokens=512`. Não cai em gate global (não existe mais) e não bloqueia.

## Resolução da API key (precedência)

1. **Config do tenant** (`copilot_config.api_key`, cifrada AES-GCM via
   `PATCH /v1/copilot/config`) — **vence** se preenchida.
2. **Env default** por provedor, usada como **fallback** quando a key do tenant
   está vazia: `COPILOT_OPENAI_API_KEY`, `COPILOT_GEMINI_API_KEY`,
   `COPILOT_ANTHROPIC_API_KEY`.
3. **Ambas vazias** → erro acionável `category:"api_key_missing"`
   ("configure a API key do copiloto"), **não** um 502 genérico.

> O seed demo **não** grava api_key placeholder (ficaria "configurado" mas
> falharia): cria a config com `api_key` vazia (`has_api_key:false`), então o
> fallback de env passa a valer. `GET /v1/copilot/config` nunca devolve a key em
> claro (só `has_api_key`).

## base_url

`copilot_config.base_url` vazio → default do provedor (OpenAI:
`https://api.openai.com/v1`). Se setado (proxy/gateway corporativo), é usado como
está; um base_url errado também causa 502.

## Erros do provedor (502)

Toda falha de chamada vira `integration_unavailable` (502) com mensagem amigável
**+ `details.category`** para a UI orientar:

| category | causa real (logada server-side) | mensagem ao usuário |
|---|---|---|
| `api_key` | 401 / `invalid_api_key` | "API key inválida ou ausente — verifique a config" |
| `model` | 404 / `model_not_found` | "modelo configurado indisponível para esta key" |
| `quota` | 429 / `insufficient_quota` | "cota/limite do provedor atingido" |
| `unreachable` | timeout / DNS / conexão recusada | "copiloto temporariamente indisponível" |
| `api_key_missing` | sem key (tenant nem env) | "configure a API key do copiloto" |

A **causa real** (status HTTP + corpo do erro do provedor) é registrada no log do
servidor (`copilot provider call failed`, com `provider/model/base_url/cause`) e
no `AILog`; o corpo bruto **nunca** vai ao cliente.

## Conectividade de saída (egress)

O backend precisa de **egress** para `api.openai.com` (ou o `base_url`
configurado). Em rede restrita, libere esse destino. No startup, se
`COPILOT_OPENAI_API_KEY` estiver setada, há um probe best-effort (não-fatal) que
loga `reachable=true|false` para `api.openai.com`.
