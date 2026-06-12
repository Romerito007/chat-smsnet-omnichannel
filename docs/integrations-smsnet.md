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

- **ProviderHub (HTTP):** config do **tenant no DB** (se existir e habilitada) →
  senão **env default** (`ISP_GATEWAY_API_HOST/KEY`). A chave do tenant fica
  **cifrada em repouso (AES-GCM)**; a chave de env é lida só no backend.
  `GET /v1/providerhub/config` nunca devolve a chave em claro nem o host de env:
  retorna `has_api_key`, `source` (`tenant|env|none`) e `configured` (bool). Para
  `source:"env"` só informa que está configurado (sem host/chave).
- **MCP:** os servidores `SMSNET_CONSULTAS` (read) e `SMSNET_OPERACOES` (write)
  entram no substrato MCP genérico via env default; um servidor **registrado pelo
  tenant com o mesmo nome** sobrescreve a URL (tenant DB override → env default).
  - `CONSULTAS` = **read**: a IA chama no loop **sem aprovação**.
  - `OPERACOES` = **write**: **sempre** `human_approval_required` → cria um
    `mcp_approval` pendente; só executa após
    `POST /v1/conversations/{id}/copilot/approvals/{id}` (gate
    `integration.execute_action`), **auditado**.

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
