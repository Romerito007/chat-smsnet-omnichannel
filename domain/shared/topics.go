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
