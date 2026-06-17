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
		"UpdateProfileRequest":  object(M{"name": str(), "avatar_attachment_id": str(), "preferences": userPreferencesSchema()}),
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
			"preferences":          userPreferencesSchema(),
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
			"tenant_id": str(), "user_id": str(),
			"name":       describedStr("Read-only, derived: the agent's display name, resolved in batch so the dashboard renders the agent instead of a raw user id. Empty when unresolved."),
			"avatar_url": describedStr("Read-only, derived: the agent's short-lived signed avatar URL (loadable in <img src>, no Authorization). Empty when the agent has no ready avatar."),
			"status":     str(), "current_load": integer(),
			"max_concurrent_chats": integer(), "last_seen_at": dateTime(),
		}),
		"SetStatusRequest": object(M{"user_id": str(), "status": str()}, "status"),

		// ── conversations / messages ───────────────────────────────────────────
		"Conversation": object(M{
			"id": str(), "tenant_id": str(), "contact_id": str(), "channel": str(),
			"channel_id": describedStr("Id of the specific ChannelConnection this conversation belongs to (e.g. which of several same-type WhatsApp numbers). Empty only for conversations created without one."),
			"sector_id":  str(), "queue_id": str(), "status": conversationStatusEnum(), "assigned_to": str(),
			"priority":          str(),
			"protocol":          describedStr("Per-tenant/year protocol number (\"2026-000123\") assigned when the conversation is opened on a channel with uses_protocol=true; empty otherwise. Searchable via GET /v1/conversations?protocol=."),
			"tags":              tagIDArray(),
			"custom_attributes": customAttributesObject(),
			"last_message_at":   dateTime(),
			"unread_count":      integer(), "last_read_at": dateTime(),
			"created_at": dateTime(), "updated_at": dateTime(), "closed_at": dateTime(),
			"last_message":       ref("LastMessage"),
			"contact_name":       describedStr("Read-only, derived: the conversation contact's display name, resolved in batch so the inbox renders the row without a per-contact fetch. Empty when the contact is absent."),
			"contact_avatar_url": describedStr("Read-only, derived: the conversation contact's short-lived signed avatar URL (loadable in <img src>, no Authorization), resolved in batch for the inbox. Empty when the contact has no ready avatar."),
			"agent_name":         describedStr("Read-only, derived: the assignee's display name, resolved in batch. Empty when the conversation is unassigned."),
			"agent_avatar_url":   describedStr("Read-only, derived: the assignee's short-lived signed avatar URL (no Authorization), resolved in batch. Empty when unassigned or no ready avatar."),
			"whatsapp_window":    ref("WhatsAppWindow"),
		}),
		"LastMessage": object(M{
			"preview": str(), "sender_type": str(), "message_type": str(), "created_at": dateTime(),
		}),
		"WhatsAppWindow": withDesc(object(M{
			"open":       describedBool("True when the customer messaged in the last 24h (free-form text/media is deliverable). False otherwise — only a template can be sent."),
			"expires_at": describedStr("When the 24h window closes = last inbound customer message + 24h (ISO 8601). Absent when the customer has never messaged."),
		}, "open"), "Read-only, derived server-side. Present ONLY on WhatsApp channels (omitted for other channel types — the 24h window does not apply). The front uses it to warn \"outside the window, use a template\"; the backend does NOT block free-form sends (the provider rejects and reports failed via delivery receipt)."),
		"AssignableAgent": object(M{
			"id": str(), "name": str(),
			"status":       describedStr("Presence status: online | available | busy | away | paused | lunch | training | offline (\"offline\" when the agent has no live presence). The list ALWAYS includes offline agents of the sector — they are selectable for manual assign/transfer (the operator sees the status and chooses). Render a presence badge; do NOT hide or disable offline agents."),
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
			"contact_id": str(),
			"channel_id": describedStr("Required id of the specific ChannelConnection the conversation leaves through. The channel TYPE is derived from this connection — the client does NOT send a channel type."),
			"sector_id":  str(), "queue_id": str(),
			"assigned_to": str(), "priority": str(), "tags": tagIDArray(),
		}, "contact_id", "channel_id"),
		"UpdateConversationRequest": object(M{
			"sector_id": str(), "queue_id": str(), "status": conversationStatusEnum(), "assigned_to": str(),
			"priority": str(), "tags": tagIDArray(),
			"custom_attributes": customAttributesObject(),
		}),
		"Attachment": object(M{"id": str(), "url": describedStr("Signed, time-boxed channel-media URL (GET /v1/channel-media/{token}) loadable DIRECTLY in <img>/<audio>/<video> src — no Authorization header, no per-image access check. It is a bearer URL (the token grants access to that one object until it expires) and is regenerated on each read; for an access-checked download use GET /v1/attachments/{id}/download."), "content_type": str(), "filename": str(), "size": integer()}),
		"Message": object(M{
			"id": str(), "conversation_id": str(), "sender_type": str(), "sender_id": str(),
			"direction": str(), "message_type": str(), "text": str(),
			"attachments": arr(ref("Attachment")), "template": ref("MessageTemplate"),
			"contacts": arr(ref("MessageContact")), "location": ref("MessageLocation"),
			"interactive": ref("MessageInteractive"), "interactive_reply": ref("MessageInteractiveReply"),
			"metadata":        freeObject(),
			"delivery_status": str(), "external_message_id": str(),
			"created_at": dateTime(), "edited_at": dateTime(),
		}),
		"MessageTemplate": object(M{
			"template_id": describedStr("Opaque integrator template id (the chat does not interpret it)."),
			"params":      withDesc(M{"type": "object", "additionalProperties": M{"type": "string"}}, "Filled named variables (key→value). On a template send the chat validates that every declared variable is present and no extras; the resolved display text is server-side only and never sent to the integrator."),
		}, "template_id"),
		// message_type=contact: 1..10 vCards (the gateway maps these 1:1 to the
		// WhatsApp contacts[] block). See docs/message-types-v2.md.
		"MessageContact": object(M{
			"name": object(M{
				"formatted": describedStr("Display name (maps to Meta name.formatted_name)."),
				"first":     str(), "last": str(),
			}, "formatted"),
			"phones": describedArr(object(M{
				"phone": describedStr("E.164 phone, required."),
				"type":  describedStr("Free hint: CELL|HOME|WORK|…"),
				"wa_id": describedStr("WhatsApp id when known."),
			}, "phone"), "At least one phone is required."),
			"emails":       arr(object(M{"email": str(), "type": str()})),
			"organization": object(M{"company": str(), "title": str()}),
		}, "name", "phones"),
		// message_type=location: a single geographic point (maps to Meta location).
		"MessageLocation": object(M{
			"latitude":  describedNumber("Required, [-90, 90]."),
			"longitude": describedNumber("Required, [-180, 180]."),
			"name":      str(), "address": str(),
		}, "latitude", "longitude"),
		// message_type=interactive: an OUTBOUND menu (reply buttons or list). v1
		// supports a text-only header. Maps to the Meta interactive.{button|list} block.
		"MessageInteractive": object(M{
			"kind":   describedStr("'buttons' or 'list'."),
			"header": describedStr("Optional text header (≤60 chars; v1 is text-only)."),
			"body":   describedStr("Required. ≤1024 for buttons, ≤4096 for list."),
			"footer": describedStr("Optional (≤60)."),
			"buttons": describedArr(object(M{
				"id":    describedStr("Stable, unique within the message (≤256)."),
				"title": describedStr("≤20 chars."),
			}, "id", "title"), "Reply buttons when kind=buttons (max 3)."),
			"button": describedStr("List open-label (kind=list, required, ≤20)."),
			"sections": describedArr(object(M{
				"title": describedStr("Optional section title (≤24)."),
				"rows": describedArr(object(M{
					"id":          describedStr("Stable, unique within the message (≤200)."),
					"title":       describedStr("≤24 chars."),
					"description": describedStr("Optional (≤72)."),
				}, "id", "title"), "Section rows."),
			}, "rows"), "List sections when kind=list (max 10 sections, ≤10 rows total)."),
		}, "kind", "body"),
		// message_type=interactive_reply: the INBOUND customer choice on a menu.
		"MessageInteractiveReply": object(M{
			"kind":               describedStr("'button' or 'list'."),
			"id":                 describedStr("The chosen button/row id (stable — branch automations on this)."),
			"title":              str(),
			"description":        describedStr("Only for list replies."),
			"context_message_id": describedStr("Internal id of the menu message this reply answers (resolved from Meta context.id)."),
		}, "id"),
		"SendMessageRequest": object(M{
			"message_type": str(), "text": str(), "attachments": arr(ref("Attachment")),
			"template":    withDesc(ref("MessageTemplate"), "Required when message_type=template (WhatsApp). The template_id must exist on the conversation's channel."),
			"contacts":    describedArr(ref("MessageContact"), "Required when message_type=contact (1..10 vCards)."),
			"location":    withDesc(ref("MessageLocation"), "Required when message_type=location."),
			"interactive": withDesc(ref("MessageInteractive"), "Required when message_type=interactive (outbound menu)."),
			"metadata":    freeObject(),
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
			"base_url":  describedStr("Outbound URL: when set, the channel auto-manages a webhook subscription (signed with the channel secret) that delivers conversation + message events to it. outbound_url is an accepted alias on write."),
			"auth_type": str(), "has_secret": boolean(), "has_inbound_token": boolean(),
			"business_hours":       ref("BusinessHours"),
			"out_of_hours_message": describedStr("Auto-sent to the customer ONCE when a NEW inbound conversation opens while the channel is OUTSIDE business hours (the conversation still enters normally — this is only a notice). Empty = disabled."),
			"enabled":              boolean(),
			"uses_protocol":        describedBool("When true, inbound opens a NEW conversation with a NEW protocol number for this channel; a closed last conversation is NOT reopened. When false (default), a closed last conversation is reopened and no protocol is assigned."),
			"whatsapp_templates":   arr(ref("WhatsAppTemplate")),
			"created_at":           dateTime(), "updated_at": dateTime(),
		}),
		"ChannelCreated": object(M{
			"id": str(), "tenant_id": str(), "type": str(), "name": str(), "status": str(),
			"base_url": str(), "auth_type": str(), "has_secret": boolean(), "has_inbound_token": boolean(),
			"business_hours": ref("BusinessHours"), "out_of_hours_message": str(),
			"enabled": boolean(), "uses_protocol": boolean(), "whatsapp_templates": arr(ref("WhatsAppTemplate")),
			"created_at": dateTime(), "updated_at": dateTime(),
			"inbound_token": str(), "outbound_secret": str(),
		}),
		"CreateChannelRequest": object(M{
			"type": str(), "name": str(),
			"base_url":     describedStr("Outbound URL the channel delivers events to (auto-managed webhook). outbound_url is an accepted alias."),
			"outbound_url": str(),
			"auth_type":    str(), "secret": str(), "outbound_secret": str(),
			"business_hours": ref("BusinessHours"), "uses_protocol": boolean(),
			"out_of_hours_message": describedStr("Optional. Auto-sent to the customer when a NEW conversation opens outside business hours. Empty/omitted = disabled."),
			"whatsapp_templates":   arr(ref("WhatsAppTemplate")),
		}, "type"),
		"UpdateChannelRequest": object(M{
			"name": str(), "status": str(), "base_url": str(), "outbound_url": str(),
			"auth_type": str(), "secret": str(), "outbound_secret": str(),
			"business_hours":       ref("BusinessHours"),
			"out_of_hours_message": describedStr("When set, replaces the out-of-hours auto-message (empty string disables it)."),
			"enabled":              boolean(), "uses_protocol": boolean(),
			"whatsapp_templates": withDesc(arr(ref("WhatsAppTemplate")), "Render-only mirror of the integrator's WhatsApp templates. Replaces the whole list (the integrator pushes the full set)."),
		}),
		"WhatsAppTemplate": object(M{
			"id":       describedStr("Opaque integrator template id; stored and echoed verbatim, never interpreted."),
			"name":     str(),
			"language": str(),
			"category": str(),
			"body": object(M{
				"text":      describedStr("Body text with {{name}} placeholders the chat substitutes for the display string."),
				"variables": arr(object(M{"key": str(), "label": str(), "example": str()}, "key")),
			}, "text"),
			"header":  object(M{"type": str(), "text": str()}),
			"buttons": arr(object(M{"type": str(), "text": str(), "url": str()})),
			"footer":  str(),
		}, "id", "name"),
		"DeliveryReceiptRequest": object(M{
			"inbound_token": describedStr("Channel integration token (alternative to the X-Inbound-Token header)."),
			"message_id":    describedStr("The chat's own message id (as delivered in the message_created webhook). The receipt is correlated by this id."),
			"status":        enum("delivered", "read", "failed"),
		}, "message_id", "status"),
		"ContactIdentityRequest": object(M{
			"inbound_token": describedStr("Channel integration token (alternative to the X-Inbound-Token header)."),
			"contact_id":    describedStr("The chat's contact id (as delivered in the message_created webhook contact.id). The identity is added to THIS contact."),
			"channel":       describedStr("Identity channel slug (e.g. \"whatsapp\"). Optional — defaults to the {channel} path type."),
			"external_id":   describedStr("The verified external identifier to persist, e.g. a WhatsApp JID \"554499088478@s.whatsapp.net\"."),
		}, "contact_id", "external_id"),
		"InboundMessageRequest": object(M{
			"inbound_token": str(), "tenant_key": str(), "integration_key": str(), "webhook_verify_token": str(),
			"external_message_id": str(), "external_contact_id": str(), "contact_name": str(),
			"contact_phone": str(), "contact_document": str(), "channel": str(), "text": str(),
			"attachments": arr(ref("Attachment")),
			"contacts":    describedArr(ref("MessageContact"), "Set when the customer shares contact(s) (message_type=contact)."),
			"location":    withDesc(ref("MessageLocation"), "Set when the customer shares a location (message_type=location)."),
			"interactive_reply": withDesc(object(M{
				"kind": describedStr("'button' or 'list'."), "id": str(), "title": str(), "description": str(),
				"context_external_id": describedStr("Meta context.id — the external id of the menu message the chat sent; resolved to the internal menu id."),
			}, "id"), "Set when the customer answers an interactive menu (message_type=interactive_reply)."),
			"metadata": freeObject(), "timestamp": integer(),
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
		"RotatedInboundToken":   object(M{"inbound_token": str()}, "inbound_token"),
		"RotatedOutboundSecret": object(M{"outbound_secret": describedStr("The new outbound HMAC secret, revealed once. The previous secret stops working — the integrator must verify our outbound signature with this value, and the channel's managed webhook now signs with it.")}, "outbound_secret"),
		"TestResult":            object(M{"ok": boolean(), "external_message_id": str(), "error": str()}),

		// ── automation rules (Chatwoot-style trigger/conditions/actions engine) ──
		// An AutomationRule reacts to a conversation/message lifecycle event, matches
		// AND-conditions against the conversation/contact (or the message text), and
		// runs an ordered list of actions. Anti-loop: actions run as origin=automation
		// so the events they emit never re-trigger rules; message/attachment actions
		// are also fused per conversation. Each action reads only its own param.
		"AutomationRule": object(M{
			"id": str(), "tenant_id": str(), "name": str(), "description": str(),
			"event":      automationRuleEventEnum(),
			"enabled":    boolean(),
			"priority":   describedInt("Firing order among rules on the same event (ascending; lower first). Ties break by created_at then id."),
			"conditions": arr(ref("AutomationRuleCondition")),
			"actions":    arr(ref("AutomationRuleAction")),
			"health":     ref("AutomationRuleHealth"),
			"created_at": dateTime(), "updated_at": dateTime(),
		}),
		"AutomationRuleHealth": object(M{
			"ok":           describedBool("False when an action references a deleted agent/tag/sector/webhook (the action is skipped at runtime — skipped_missing_ref)."),
			"missing_refs": arr(object(M{"action_index": integer(), "kind": enum("agent", "sector", "tag", "attachment", "webhook"), "id": str()})),
		}),
		"AutomationRuleCondition": object(M{
			"field":    enum("status", "channel", "assigned_agent_id", "sector_id", "queue_id", "priority", "tags", "contact_phone", "message_content"),
			"operator": withDesc(enum("equal_to", "not_equal_to", "contains", "does_not_contain"), "Allowed operators depend on the field: scalar fields → equal_to/not_equal_to; tags & message_content → contains/does_not_contain; contact_phone → equal_to/contains."),
			"value":    describedStr("Comparison value; for tags it is a single tag id; for message_content the substring to match (case-insensitive) against the triggering message text."),
		}, "field", "operator", "value"),
		"AutomationRuleAction": object(M{
			"type": withDesc(enum(
				"send_webhook", "send_message", "send_attachment",
				"assign_agent", "assign_team", "remove_assigned_agent", "remove_assigned_team",
				"add_tag", "remove_tag", "change_priority",
				"resolve_conversation", "open_conversation", "mark_pending",
			), "Action kind. Each reads only its own param (below)."),
			"webhook_id":    describedStr("send_webhook: id of a registered webhook (/v1/webhooks)."),
			"text":          describedStr("send_message: the message text (sent as System Automation)."),
			"attachment_id": describedStr("send_attachment: an uploaded, ready attachment id (same tenant)."),
			"agent_id":      describedStr("assign_agent: the agent (user) id."),
			"sector_id":     describedStr("assign_team: the sector (team) id."),
			"tag_id":        describedStr("add_tag/remove_tag: the tag id."),
			"priority":      describedStr("change_priority: one of low|normal|high|urgent."),
		}, "type"),
		"CreateAutomationRuleRequest": object(M{
			"name": str(), "description": str(),
			"event":      automationRuleEventEnum(),
			"enabled":    describedBool("Defaults to true when omitted."),
			"priority":   describedInt("Firing order among rules on the same event (ascending). Default 0."),
			"conditions": withDesc(arr(ref("AutomationRuleCondition")), "AND-combined. Empty = match every occurrence of the event."),
			"actions":    arr(ref("AutomationRuleAction")),
		}, "name", "event", "actions"),
		"UpdateAutomationRuleRequest": object(M{
			"name": str(), "description": str(), "event": automationRuleEventEnum(), "enabled": boolean(),
			"priority":   integer(),
			"conditions": arr(ref("AutomationRuleCondition")), "actions": arr(ref("AutomationRuleAction")),
		}),
		"RuleEvaluationLog": object(M{
			"id": str(), "rule_id": str(), "event": automationRuleEventEnum(), "conversation_id": str(),
			"action_type":   describedStr("The action this row is about (empty for rule-level skips)."),
			"status":        withDesc(enum("action_enqueued", "skipped_dedup", "skipped_automation", "skipped_stale", "skipped_budget", "skipped_missing_ref", "error"), "Per-action outcome. skipped_automation = anti-loop origin suppression; skipped_stale = conditions no longer matched the live conversation; skipped_budget = per-conversation message fuse; skipped_missing_ref = referenced entity deleted."),
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
			"transports":                      describedArr(enum("http", "mcp"), "SMSNET surfaces this profile enables. http = ProviderHub gateway (required for manual search); mcp = CONSULTAS/OPERACOES."),
			"is_default":                      describedBool("True for the tenant's default profile (at most one)."),
			"actions":                         describedArr(enum("cliente", "planos", "empresa", "liberacao", "chamado"), "The ISP catalog universe — every action this profile's ISP could perform."),
			"enabled_actions":                 describedArr(enum("cliente", "planos", "empresa", "liberacao", "chamado"), "The subset of actions this profile actually OFFERS — gates both the copilot tools and the manual /external/* path. Defaults to all on create; the tenant unchecks what it doesn't want."),
			"usa_pegar_fatura_atrasada":       describedBool("How many invoices /cliente returns: false (default) = ALL invoices; true = ONLY the oldest (overdue) one. Most tenants list all; enable only for an oldest-invoice billing flow."),
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
			"transports":                      describedArr(enum("http", "mcp"), "REQUIRED. SMSNET surfaces to enable (at least one): http (gateway) and/or mcp."),
			"enabled_actions":                 describedArr(enum("cliente", "planos", "empresa", "liberacao", "chamado"), "Operations this profile offers (subset of the ISP catalog). Omit → all catalog actions enabled; the tenant unchecks what it doesn't want."),
			"is_default":                      describedBool("Make this the default profile. The first profile of a tenant is always the default."),
			"usa_pegar_fatura_atrasada":       describedBool("How many invoices /cliente returns: false (default) = ALL invoices; true = ONLY the oldest (overdue) one. Most tenants list all; enable only for an oldest-invoice billing flow."),
			"usa_extrair_linha_digitavel_pdf": boolean(),
			"timeout_ms":                      integer(),
			"enabled":                         describedBool("Defaults to true when omitted."),
		}, "label", "isp_type", "transports"),
		"UpdateISPProfileRequest": object(M{
			"label":                           str(),
			"isp_type":                        ispTypeStr(),
			"credentials":                     withDesc(stringMap(), "Replaces the credentials; keys must match the catalog for the (possibly new) isp_type 1:1. Write-only."),
			"transports":                      describedArr(enum("http", "mcp"), "When set, replaces the enabled transports (still must be non-empty)."),
			"enabled_actions":                 describedArr(enum("cliente", "planos", "empresa", "liberacao", "chamado"), "When set, replaces the offered actions (subset of the ISP catalog)."),
			"usa_pegar_fatura_atrasada":       describedBool("How many invoices /cliente returns: false (default) = ALL invoices; true = ONLY the oldest (overdue) one. Most tenants list all; enable only for an oldest-invoice billing flow."),
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
			"valor": number(), "vencimento": describedStr("Due date, dd/mm/aaaa (e.g. \"15/03/2026\")."),
			"vencida":     describedBool("Read-only, derived server-side: true when overdue (vencimento before today in America/Sao_Paulo, compared by day). The front renders state/colour from this — it does not re-decide the rule."),
			"dias_atraso": describedInt("Read-only, derived: whole days overdue (0 when not overdue or the date is unparseable). Due today → 0; due yesterday → 1."),
			"link":        str(), "linha_digitavel": str(), "pix": str(),
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
			"custom_attributes":    customAttributesObject(),
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
			"custom_attributes":    customAttributesObject(),
		}),

		// ── WhatsApp groups (Domain 1: configuration) ───────────────────────────
		"Group": object(M{
			"id": str(), "tenant_id": str(), "channel_id": str(),
			"group_jid":    describedStr("The group's WhatsApp JID (e.g. \"1203...@g.us\"). Unique per tenant; the idempotency key for the sync."),
			"name":         str(),
			"description":  str(),
			"participants": describedArr(str(), "Raw participant identifiers (metadata, NOT contacts)."),
			"group_admins": describedArr(str(), "Raw admin identifiers (metadata, NOT contacts)."),
			"company_id":   str(), "whatsapp_wid": str(), "owner_name": str(), "owner_jid": str(),
			"activated":  boolean(),
			"attend":     describedStr("Whether the chat attends this group. Defaults to true on first sync; a re-sync never resets the operator's choice."),
			"synced_at":  dateTime(),
			"created_at": dateTime(), "updated_at": dateTime(),
		}),
		"UpdateGroupAttendRequest": object(M{
			"attend": withDesc(boolean(), "Mark the group to attend (true) or not (false). Required."),
		}, "attend"),
		"GroupSyncRequest": object(M{
			"channel_id": describedStr("The channel whose gateway should push its group list."),
		}, "channel_id"),
		"GroupBatchRequest": object(M{
			"inbound_token": describedStr("Channel integration token (alternative to the X-Inbound-Token header)."),
			"groups":        describedArr(ref("GroupBatchItem"), "The group-sync batch (≤2000 groups)."),
		}, "groups"),
		"GroupBatchItem": object(M{
			"groupId":      describedStr("The group's WhatsApp JID in the gateway's shape (mapped to group_jid). group_jid is also accepted."),
			"group_jid":    str(),
			"subject":      describedStr("The group's name in the gateway's shape (mapped to name). name is also accepted."),
			"name":         str(),
			"description":  str(),
			"participants": arr(str()),
			"group_admins": arr(str()),
			"admins":       describedArr(str(), "Alias of group_admins."),
			"company_id":   str(), "whatsapp_wid": str(), "owner_name": str(), "owner_jid": str(),
			"activated": boolean(),
		}),

		// ── webhooks ───────────────────────────────────────────────────────────
		"Webhook": object(M{
			"id": str(), "tenant_id": str(), "name": str(), "url": str(),
			"events": arr(webhookEventEnum()), "scopes": arr(str()), "has_secret": boolean(), "enabled": boolean(),
			"rate_limit_per_minute": integer(),
			"managed":               describedBool("True when this subscription is owned/kept in sync by a channel connection (created from the channel's outbound URL). Managed subscriptions are read-only here — edit/delete them through the channel, not the webhooks API."),
			"owned_by_channel_id":   describedStr("Id of the channel that manages this subscription (present only when managed)."),
			"created_by":            str(), "created_at": dateTime(), "updated_at": dateTime(),
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
		// every event. data is the event payload: a WebhookConversationData for the
		// conversation_* events, a WebhookMessageData for message_created/updated, etc.
		"WebhookEnvelope": object(M{
			"id": str(), "event": webhookEventEnum(), "created_at": dateTime(), "data": freeObject(),
		}),
		// WebhookIdentity is a contact's external identifier on a channel — the routing
		// key the channel gateway dials (e.g. the WhatsApp JID). The recipient of an
		// outbound message is resolved from contact.identities[channel == "whatsapp"].
		"WebhookIdentity": object(M{
			"channel":     describedStr("Channel of the identifier (whatsapp, telegram, instagram, …)."),
			"external_id": describedStr("The contact's id on that channel (e.g. the WhatsApp JID) — the gateway's routing key."),
		}),
		// WebhookContact is the recipient block embedded in message/conversation
		// webhooks so the gateway can route without a second call. No PII beyond
		// name/phone.
		"WebhookContact": object(M{
			"id": str(), "name": str(), "phone": str(),
			"identities":        arr(ref("WebhookIdentity")),
			"custom_attributes": freeObject(),
		}),
		// WebhookAgent is the sender block for an agent-authored message: id + name
		// only (deliberately no email/PII). Absent for customer/automation/system.
		"WebhookAgent": object(M{"id": str(), "name": str()}),
		// WebhookMessageData is the data of a message_created/message_updated event:
		// the message plus the enrichment blocks. contact is the recipient; agent is
		// present ONLY for an agent-authored message; conversation carries the
		// conversation's custom_attributes.
		"WebhookMessageData": object(M{
			"id": str(), "conversation_id": str(), "sender_type": str(), "sender_id": str(),
			"direction": str(), "message_type": str(), "text": str(),
			"attachments": arr(ref("Attachment")), "template": ref("MessageTemplate"),
			"internal": boolean(), "delivery_status": str(), "created_at": dateTime(), "edited_at": dateTime(),
			"contact": ref("WebhookContact"), "agent": ref("WebhookAgent"),
			"conversation": object(M{"custom_attributes": freeObject()}),
		}),
		// WebhookConversationData is the data of a conversation_* event: the
		// conversation plus custom_attributes, the recipient contact and the assigned
		// agent (null when unassigned, or for inbound where agents aren't resolved).
		"WebhookConversationData": object(M{
			"id": str(), "tenant_id": str(), "contact_id": str(), "channel": str(), "channel_id": str(),
			"sector_id": str(), "queue_id": str(), "status": str(), "assigned_to": str(),
			"priority": str(), "protocol": str(), "tags": arr(str()),
			"last_message_at": dateTime(), "unread_count": integer(), "last_read_at": dateTime(), "updated_at": dateTime(),
			"custom_attributes": freeObject(),
			"contact":           ref("WebhookContact"),
			"assigned_agent":    ref("WebhookAgent"),
		}),

		// ── copilot ────────────────────────────────────────────────────────────
		// CopilotConfig is the tenant's shared AI INFRASTRUCTURE only. Behavior
		// (privacy gates, sampling, persona) lives per-assistant on CopilotAssistant.
		"CopilotConfig": object(M{
			"id": str(), "tenant_id": str(), "provider": str(), "model": str(), "has_key": boolean(),
			"base_url": str(), "enabled": boolean(),
			"created_at": dateTime(), "updated_at": dateTime(),
		}),
		"SaveCopilotConfigRequest": object(M{
			"provider": enum("openai", "anthropic", "gemini", "mistral", "deepseek", "perplexity"),
			"model":    str(), "api_key": describedStr("New provider key; OMIT to keep the stored one, empty string to clear. Never returned."),
			"base_url": str(), "enabled": boolean(),
		}),
		// CopilotAssistant is a named assistant (many per tenant). It reuses the
		// tenant CopilotConfig for provider/model/key/base_url and adds routing: the
		// channels it serves and its EXTERNAL TOOL SOURCE — a pinned ISP profile
		// (SMSNET tools, injected server-side) XOR a custom MCP server (its tools
		// only), never both. With neither, no external tools are offered.
		"CopilotAssistant": object(M{
			"id": str(), "tenant_id": str(), "name": str(),
			"channel_ids":             describedArr(str(), "Ids of the specific ChannelConnections this assistant serves (not types)."),
			"isp_profile_id":          describedStr("Pinned providerhub ISP profile id (SMSNET tools). Mutually exclusive with mcp_server_id; empty for none."),
			"mcp_server_id":           describedStr("Pinned tenant MCP server id (its tools only). Mutually exclusive with isp_profile_id; empty for none. A server referenced here is the ONLY way it reaches the copilot."),
			"transport":               describedStr("SMSNET surface for the pinned ISP profile: http or mcp; must be one the profile enables. Empty when no ISP profile. Note: the copilot uses mcp; http is accepted but the copilot-over-http tool bridge is not built yet (phase 2)."),
			"allow_customer_data":     describedBool("Behavior gate: let the customer profile enter the prompt. (Financial/monitoring data are consulted on demand via ISP tools, never pre-injected — no gate.)"),
			"human_approval_required": describedBool("Require human approval for write tools (a proposed write always requires approval regardless)."),
			"write_modes":             withDesc(stringMap(), "Per-write-operation copilot mode, keyed by ISP write action (liberacao/chamado). Value: \"automatico\" (run without approval) or \"mediante_aprovacao\" (propose; default for any unset op). Affects ONLY the copilot — the human agent's actions always require approval."),
			"temperature":             describedNumber("Sampling temperature 0–2."),
			"max_tokens":              describedInt("Max output tokens (>0)."),
			"system_instructions":     describedStr("Free-text persona/conduct, APPENDED to the fixed action system prompt (never replaces it)."),
			"enabled":                 boolean(),
			"created_at":              dateTime(), "updated_at": dateTime(),
		}),
		"CreateCopilotAssistantRequest": object(M{
			"name": str(), "channel_ids": withDesc(arr(str()), "Ids of ChannelConnections to serve; each must exist for the tenant."),
			"isp_profile_id":          describedStr("Optional providerhub ISP profile id to pin; must exist. Mutually exclusive with mcp_server_id (both set → 422)."),
			"mcp_server_id":           describedStr("Optional tenant MCP server id to pin; must exist. Mutually exclusive with isp_profile_id (both set → 422)."),
			"transport":               describedStr("http|mcp; only with an isp_profile_id. Must be a transport the profile enables (else 422). Optional when the profile enables exactly one (that one is used); required when it enables both."),
			"allow_customer_data":     describedBool("Default false."),
			"human_approval_required": describedBool("Default false."),
			"write_modes":             withDesc(stringMap(), "Per-write-operation mode (keyed by ISP write action: liberacao/chamado): \"automatico\" or \"mediante_aprovacao\". Keys must be write actions ENABLED on the pinned ISP profile. Unset ops default to approval. Requires an isp_profile_id."),
			"temperature":             describedNumber("0–2; default 0.7 when omitted."),
			"max_tokens":              describedInt(">0; default 512 when omitted."),
			"system_instructions":     describedStr("Optional persona/conduct text."),
			"enabled":                 describedBool("Defaults to true when omitted."),
		}, "name"),
		"UpdateCopilotAssistantRequest": object(M{
			"name": str(), "channel_ids": arr(str()),
			"isp_profile_id":          describedStr("Mutually exclusive with mcp_server_id."),
			"mcp_server_id":           describedStr("Mutually exclusive with isp_profile_id."),
			"transport":               describedStr("http|mcp; validated against the pinned profile's enabled transports."),
			"allow_customer_data":     boolean(),
			"human_approval_required": boolean(), "temperature": describedNumber("0–2."), "max_tokens": describedInt(">0."),
			"write_modes":         withDesc(stringMap(), "When set, replaces the per-write-operation modes (keys = ISP write actions enabled on the pinned profile; values = automatico|mediante_aprovacao)."),
			"system_instructions": str(), "enabled": boolean(),
		}),
		"SuggestReplyRequest": object(M{"conversation_id": str(), "instruction": str()}, "conversation_id"),
		"AskRequest": object(M{
			"conversation_id": str(),
			"instruction":     describedStr("The agent's question in natural language (e.g. \"como está a fatura desse cliente?\")."),
			"history":         withDesc(arr(object(M{"role": enum("agent", "assistant"), "text": str()}, "role", "text")), "The prior agent↔assistant turns (oldest first), so the assistant keeps the thread. Ephemeral and front-managed; the backend does not persist it and caps it to the last few turns."),
		}, "conversation_id"),
		"SummarizeRequest":  object(M{"conversation_id": str()}, "conversation_id"),
		"ClassifyRequest":   object(M{"conversation_id": str(), "categories": arr(str())}, "conversation_id", "categories"),
		"NextActionRequest": object(M{"conversation_id": str()}, "conversation_id"),
		"ProposedAction":    object(M{"approval_id": str(), "server": str(), "tool": str(), "args": freeObject()}),
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
		// ── custom attributes ──────────────────────────────────────────────────
		"CustomAttributeDefinition": object(M{
			"id": str(), "tenant_id": str(), "key": str(), "label": str(), "description": str(),
			"type":       enum("text", "number", "boolean", "date", "list"),
			"applies_to": enum("contact", "conversation"),
			"options":    arr(str()), "regex": str(),
			"created_at": dateTime(), "updated_at": dateTime(),
		}),
		"CreateCustomAttributeRequest": object(M{
			"key":         describedStr("Unique key within (tenant, applies_to); immutable after creation."),
			"label":       str(),
			"description": str(),
			"type":        enum("text", "number", "boolean", "date", "list"),
			"applies_to":  enum("contact", "conversation"),
			"options":     describedArr(str(), "Required (non-empty) when type=list; rejected otherwise."),
			"regex":       describedStr("Optional validation pattern; only valid when type=text."),
		}, "key", "label", "type", "applies_to"),
		"UpdateCustomAttributeRequest": object(M{
			"label": str(), "description": str(),
			"options": arr(str()), "regex": str(),
		}),
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
		"Bucket":    object(M{"key": str(), "count": integer()}),
		"DateCount": object(M{"date": str(), "count": integer()}),
		"AgentStat": object(M{
			"agent_id":      str(),
			"name":          describedStr("Read-only, derived: the agent's display name, resolved in batch so the report renders the agent instead of a raw id. Empty when unresolved."),
			"avatar_url":    describedStr("Read-only, derived: the agent's short-lived signed avatar URL (no Authorization). Empty when the agent has no ready avatar."),
			"conversations": integer(), "avg_resolution_seconds": number(),
		}),
		"SectorStat": object(M{
			"sector_id":     str(),
			"name":          describedStr("Read-only, derived: the sector's display name, resolved in batch so the report renders the sector instead of a raw id. Empty for sector-less conversations or when unresolved."),
			"conversations": integer(),
		}),
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
		"ReportAutomation":   object(M{"total_evaluations": integer(), "by_status": arr(ref("Bucket")), "by_event": arr(ref("Bucket")), "by_action": arr(ref("Bucket"))}),
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
			"size": integer(), "storage_provider": str(), "status": str(),
			"download_url": describedStr("Signed, JWT-less channel-media URL (GET /v1/channel-media/{token}) renderable directly in <img>/<audio>/<video> src. Time-boxed bearer token, regenerated on each read."),
			"created_at":   dateTime(),
		}),
	}
}
