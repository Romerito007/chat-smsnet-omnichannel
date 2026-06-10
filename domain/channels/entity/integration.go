// Package entity holds the channels domain aggregates: Integration (inbound
// credential/config) and InboundRecord (idempotency ledger).
package entity

import "time"

// Integration is a per-tenant channel inbound credential and configuration. The
// IntegrationKey is the public identifier the provider sends; Secret is used to
// validate the signature/token of inbound requests.
type Integration struct {
	ID                string
	TenantID          string
	Channel           string
	Name              string
	IntegrationKey    string
	Secret            string
	Enabled           bool
	AutomationEnabled bool
	// DefaultQueueID, when set, is where new conversations are enqueued if
	// automation is not enabled.
	DefaultQueueID string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
