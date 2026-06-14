package openapi

// schemas returns the reusable component schemas referenced by the paths. They
// mirror the real presenter DTOs (field names, types, enums).
func schemas() M {
	return M{
		// ── cross-cutting ──────────────────────────────────────────────────────
		"Error": object(M{
			"error": object(M{
				"code":       enum("validation_error", "unauthorized", "forbidden", "not_found", "conflict", "rate_limited", "integration_unavailable", "internal_error"),
				"message":    str(),
				"details":    freeObject(),
				"request_id": str(),
			}, "code", "message"),
		}, "error"),
		"PageInfo": object(M{
			"next_cursor": str(),
			"has_more":    boolean(),
		}, "has_more"),
		"MessageAck": object(M{"message": str()}),

		// ── platform plane (provisioner; X-Platform-Key) ───────────────────────
		"ProvisionTenantRequest": object(M{
			"tenant_name":    str(),
			"owner_name":     str(),
			"owner_email":    str(),
			"owner_password": str(),
			"external_ref":   describedStr("The provisioner's natural key for this account. Durable idempotency: a repeat with the same external_ref returns the existing tenant + a fresh token instead of creating a duplicate."),
		}, "tenant_name", "owner_name", "owner_email", "owner_password", "external_ref"),
		"ProvisionTenantResponse": object(M{
			"tenant":            object(M{"id": str(), "name": str()}),
			"owner":             object(M{"id": str(), "email": str()}),
			"access_token":      describedStr("A ready-to-use tenant-scoped Bearer access token for the owner. Use it directly on the tenant API (e.g. POST /v1/channels) — no extra login step. Access-only (no refresh)."),
			"token_type":        str(),
			"access_expires_at": dateTime(),
			"created":           describedBool("True when a new tenant was created; false when an existing tenant was returned for a repeated external_ref."),
		}),

		// ── auth / iam ─────────────────────────────────────────────────────────
		"LoginRequest":          object(M{"email": str(), "password": str()}, "email", "password"),
		"RefreshRequest":        object(M{"refresh_token": str()}, "refresh_token"),
		"LogoutRequest":         object(M{"refresh_token": str()}, "refresh_token"),
		"SignupRequest":         object(M{"company_name": str(), "owner_name": str(), "email": str(), "password": str()}, "company_name", "owner_name", "email", "password"),
		"VerifyEmailRequest":    object(M{"token": str()}, "token"),
		"EmailRequest":          object(M{"email": str()}, "email"),
		"ResetPasswordRequest":  object(M{"token": str(), "new_password": str()}, "token", "new_password"),
		"AcceptInviteRequest":   object(M{"token": str(), "name": str(), "password": str()}, "token", "name", "password"),
		"InviteUserRequest":     object(M{"email": str(), "role_ids": arr(str()), "sector_ids": arr(str())}, "email"),
		"InviteResponse":        object(M{"id": str(), "email": str()}),
		"UpdateProfileRequest":  object(M{"name": str(), "avatar_attachment_id": str()}),
		"ChangePasswordRequest": object(M{"current_password": str(), "new_password": str()}, "current_password", "new_password"),
		"TokenResponse": object(M{
			"access_token": str(), "token_type": str(), "access_expires_at": dateTime(),
			"refresh_token": str(), "refresh_expires_at": dateTime(),
			"user": ref("User"), "permissions": arr(str()),
		}),
		"MeResponse": object(M{
			"user": ref("User"), "permissions": arr(str()),
			"sector_scope": enum("own", "all"), "sector_ids": arr(str()),
		}),
		"User": object(M{
			"id": str(), "tenant_id": str(), "name": str(), "email": str(),
			"status":   enum("active", "disabled", "pending_verification"),
			"role_ids": arr(str()), "sector_ids": arr(str()), "max_concurrent_chats": integer(),
			"avatar_attachment_id": str(),
			"avatar_url":           describedStr("Read-only, derived: a short-lived signed URL loadable directly in <img src> (no Authorization). Present only when the avatar exists and is ready. Do not cache long-term."),
			"created_at":           dateTime(), "updated_at": dateTime(),
		}),
		"CreateUserRequest": object(M{
			"name": str(), "email": str(), "password": str(),
			"role_ids": arr(str()), "sector_ids": arr(str()), "max_concurrent_chats": integer(),
		}, "name", "email", "password"),
		"UpdateUserRequest": object(M{
			"name": str(), "password": str(), "status": enum("active", "disabled"),
			"role_ids": arr(str()), "sector_ids": arr(str()), "max_concurrent_chats": integer(),
		}),
		"Role": object(M{
			"id": str(), "tenant_id": str(), "name": str(), "permissions": arr(str()),
			"sector_scope": enum("own", "all"), "created_at": dateTime(), "updated_at": dateTime(),
		}),
		"CreateRoleRequest": object(M{"name": str(), "permissions": arr(str()), "sector_scope": enum("own", "all")}, "name"),
		"UpdateRoleRequest": object(M{"name": str(), "permissions": arr(str()), "sector_scope": enum("own", "all")}),

		// ── tenant / sectors / queues / presence ───────────────────────────────
		"Tenant":              object(M{"id": str(), "name": str(), "status": str(), "settings": freeObject(), "created_at": dateTime(), "updated_at": dateTime()}),
		"UpdateTenantRequest": object(M{"name": str(), "settings": freeObject()}),
		"Sector": object(M{
			"id": str(), "tenant_id": str(), "name": str(), "description": str(),
			"enabled": boolean(), "created_at": dateTime(), "updated_at": dateTime(),
		}),
		"CreateSectorRequest": object(M{"name": str(), "description": str(), "enabled": boolean()}, "name"),
		"UpdateSectorRequest": object(M{"name": str(), "description": str(), "enabled": boolean()}),
		"Queue": object(M{
			"id": str(), "tenant_id": str(), "sector_id": str(), "name": str(),
			"strategy": str(), "max_wait_seconds": integer(), "enabled": boolean(),
			"created_at": dateTime(), "updated_at": dateTime(),
		}),
		"CreateQueueRequest": object(M{"sector_id": str(), "name": str(), "strategy": str(), "max_wait_seconds": integer(), "enabled": boolean()}, "sector_id", "name"),
		"UpdateQueueRequest": object(M{"name": str(), "strategy": str(), "max_wait_seconds": integer(), "enabled": boolean()}),
		"Presence": object(M{
			"tenant_id": str(), "user_id": str(), "status": str(), "current_load": integer(),
			"max_concurrent_chats": integer(), "last_seen_at": dateTime(),
		}),
		"SetStatusRequest": object(M{"user_id": str(), "status": str()}, "status"),

		// ── conversations / messages ───────────────────────────────────────────
		"Conversation": object(M{
			"id": str(), "tenant_id": str(), "contact_id": str(), "channel": str(),
			"channel_id": describedStr("Id of the specific ChannelConnection this conversation belongs to (e.g. which of several same-type WhatsApp numbers). Empty only for conversations created without one."),
			"sector_id":  str(), "queue_id": str(), "status": conversationStatusEnum(), "assigned_to": str(),
			"priority": str(), "tags": tagIDArray(), "last_message_at": dateTime(),
			"unread_count": integer(), "last_read_at": dateTime(),
			"created_at": dateTime(), "updated_at": dateTime(), "closed_at": dateTime(),
			"last_message":       ref("LastMessage"),
			"contact_name":       describedStr("Read-only, derived: the conversation contact's display name, resolved in batch so the inbox renders the row without a per-contact fetch. Empty when the contact is absent."),
			"contact_avatar_url": describedStr("Read-only, derived: the conversation contact's short-lived signed avatar URL (loadable in <img src>, no Authorization), resolved in batch for the inbox. Empty when the contact has no ready avatar."),
			"agent_name":         describedStr("Read-only, derived: the assignee's display name, resolved in batch. Empty when the conversation is unassigned."),
			"agent_avatar_url":   describedStr("Read-only, derived: the assignee's short-lived signed avatar URL (no Authorization), resolved in batch. Empty when unassigned or no ready avatar."),
		}),
		"LastMessage": object(M{
			"preview": str(), "sender_type": str(), "message_type": str(), "created_at": dateTime(),
		}),
		"AssignableAgent": object(M{
			"id": str(), "name": str(), "status": str(),
			"current_load": integer(), "max_concurrent_chats": integer(),
			"avatar_url": describedStr("Read-only, derived: the agent's short-lived signed avatar URL (loadable in <img src>, no Authorization). Empty when the agent has no ready avatar."),
		}),
		"McpCallLog": object(M{
			"id": str(), "conversation_id": str(), "server_name": str(), "tool": str(),
			"write": boolean(), "status": str(), "latency_ms": integer(),
			"error_summary": str(), "created_at": dateTime(),
		}),
		"ConversationEvent": object(M{
			"id": str(), "conversation_id": str(), "type": str(),
			"actor_type": enum("agent", "customer", "system", "copilot"),
			"actor_id":   str(), "data": freeObject(), "created_at": dateTime(),
		}),
		"CreateConversationRequest": object(M{
			"contact_id": str(), "channel": str(),
			"channel_id": describedStr("Optional id of the specific ChannelConnection (inbound sets it automatically; provide it for outbound to leave through the right connection)."),
			"sector_id":  str(), "queue_id": str(),
			"assigned_to": str(), "priority": str(), "tags": tagIDArray(),
		}, "contact_id", "channel"),
		"UpdateConversationRequest": object(M{
			"sector_id": str(), "queue_id": str(), "status": conversationStatusEnum(), "assigned_to": str(),
			"priority": str(), "tags": tagIDArray(),
		}),
		"Attachment": object(M{"id": str(), "url": str(), "content_type": str(), "filename": str(), "size": integer()}),
		"Message": object(M{
			"id": str(), "conversation_id": str(), "sender_type": str(), "sender_id": str(),
			"direction": str(), "message_type": str(), "text": str(),
			"attachments": arr(ref("Attachment")), "metadata": freeObject(),
			"delivery_status": str(), "external_message_id": str(),
			"created_at": dateTime(), "edited_at": dateTime(),
		}),
		"SendMessageRequest": object(M{
			"message_type": str(), "text": str(), "attachments": arr(ref("Attachment")), "metadata": freeObject(),
		}),
		"EditMessageRequest":  object(M{"text": str()}, "text"),
		"InternalNoteRequest": object(M{"text": str(), "mention_user_ids": arr(str())}, "text"),
		"CloseRequest":        object(M{"close_reason_id": str(), "note": str()}),
		"AssignRequest":       object(M{"agent_id": str()}),
		"TransferRequest":     object(M{"sector_id": str(), "agent_id": str()}),
		"EnqueueRequest":      object(M{"queue_id": str()}),
		"RoutingRunRequest":   object(M{"conversation_id": str()}, "conversation_id"),
		"ApplyTagsRequest":    object(M{"add": tagIDArray(), "remove": tagIDArray()}),

		// ── channels ───────────────────────────────────────────────────────────
		"Channel": object(M{
			"id": str(), "tenant_id": str(), "type": str(), "name": str(), "status": str(),
			"base_url": str(), "auth_type": str(), "has_secret": boolean(), "has_inbound_token": boolean(),
			"default_sector_id": str(), "business_hours": ref("BusinessHours"),
			"enabled":    boolean(),
			"created_at": dateTime(), "updated_at": dateTime(),
		}),
		"ChannelCreated": object(M{
			"id": str(), "tenant_id": str(), "type": str(), "name": str(), "status": str(),
			"base_url": str(), "auth_type": str(), "has_secret": boolean(), "has_inbound_token": boolean(),
			"default_sector_id": str(), "business_hours": ref("BusinessHours"),
			"enabled":    boolean(),
			"created_at": dateTime(), "updated_at": dateTime(),
			"inbound_token": str(), "outbound_secret": str(),
		}),
		"CreateChannelRequest": object(M{
			"type": str(), "name": str(), "base_url": str(), "outbound_url": str(),
			"auth_type": str(), "secret": str(), "outbound_secret": str(),
			"default_sector_id": str(), "business_hours": ref("BusinessHours"),
		}, "type"),
		"UpdateChannelRequest": object(M{
			"name": str(), "status": str(), "base_url": str(), "outbound_url": str(),
			"auth_type": str(), "secret": str(), "outbound_secret": str(),
			"default_sector_id": str(), "business_hours": ref("BusinessHours"),
			"enabled": boolean(),
		}),
		"InboundMessageRequest": object(M{
			"inbound_token": str(), "tenant_key": str(), "integration_key": str(), "webhook_verify_token": str(),
			"external_message_id": str(), "external_contact_id": str(), "contact_name": str(),
			"contact_phone": str(), "contact_document": str(), "channel": str(), "text": str(),
			"attachments": arr(ref("Attachment")), "metadata": freeObject(), "timestamp": integer(),
		}),
		// Chatwoot-compatible multipart/form-data: content + message_type + file_type
		// + attachments[] (raw files). Routing fields mirror the JSON shape.
		"InboundMessageMultipart": M{
			"type": "object",
			"properties": M{
				"inbound_token": str(), "external_message_id": str(),
				"external_contact_id": str(), "contact_phone": str(), "contact_name": str(),
				"contact_document": str(), "content": str(), "message_type": str(),
				"private":       boolean(),
				"file_type":     enum("image", "audio", "video", "document"),
				"timestamp":     integer(),
				"attachments[]": M{"type": "array", "items": M{"type": "string", "format": "binary"}},
			},
		},
		// ChannelOutboundMessage documents the Chatwoot-compatible envelope this
		// backend POSTs to a channel's outbound_url (not a served /v1 endpoint).
		"ChannelOutboundMessage": object(M{
			"delivery_id": str(), "conversation_id": str(), "timestamp": integer(),
			"contact": object(M{"id": str(), "name": str(), "phone": str(), "external_id": str()}),
			"message": object(M{
				"content": str(), "text": str(), "message_type": str(), "private": boolean(),
				"file_type": str(),
				"attachments": arr(object(M{
					"url": str(), "data_url": str(),
					"file_type":    enum("image", "audio", "video", "file"),
					"content_type": str(), "filename": str(), "size": integer(),
				})),
			}),
			"metadata": freeObject(),
		}),
		"RotatedInboundToken": object(M{"inbound_token": str()}, "inbound_token"),
		"TestResult":          object(M{"ok": boolean(), "external_message_id": str(), "error": str()}),

		// ── automation rules (Chatwoot-style trigger/conditions/actions engine) ──
		// An AutomationRule reacts to a conversation/message lifecycle event, matches
		// AND-conditions against the conversation/contact, and runs actions (only
		// send_webhook for now, referencing a registered webhook by id).
		"AutomationRule": object(M{
			"id": str(), "tenant_id": str(), "name": str(), "description": str(),
			"event":      automationRuleEventEnum(),
			"enabled":    boolean(),
			"conditions": arr(ref("AutomationRuleCondition")),
			"actions":    arr(ref("AutomationRuleAction")),
			"created_at": dateTime(), "updated_at": dateTime(),
		}),
		"AutomationRuleCondition": object(M{
			"field":    enum("status", "channel", "assigned_agent_id", "sector_id", "queue_id", "priority", "tags", "contact_phone"),
			"operator": withDesc(enum("equal_to", "not_equal_to", "contains", "does_not_contain"), "Allowed operators depend on the field: scalar fields → equal_to/not_equal_to; tags → contains/does_not_contain; contact_phone → equal_to/contains."),
			"value":    describedStr("Comparison value; for tags it is a single tag id."),
		}, "field", "operator", "value"),
		"AutomationRuleAction": object(M{
			"type":       withDesc(enum("send_webhook"), "Only send_webhook is supported; it delivers the event to webhook_id via the webhooks pipeline."),
			"webhook_id": describedStr("Id of a registered webhook (/v1/webhooks). Must exist for the tenant."),
		}, "type"),
		"CreateAutomationRuleRequest": object(M{
			"name": str(), "description": str(),
			"event":      automationRuleEventEnum(),
			"enabled":    describedBool("Defaults to true when omitted."),
			"conditions": withDesc(arr(ref("AutomationRuleCondition")), "AND-combined. Empty = match every occurrence of the event."),
			"actions":    arr(ref("AutomationRuleAction")),
		}, "name", "event", "actions"),
		"UpdateAutomationRuleRequest": object(M{
			"name": str(), "description": str(), "event": automationRuleEventEnum(), "enabled": boolean(),
			"conditions": arr(ref("AutomationRuleCondition")), "actions": arr(ref("AutomationRuleAction")),
		}),
		"RuleEvaluationLog": object(M{
			"id": str(), "rule_id": str(), "event": automationRuleEventEnum(), "conversation_id": str(),
			"status":        withDesc(enum("action_enqueued", "skipped_dedup", "error"), "Outcome of a rule firing. skipped_dedup is the anti-loop guard (same rule+conversation+event fired within a short window)."),
			"error_summary": str(), "created_at": dateTime(),
		}),

		// ── providerhub ────────────────────────────────────────────────────────
		// ProviderHubGatewayStatus is the GET /v1/providerhub/config response. The
		// SMSNET gateway is infra now (env ISP_GATEWAY_API_HOST/KEY), so this reports
		// whether the gateway is configured plus a summary of the tenant's ISP
		// profiles. The infra host/key are never returned. The "active" ISP always
		// comes from a profile (default or explicit), managed under /providerhub/profiles.
		"ProviderHubGatewayStatus": object(M{
			"source":             withDesc(enum("env", "none"), "Gateway resolution: env (infra default ISP_GATEWAY_API_HOST/KEY is set) | none (gateway not configured). The host/key are never returned."),
			"configured":         describedBool("True when the shared SMSNET gateway (env host) is configured."),
			"has_profiles":       describedBool("True when the tenant has at least one ISP profile."),
			"default_profile_id": describedStr("Id of the tenant's default ISP profile, if any."),
			"profiles_count":     describedInt("Number of ISP profiles the tenant has."),
		}),
		// ProviderHubCatalog is the static, versioned catalog of supported ISPs
		// (GET /v1/providerhub/catalog): per ISP the credential fields to render and
		// the actions it supports — the front hard-codes nothing.
		"ProviderHubCatalog": object(M{
			"version": str(),
			"isps":    arr(ref("ISPCatalogEntry")),
		}),
		"ISPCatalogEntry": object(M{
			"slug": str(), "label": str(),
			"credentials": arr(ref("ISPCredentialField")),
			"actions":     arr(enum("cliente", "planos", "empresa", "liberacao", "chamado")),
			"search_by":   arr(enum("cpfcnpj", "phone", "email")),
		}),
		"ISPCredentialField": object(M{
			"key": str(), "label": str(),
			"secret": describedStr("True → render a masked input; the value is never echoed back by the profile endpoints."),
		}),
		// ISPProfile is one addressable ISP configuration a tenant holds (many per
		// tenant). Credentials are masked: only the keys are returned, never values.
		// actions[] is derived from the catalog for the profile's isp_type.
		"ISPProfile": object(M{
			"id":                              str(),
			"tenant_id":                       str(),
			"label":                           describedStr("Human label distinguishing profiles (e.g. \"IXC matriz\")."),
			"isp_type":                        ispTypeStr(),
			"credential_keys":                 describedArr(str(), "ISP credential KEYS only (e.g. hubsoft_host); values are never returned."),
			"is_default":                      describedBool("True for the tenant's default profile (at most one)."),
			"actions":                         describedArr(enum("cliente", "planos", "empresa", "liberacao", "chamado"), "Actions this profile's ISP supports, from the catalog — gate per-ISP actions with this."),
			"usa_pegar_fatura_atrasada":       boolean(),
			"usa_extrair_linha_digitavel_pdf": boolean(),
			"timeout_ms":                      integer(),
			"enabled":                         boolean(),
			"created_at":                      dateTime(),
			"updated_at":                      dateTime(),
		}),
		"CreateISPProfileRequest": object(M{
			"label":                           describedStr("Required. Human label distinguishing profiles."),
			"isp_type":                        ispTypeStr(),
			"credentials":                     withDesc(stringMap(), "ISP credentials; keys must match the catalog for isp_type 1:1 (GET /v1/providerhub/catalog). Values are write-only and never returned."),
			"is_default":                      describedBool("Make this the default profile. The first profile of a tenant is always the default."),
			"usa_pegar_fatura_atrasada":       boolean(),
			"usa_extrair_linha_digitavel_pdf": boolean(),
			"timeout_ms":                      integer(),
			"enabled":                         describedBool("Defaults to true when omitted."),
		}, "label", "isp_type"),
		"UpdateISPProfileRequest": object(M{
			"label":                           str(),
			"isp_type":                        ispTypeStr(),
			"credentials":                     withDesc(stringMap(), "Replaces the credentials; keys must match the catalog for the (possibly new) isp_type 1:1. Write-only."),
			"usa_pegar_fatura_atrasada":       boolean(),
			"usa_extrair_linha_digitavel_pdf": boolean(),
			"timeout_ms":                      integer(),
			"enabled":                         boolean(),
		}),
		"ISPProfileTestResult": object(M{
			"ok": boolean(), "latency_ms": integer(), "error": str(),
		}),
		"ClienteRequest": object(M{
			"isp_config_id": describedStr("ISP profile id to use; omit to use the tenant default. If there is no default and 2+ profiles, the response is a NeedsISPSelection prompt."),
			"cpfcnpj":       str(), "phone": str(), "email": str(),
			"id_cliente": describedStr("Target a specific contract after a needs_input selection."),
		}),
		"ISPSelectorRequest": object(M{
			"isp_config_id": describedStr("ISP profile id to use; omit to use the tenant default."),
		}),
		"LiberacaoRequest": object(M{"isp_config_id": str(), "id_cliente": str()}, "id_cliente"),
		"ChamadoRequest":   object(M{"isp_config_id": str(), "id_cliente": str(), "subject": str(), "message": str()}, "id_cliente"),
		// NeedsISPSelection is returned (HTTP 200) by the external endpoints when the
		// ISP profile is ambiguous (no default, 2+ eligible). The agent picks one and
		// re-sends with isp_config_id. NOT an error.
		"NeedsISPSelection": object(M{
			"needs_isp_selection": describedBool("Always true on this response shape."),
			"eligible": arr(object(M{
				"id": str(), "label": str(), "isp_type": str(),
				"actions": arr(enum("cliente", "planos", "empresa", "liberacao", "chamado")),
			})),
		}, "needs_isp_selection", "eligible"),

		// ── Customer 360 (smsnet-integrations on-demand results) ────────────────
		"Fatura": object(M{
			"valor": number(), "vencimento": str(), "link": str(),
			"linha_digitavel": str(), "pix": str(),
		}, "valor"),
		"Cliente": object(M{
			"nome": str(), "cpfcnpj": str(), "contrato_status_display": str(),
			"valor_check_out": number(), "faturas": arr(ref("Fatura")),
		}),
		"ContratoOption": object(M{
			"id_cliente": str(), "label": str(), "endereco": str(), "status": str(),
		}, "id_cliente", "label"),
		// ClienteResult is a single object whose `needs_selection` flag drives a
		// oneOf: when true the customer has multiple contracts and `options` holds
		// the choices (the frontend renders a contract selector and re-queries with
		// id_cliente); otherwise `cliente` is populated.
		"ClienteResult": M{
			"oneOf":       []any{ref("ClienteFound"), ref("ClienteNeedsSelection")},
			"description": "When needs_selection is true, render a contract selector from options[].id_cliente; otherwise use cliente.",
		},
		"ClienteFound": object(M{
			"needs_selection": M{"type": "boolean", "const": false},
			"cliente":         ref("Cliente"),
		}, "needs_selection", "cliente"),
		"ClienteNeedsSelection": object(M{
			"needs_selection": M{"type": "boolean", "const": true},
			"options":         arr(ref("ContratoOption")),
		}, "needs_selection", "options"),
		"Plano": object(M{
			"nome": str(), "valor": number(), "velocidade": str(), "descricao": str(),
		}, "nome"),
		"PlanosResult": object(M{"data": arr(ref("Plano"))}),
		"Empresa": object(M{
			"nome": str(), "cnpj": str(), "telefone": str(), "email": str(),
			"endereco": str(), "site": str(),
		}),
		"Liberacao": object(M{"liberado": boolean(), "protocolo": str(), "liberado_ate": str(), "msg": str()}),
		"Chamado":   object(M{"protocolo": str(), "msg": str()}),

		// ── contacts ───────────────────────────────────────────────────────────
		"ContactExternalID": object(M{
			"channel":     contactIdentityChannelEnum(),
			"external_id": str(),
		}),
		"Contact": object(M{
			"id": str(), "tenant_id": str(), "name": str(), "phones": arr(str()),
			"document": str(), "email": str(), "external_ids": arr(ref("ContactExternalID")),
			"tags": tagIDArray(), "notes": str(),
			"avatar_attachment_id": describedStr("Attachment id of the contact's avatar (write it via PATCH)."),
			"avatar_url":           describedStr("Read-only, derived: a short-lived signed URL loadable directly in <img src> (no Authorization). Present only when the avatar exists and is ready. Do not cache long-term."),
			"created_at":           dateTime(), "updated_at": dateTime(),
		}),
		"CreateContactRequest": object(M{
			"name": str(),
			"phones": arr(describedStr("Phone number; normalized to E.164 on write (default region BR). " +
				"Invalid numbers are rejected with 400 validation_error.")),
			"document":     describedStr("CPF (11 digits) or CNPJ (14 digits); validated by check digits and stored digits-only (no mask)."),
			"email":        describedStr("Email address; format-validated and stored lowercased."),
			"external_ids": arr(ref("ContactExternalID")), "tags": tagIDArray(), "notes": str(),
			"avatar_attachment_id": describedStr("Attachment id (image, status=ready, same tenant) to use as avatar; uploaded via the avatar upload-url flow. Invalid -> 400 validation_error."),
		}, "name"),
		"UpdateContactRequest": object(M{
			"name": str(),
			"phones": arr(describedStr("Phone number; normalized to E.164 on write (default region BR). " +
				"Invalid numbers are rejected with 400 validation_error.")),
			"document":     describedStr("CPF (11 digits) or CNPJ (14 digits); validated by check digits and stored digits-only (no mask)."),
			"email":        describedStr("Email address; format-validated and stored lowercased."),
			"external_ids": arr(ref("ContactExternalID")), "tags": tagIDArray(), "notes": str(),
			"avatar_attachment_id": describedStr("Attachment id (image, status=ready, same tenant) to use as avatar; empty string clears it. Invalid -> 400 validation_error."),
		}),

		// ── webhooks ───────────────────────────────────────────────────────────
		"Webhook": object(M{
			"id": str(), "tenant_id": str(), "name": str(), "url": str(),
			"events": arr(webhookEventEnum()), "scopes": arr(str()), "has_secret": boolean(), "enabled": boolean(),
			"rate_limit_per_minute": integer(), "created_by": str(), "created_at": dateTime(), "updated_at": dateTime(),
		}),
		"WebhookCreated": object(M{
			"id": str(), "tenant_id": str(), "name": str(), "url": str(),
			"events": arr(webhookEventEnum()), "scopes": arr(str()), "has_secret": boolean(), "enabled": boolean(),
			"rate_limit_per_minute": integer(), "created_at": dateTime(), "updated_at": dateTime(),
			"secret": str(),
		}),
		"CreateWebhookRequest": object(M{
			"name": str(), "url": str(), "events": arr(webhookEventEnum()),
			"scopes": arr(str()), // sector ids; empty = all sectors
			"secret": str(), "enabled": boolean(), "rate_limit_per_minute": integer(),
		}, "url", "events"),
		"UpdateWebhookRequest": object(M{
			"name": str(), "url": str(), "events": arr(webhookEventEnum()), "scopes": arr(str()),
			"enabled": boolean(), "rate_limit_per_minute": integer(),
		}),
		"WebhookDelivery": object(M{
			"id": str(), "webhook_id": str(), "event": webhookEventEnum(), "payload": ref("WebhookEnvelope"), "status": str(),
			"attempts": integer(), "last_error": str(), "next_retry_at": dateTime(),
			"created_at": dateTime(), "updated_at": dateTime(),
		}),
		// WebhookEnvelope is the exact JSON body delivered (and HMAC-signed) for
		// every event. data is the event payload: a conversation object for the
		// conversation_* events, a message object for message_created, etc.
		"WebhookEnvelope": object(M{
			"id": str(), "event": webhookEventEnum(), "created_at": dateTime(), "data": freeObject(),
		}),

		// ── copilot ────────────────────────────────────────────────────────────
		"CopilotConfig": object(M{
			"id": str(), "tenant_id": str(), "provider": str(), "model": str(), "has_key": boolean(),
			"base_url": str(), "temperature": number(), "max_tokens": integer(),
			"allow_customer_data": boolean(), "allow_financial_data": boolean(), "allow_monitoring_data": boolean(),
			"human_approval_required": boolean(), "enabled": boolean(),
			"created_at": dateTime(), "updated_at": dateTime(),
		}),
		"SaveCopilotConfigRequest": object(M{
			"provider": enum("openai", "anthropic", "gemini", "mistral", "deepseek", "perplexity"),
			"model":    str(), "api_key": str(), "base_url": str(), "temperature": number(), "max_tokens": integer(),
			"allow_customer_data": boolean(), "allow_financial_data": boolean(), "allow_monitoring_data": boolean(),
			"human_approval_required": boolean(), "enabled": boolean(),
		}),
		// CopilotAssistant is a named assistant (many per tenant). It reuses the
		// tenant CopilotConfig for provider/key/policies and adds routing: the
		// channel types it serves and an optional pinned ISP profile. With an ISP
		// profile the backend exposes the SMSNET tools to the model and injects the
		// ISP config server-side; without one, no ISP tools are offered.
		"CopilotAssistant": object(M{
			"id": str(), "tenant_id": str(), "name": str(),
			"channel_ids":    describedArr(str(), "Ids of the specific ChannelConnections this assistant serves (not types)."),
			"isp_profile_id": describedStr("Pinned providerhub ISP profile id, or empty for no ISP tools."),
			"enabled":        boolean(),
			"created_at":     dateTime(), "updated_at": dateTime(),
		}),
		"CreateCopilotAssistantRequest": object(M{
			"name": str(), "channel_ids": withDesc(arr(str()), "Ids of ChannelConnections to serve; each must exist for the tenant."),
			"isp_profile_id": describedStr("Optional providerhub ISP profile id to pin; must exist."),
			"enabled":        describedBool("Defaults to true when omitted."),
		}, "name"),
		"UpdateCopilotAssistantRequest": object(M{
			"name": str(), "channel_ids": arr(str()), "isp_profile_id": str(), "enabled": boolean(),
		}),
		"SuggestReplyRequest": object(M{"conversation_id": str(), "instruction": str()}, "conversation_id"),
		"SummarizeRequest":    object(M{"conversation_id": str()}, "conversation_id"),
		"ClassifyRequest":     object(M{"conversation_id": str(), "categories": arr(str())}, "conversation_id", "categories"),
		"NextActionRequest":   object(M{"conversation_id": str()}, "conversation_id"),
		"ProposedAction":      object(M{"approval_id": str(), "server": str(), "tool": str(), "args": freeObject()}),
		"CopilotResult": object(M{
			"action":   enum("suggest_reply", "summarize", "classify", "next_action"),
			"provider": str(), "model": str(), "text": str(), "categories": arr(str()),
			"tokens_input": integer(), "tokens_output": integer(), "estimated_cost": number(),
			"requires_approval": boolean(), "proposed_actions": arr(ref("ProposedAction")),
		}),

		// ── mcp ────────────────────────────────────────────────────────────────
		"McpServer": object(M{
			"id": str(), "tenant_id": str(), "name": str(), "transport": enum("streamable_http"),
			"base_url": str(), "auth_header": str(), "has_auth": boolean(), "kind": enum("read", "write"),
			"enabled": boolean(), "created_at": dateTime(), "updated_at": dateTime(),
		}),
		"CreateMcpServerRequest": object(M{
			"name": str(), "base_url": str(), "auth_header": str(), "auth_token": str(), "kind": enum("read", "write"),
		}, "name", "base_url", "kind"),
		"UpdateMcpServerRequest": object(M{
			"name": str(), "base_url": str(), "auth_header": str(), "auth_token": str(), "kind": enum("read", "write"), "enabled": boolean(),
		}),
		"McpTool": object(M{
			"server_id": str(), "server_name": str(), "name": str(), "description": str(),
			"schema": freeObject(), "write": boolean(),
		}),
		"McpToolList":    object(M{"tools": arr(ref("McpTool"))}),
		"RunToolRequest": object(M{"server_id": str(), "tool": str(), "args": freeObject()}, "server_id", "tool"),
		"DecideRequest":  object(M{"approve": boolean(), "reason": str()}, "approve"),
		"McpApproval": object(M{
			"id": str(), "tenant_id": str(), "conversation_id": str(), "server_id": str(), "server_name": str(),
			"tool": str(), "args": freeObject(), "status": enum("pending", "approved", "rejected", "executed", "failed"),
			"proposed_by": str(), "decided_by": str(), "reason": str(), "result": str(), "error": str(),
			"created_at": dateTime(), "decided_at": dateTime(),
		}),
		"McpRunResult": object(M{
			"executed": boolean(), "result": str(), "approval": ref("McpApproval"), "tool": str(), "write": boolean(),
		}),

		// ── conversationtools ──────────────────────────────────────────────────
		"Tag":                      object(M{"id": str(), "tenant_id": str(), "name": str(), "color": str(), "description": str(), "enabled": boolean(), "created_at": dateTime(), "updated_at": dateTime()}),
		"CreateTagRequest":         object(M{"name": str(), "color": str(), "description": str(), "enabled": boolean()}, "name"),
		"UpdateTagRequest":         object(M{"name": str(), "color": str(), "description": str(), "enabled": boolean()}),
		"CannedResponse":           object(M{"id": str(), "tenant_id": str(), "sector_ids": arr(str()), "global": boolean(), "shortcut": str(), "title": str(), "body": str(), "enabled": boolean(), "created_at": dateTime(), "updated_at": dateTime()}),
		"CreateCannedRequest":      object(M{"sector_ids": arr(str()), "shortcut": str(), "title": str(), "body": str(), "enabled": boolean()}, "shortcut", "body"),
		"UpdateCannedRequest":      object(M{"sector_ids": arr(str()), "shortcut": str(), "title": str(), "body": str(), "enabled": boolean()}),
		"CloseReason":              object(M{"id": str(), "tenant_id": str(), "name": str(), "requires_note": boolean(), "enabled": boolean(), "created_at": dateTime(), "updated_at": dateTime()}),
		"CreateCloseReasonRequest": object(M{"name": str(), "requires_note": boolean(), "enabled": boolean()}, "name"),
		"UpdateCloseReasonRequest": object(M{"name": str(), "requires_note": boolean(), "enabled": boolean()}),

		// ── businesshours ──────────────────────────────────────────────────────
		"Holiday": object(M{"id": str(), "tenant_id": str(), "date": str(), "name": str(),
			"scope":       enum("all_channels", "channels"),
			"channel_ids": describedArr(str(), "Channel ids the holiday applies to when scope is \"channels\"; empty/absent for \"all_channels\"."),
			"recurring":   boolean(), "created_at": dateTime(), "updated_at": dateTime()}),
		"CreateHolidayRequest": object(M{"date": str(), "name": str(),
			"channel_ids": describedArr(str(), "Restrict the holiday to these channels; empty applies it to all channels."),
			"recurring":   boolean()}, "date", "name"),
		"UpdateHolidayRequest": object(M{"date": str(), "name": str(),
			"channel_ids": describedArr(str(), "Replace the channel scope; empty applies the holiday to all channels."),
			"recurring":   boolean()}),
		"BusinessHoursInterval": object(M{
			"start": describedStr(`Local opening time "HH:MM" (inclusive).`),
			"end":   describedStr(`Local closing time "HH:MM" (exclusive). Must be after start — intervals never cross midnight. For an overnight shift (e.g. 22:00–05:30) model it as two days: end the first day at "24:00" and start the next day at "00:00" for a contiguous span.`),
		}, "start", "end"),
		"BusinessHoursDay": object(M{
			"day":       describedInt("Weekday, 0=Sunday..6=Saturday. A day absent (or with no intervals) is closed."),
			"intervals": describedArr(ref("BusinessHoursInterval"), "Open intervals for the day, e.g. a morning and an afternoon window split by lunch. Must not overlap."),
		}, "day"),
		"BusinessHours": withDesc(object(M{
			"timezone": describedStr(`IANA timezone the schedule is evaluated in (e.g. "America/Sao_Paulo"). Defaults to UTC. "Open now?" is resolved in this timezone, not the server's.`),
			"weekly":   describedArr(ref("BusinessHoursDay"), "Per-weekday open intervals. An empty/absent document means always open (24/7)."),
		}), "A channel's weekly business hours. Lives on the ChannelConnection (each connection of the same type can have its own). An empty object means the channel is always open."),
		"BusinessStatus": object(M{
			"channel_id": str(), "open": boolean(), "reason": enum("open", "outside_hours", "holiday", "unconfigured"),
			"timezone": str(), "local_time": dateTime(), "holiday_name": str(),
			"today_intervals": arr(str()),
		}),

		// ── sla ────────────────────────────────────────────────────────────────
		"SLAPolicy": object(M{
			"id": str(), "tenant_id": str(), "name": str(), "sector_ids": arr(str()), "priority": str(), "channel": str(),
			"first_response_target_seconds": integer(), "resolution_target_seconds": integer(),
			"business_hours_only": boolean(), "warning_threshold_percent": integer(), "pause_on_waiting_customer": boolean(),
			"enabled": boolean(), "created_at": dateTime(), "updated_at": dateTime(),
		}),
		"CreateSLAPolicyRequest": object(M{
			"name": str(), "sector_ids": arr(str()), "priority": str(), "channel": str(),
			"first_response_target_seconds": integer(), "resolution_target_seconds": integer(),
			"business_hours_only": boolean(), "warning_threshold_percent": integer(), "pause_on_waiting_customer": boolean(),
			"enabled": boolean(),
		}, "name"),
		"UpdateSLAPolicyRequest": object(M{
			"name": str(), "sector_ids": arr(str()), "priority": str(), "channel": str(),
			"first_response_target_seconds": integer(), "resolution_target_seconds": integer(),
			"business_hours_only": boolean(), "warning_threshold_percent": integer(), "pause_on_waiting_customer": boolean(),
			"enabled": boolean(),
		}),
		"SLATracking": object(M{
			"id": str(), "conversation_id": str(), "policy_id": str(), "status": str(),
			"first_response_due_at": dateTime(), "resolution_due_at": dateTime(),
			"first_response_at": dateTime(), "resolved_at": dateTime(),
			"first_response_breached": boolean(), "resolution_breached": boolean(),
			"first_response_warned": boolean(), "resolution_warned": boolean(),
			"created_at": dateTime(), "updated_at": dateTime(),
		}),

		// ── notifications ──────────────────────────────────────────────────────
		"Notification":             object(M{"id": str(), "type": str(), "title": str(), "body": str(), "link": str(), "read": boolean(), "created_at": dateTime(), "read_at": dateTime()}),
		"NotificationPreferences":  object(M{"email_by_type": boolMap()}),
		"UpdatePreferencesRequest": object(M{"email_by_type": boolMap()}),
		"MarkAllReadResult":        object(M{"updated": integer()}),

		// ── csat ───────────────────────────────────────────────────────────────
		"Survey": object(M{
			"id": str(), "tenant_id": str(), "name": str(), "scale": str(), "question_text": str(),
			"send_on": str(), "sector_ids": arr(str()), "delay_seconds": integer(), "enabled": boolean(),
			"created_at": dateTime(), "updated_at": dateTime(),
		}),
		"CreateSurveyRequest": object(M{"name": str(), "scale": str(), "question_text": str(), "sector_ids": arr(str()), "delay_seconds": integer(), "enabled": boolean()}, "name", "question_text"),
		"UpdateSurveyRequest": object(M{"name": str(), "scale": str(), "question_text": str(), "sector_ids": arr(str()), "delay_seconds": integer(), "enabled": boolean()}),
		"CSATResponse": object(M{
			"id": str(), "conversation_id": str(), "contact_id": str(), "survey_id": str(), "agent_id": str(),
			"score": integer(), "comment": str(), "sent_at": dateTime(), "responded_at": dateTime(), "status": str(), "created_at": dateTime(),
		}),
		"SubmitCSATRequest": object(M{"score": integer(), "comment": str()}, "score"),

		// ── search ─────────────────────────────────────────────────────────────
		"ConversationHit": object(M{"id": str(), "contact_id": str(), "channel": str(), "sector_id": str(), "status": str(), "assigned_to": str(), "priority": str(), "tags": tagIDArray(), "last_message_at": dateTime(), "updated_at": dateTime()}),
		"ContactHit":      object(M{"id": str(), "name": str(), "phone": str(), "document": str()}),
		"MessageHit":      object(M{"id": str(), "conversation_id": str(), "sender_type": str(), "direction": str(), "text": str(), "created_at": dateTime()}),

		// ── reports ────────────────────────────────────────────────────────────
		"Bucket":     object(M{"key": str(), "count": integer()}),
		"DateCount":  object(M{"date": str(), "count": integer()}),
		"AgentStat":  object(M{"agent_id": str(), "conversations": integer(), "avg_resolution_seconds": number()}),
		"SectorStat": object(M{"sector_id": str(), "conversations": integer()}),
		"ReportOverview": object(M{
			"from": dateTime(), "to": dateTime(), "total_conversations": integer(),
			"open_by_status": arr(ref("Bucket")), "messages": integer(),
			"first_response_avg_seconds": number(), "resolution_avg_seconds": number(),
			"csat_avg_score": number(), "csat_response_rate": number(),
			"sla_first_response_breach_rate": number(), "sla_resolution_breach_rate": number(),
		}),
		"ReportConversations": object(M{
			"daily": arr(ref("DateCount")), "by_status": arr(ref("Bucket")), "by_sector": arr(ref("Bucket")),
			"messages_by_channel": arr(ref("Bucket")), "closed_by_reason": arr(ref("Bucket")),
		}),
		"ReportAgents":       object(M{"agents": arr(ref("AgentStat"))}),
		"ReportSectors":      object(M{"sectors": arr(ref("SectorStat"))}),
		"ReportCopilot":      object(M{"total_calls": integer(), "by_action": arr(ref("Bucket")), "tokens_input": integer(), "tokens_output": integer(), "estimated_cost": number()}),
		"ReportSLA":          object(M{"tracked": integer(), "first_response_breached": integer(), "resolution_breached": integer(), "met": integer(), "first_response_breach_rate": number(), "resolution_breach_rate": number()}),
		"ReportCSAT":         object(M{"sent": integer(), "responded": integer(), "expired": integer(), "avg_score": number(), "response_rate": number(), "by_score": arr(ref("Bucket"))}),
		"ReportExportResult": object(M{"report": str(), "format": enum("json", "csv"), "filename": str(), "download_url": str(), "expires_at": dateTime(), "bytes": integer()}),

		// ── privacy / audit ────────────────────────────────────────────────────
		"PrivacyExport": object(M{
			"id": str(), "contact_id": str(), "status": str(), "download_url": str(),
			"expires_at": dateTime(), "error": str(), "created_at": dateTime(), "completed_at": dateTime(),
		}),
		"RetentionPolicy": object(M{
			"messages_days": integer(), "closed_conversations_days": integer(), "technical_logs_days": integer(),
			"audit_logs_days": integer(), "notifications_days": integer(), "updated_at": dateTime(),
		}),
		"UpdateRetentionRequest": object(M{
			"messages_days": integer(), "closed_conversations_days": integer(), "technical_logs_days": integer(),
			"audit_logs_days": integer(), "notifications_days": integer(),
		}),
		"AuditLog": object(M{
			"id": str(), "actor_id": str(), "actor_type": str(), "action": str(), "resource_type": str(),
			"resource_id": str(), "ip": str(), "user_agent": str(), "data": freeObject(), "created_at": dateTime(),
		}),

		// ── attachments ────────────────────────────────────────────────────────
		"AvatarUploadTarget": object(M{
			"owner_type": enum("contacts", "users"),
			"owner_id":   str(),
		}, "owner_type", "owner_id"),
		"CreateUploadURLRequest": object(M{
			"conversation_id": describedStr("Conversation this attachment belongs to. Provide this OR avatar (a conversation-less avatar upload), not both."),
			"filename":        str(),
			"content_type":    str(),
			"size":            integer(),
			"avatar":          ref("AvatarUploadTarget"),
		}, "filename", "content_type", "size"),
		"UploadURLResponse":        object(M{"attachment_id": str(), "storage_key": str(), "upload_url": str(), "method": str(), "headers": stringMap(), "expires_at": dateTime()}),
		"ConfirmAttachmentRequest": object(M{"attachment_id": str(), "message_id": str()}, "attachment_id"),
		"AttachmentRecord": object(M{
			"id": str(), "conversation_id": str(), "message_id": str(), "filename": str(), "content_type": str(),
			"size": integer(), "storage_provider": str(), "status": str(), "download_url": str(), "created_at": dateTime(),
		}),
	}
}
