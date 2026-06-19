package shared

// Realtime topic builders. Topics are always tenant-scoped, following the
// convention "t:{tenant}:{scope}[:{id}]".

// TopicTenant is the tenant-wide broadcast topic (e.g. queue.updated).
func TopicTenant(tenantID TenantID) string {
	return "t:" + tenantID + ":tenant"
}

// TopicPresence is the per-tenant presence fan-out topic (team presence board).
func TopicPresence(tenantID TenantID) string {
	return "t:" + tenantID + ":presence"
}

// TopicUser is the per-user topic (personal notifications, own presence).
func TopicUser(tenantID TenantID, userID ID) string {
	return "t:" + tenantID + ":user:" + userID
}

// TopicConversation is the per-conversation topic (messages, status changes).
func TopicConversation(tenantID TenantID, conversationID ID) string {
	return "t:" + tenantID + ":conversation:" + conversationID
}

// TopicInbox is the per-sector inbox topic (new/updated conversations for a team).
func TopicInbox(tenantID TenantID, sectorID ID) string {
	return "t:" + tenantID + ":inbox:" + sectorID
}

// TopicUnassigned is the per-tenant room for queued/unassigned conversations that
// carry NO sector. Only all-scope agents (who can see the whole queue in the REST
// inbox) auto-join it, so realtime updates for sector-less conversations reach
// exactly the agents the inbox visibility would show them to — and never leak to
// sector-scoped agents.
func TopicUnassigned(tenantID TenantID) string {
	return "t:" + tenantID + ":unassigned"
}

// InboxTopicsFor returns the inbox rooms that must receive a conversation's
// list-level update (conversation.created/updated/assigned/…), mirroring the REST
// inbox visibility (visibleTo):
//   - a sectored conversation → its sector room;
//   - a sector-less (queued/unassigned) conversation → the unassigned room
//     (all-scope agents) plus, when assigned, the assignee's own user room.
//
// The per-conversation room (subscribed on open) is handled separately by callers.
func InboxTopicsFor(tenantID TenantID, sectorID, assignedTo ID) []string {
	if sectorID != "" {
		return []string{TopicInbox(tenantID, sectorID)}
	}
	out := []string{TopicUnassigned(tenantID)}
	if assignedTo != "" {
		out = append(out, TopicUser(tenantID, assignedTo))
	}
	return out
}

// DealTopicsFor returns the rooms that must receive a deal's Kanban update
// (deal.created/updated/stage_changed), mirroring the deal visibility (all-scope OR
// assigned-to-me OR my-sector) onto the rooms clients already auto-join:
//   - the unassigned room, which ONLY all-scope agents join → reaches every manager
//     who can see all deals;
//   - the deal's sector room, when it has a sector → reaches that team;
//   - the assignee's own user room, when assigned → reaches the seller even when the
//     deal carries no sector (or the seller is scoped to a different one).
//
// No client subscribes to anything new: these are the same default rooms used for
// conversation/inbox/notification fan-out.
func DealTopicsFor(tenantID TenantID, sectorID, assignedTo ID) []string {
	out := []string{TopicUnassigned(tenantID)}
	if sectorID != "" {
		out = append(out, TopicInbox(tenantID, sectorID))
	}
	if assignedTo != "" {
		out = append(out, TopicUser(tenantID, assignedTo))
	}
	return out
}
