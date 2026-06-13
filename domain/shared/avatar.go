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
