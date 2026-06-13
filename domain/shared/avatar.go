package shared

import "context"

// AvatarURLResolver batch-resolves avatar attachment ids to short-lived, signed,
// JWT-less URLs that a browser can load directly in <img src> (no Authorization
// header). It is implemented by the attachments service (reusing the signed
// channel-media token) and consulted by the contacts and iam presenters so a list
// page renders avatars without a per-item request. Only ready, same-tenant image
// attachments resolve; others are absent from the returned map.
type AvatarURLResolver interface {
	SignedAvatarURLs(ctx context.Context, attachmentIDs []string) (map[string]string, error)
}

// DisplayCard is the resolved display info (name + short-lived signed avatar URL)
// for a related entity embedded in another payload — e.g. the contact and the
// assignee shown per row in the conversation inbox — so the client renders the
// row without a per-item fetch. AvatarURL is empty when there is no ready avatar.
type DisplayCard struct {
	Name      string
	AvatarURL string
}
