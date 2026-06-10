package shared

// Realtime topic builders. Topics are always tenant-scoped, following the
// convention "t:{tenant}:{scope}[:{id}]".

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
