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
