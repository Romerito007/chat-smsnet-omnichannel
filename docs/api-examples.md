# API — exemplos de uso

Exemplos `curl` dos principais fluxos. A base é `http://localhost:8080`. Toda a
superfície versionada fica sob `/v1`. O **tenant é sempre derivado do token de
acesso** (nunca de um header). Erros seguem o envelope padrão:

```json
{ "error": { "code": "validation_error", "message": "...", "details": {}, "request_id": "..." } }
```

Convenção: exporte o token depois do login.

```bash
BASE=http://localhost:8080
```

## Autenticação

```bash
# Login → access + refresh token (ação auditada: auth.login)
curl -s -X POST $BASE/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"owner@example.com","password":"change-me-now"}'

export TOKEN="<access_token>"
AUTH="Authorization: Bearer $TOKEN"

# Logout (ação auditada: auth.logout)
curl -s -X POST $BASE/v1/auth/logout \
  -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"refresh_token":"<refresh>"}'
```

## IAM — usuários e papéis (auditado)

```bash
# Criar usuário (auditado: user.created)
curl -s -X POST $BASE/v1/users -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"Ana","email":"ana@acme.com","password":"supersecret","role_ids":["<role>"]}'

# Alterar papel/permissões (auditado: role.updated, permissions_changed=true)
curl -s -X PATCH $BASE/v1/roles/<id> -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"permissions":["conversation.read","message.send"]}'
```

## Conversas (transferência/fechamento auditados)

```bash
# Transferir (auditado: conversation.transferred)
curl -s -X POST $BASE/v1/conversations/<id>/transfer -H "$AUTH" \
  -H 'Content-Type: application/json' -d '{"sector_id":"<sector>","agent_id":"<agent>"}'

# Fechar (auditado: conversation.closed)
curl -s -X POST $BASE/v1/conversations/<id>/close -H "$AUTH" \
  -H 'Content-Type: application/json' -d '{"close_reason_id":"<reason>","note":"resolvido"}'
```

## Attachments — anexos por URL assinada

Fluxo em três passos: pedir URL assinada → fazer upload direto no storage →
confirmar e vincular à mensagem. O download sempre valida acesso à conversa.

```bash
# 1) Pedir URL de upload (valida content_type + size)
curl -s -X POST $BASE/v1/attachments/upload-url -H "$AUTH" \
  -H 'Content-Type: application/json' \
  -d '{"conversation_id":"<cv>","filename":"foto.png","content_type":"image/png","size":12345}'
# → { "attachment_id":"...", "upload_url":"...", "method":"PUT",
#     "headers":{"Content-Type":"image/png"}, "expires_at":"..." }

# 2) Upload direto no storage (backend local: PUT no upload_url; S3: PUT na URL pré-assinada)
curl -s -X PUT "<upload_url>" -H 'Content-Type: image/png' --data-binary @foto.png

# 3) Confirmar e vincular à mensagem
curl -s -X POST $BASE/v1/attachments/confirm -H "$AUTH" \
  -H 'Content-Type: application/json' \
  -d '{"attachment_id":"<id>","message_id":"<msg>"}'
# → { ..., "status":"ready", "message_id":"<msg>", "download_url":".../download" }

# 4) Download (valida acesso à conversa; local serve os bytes, S3 redireciona 302
#    para uma URL pré-assinada curta)
curl -sL $BASE/v1/attachments/<id>/download -H "$AUTH" -o foto.png
```

Configuração: `ATTACHMENTS_PROVIDER=local|s3`, `ATTACHMENTS_MAX_SIZE_BYTES`,
`ATTACHMENTS_ALLOWED_CONTENT_TYPES` (ex.: `image/*,application/pdf`),
`ATTACHMENTS_UPLOAD_TTL`, `ATTACHMENTS_DOWNLOAD_TTL`. Para S3:
`ATTACHMENTS_S3_ENDPOINT`, `_REGION`, `_BUCKET`, `_ACCESS_KEY`, `_SECRET_KEY`.

## Auditoria

```bash
# Listar trilha (permissão audit.view); filtros opcionais
curl -s "$BASE/v1/audit?action=auth.&limit=50" -H "$AUTH"
curl -s "$BASE/v1/audit?resource_id=<user_id>" -H "$AUTH"
# Cada item: { id, actor_id, actor_type, action, resource_type, resource_id,
#              ip, user_agent, data, created_at }
```

## Privacy (LGPD)

```bash
# Exportar dados do contato (permissão privacy.manage) → job assíncrono
curl -s -X POST $BASE/v1/privacy/contacts/<id>/export -H "$AUTH"
curl -s $BASE/v1/privacy/exports/<export_id> -H "$AUTH"   # poll até status=ready
curl -sL "<download_url>"                                  # URL assinada temporária

# Anonimizar (remove PII, mantém integridade; recusa sob legal hold)
curl -s -X POST $BASE/v1/privacy/contacts/<id>/anonymize -H "$AUTH"

# Retenção por tenant
curl -s $BASE/v1/privacy/retention -H "$AUTH"
curl -s -X PATCH $BASE/v1/privacy/retention -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"messages_days":365,"audit_logs_days":730,"notifications_days":90}'
```

## Relatórios

```bash
# Visões operacionais (permissão report.view)
curl -s "$BASE/v1/reports/overview?from=2026-01-01T00:00:00Z&to=2026-02-01T00:00:00Z" -H "$AUTH"
curl -s "$BASE/v1/reports/sla?sector_id=<s>" -H "$AUTH"

# Exportar relatório (permissão report.export; auditado: report.export).
# Gera o arquivo (csv|json) na hora e devolve uma URL assinada e temporária.
curl -s -X POST "$BASE/v1/reports/export?report=conversations&format=csv" -H "$AUTH"
# → 200 { "report":"conversations", "format":"csv", "filename":"...",
#          "download_url":"$BASE/v1/reports/downloads/<token>", "expires_at":"...", "bytes":1234 }

# Baixar o arquivo gerado (público: o token assinado é a credencial).
curl -s -L "$BASE/v1/reports/downloads/<token>" -o report.csv
```

## Webhooks (auditado)

```bash
# Criar (auditado: webhook.created); o secret é retornado só uma vez
curl -s -X POST $BASE/v1/webhooks -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"crm","url":"https://crm.example/hooks","events":["conversation.closed"]}'
```
