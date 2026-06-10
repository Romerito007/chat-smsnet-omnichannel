// Package models contains the BSON struct definitions persisted in MongoDB. They
// live in infra (not domain) so storage tags never leak into business code. One
// <domain>_models.go file per domain; shared_models.go holds the common base.
package models

import "time"

// Base is embedded by every persisted document. It carries the tenant scope and
// audit timestamps required by the architecture ("toda entidade respeita
// tenant_id").
type Base struct {
	ID        string    `bson:"_id"`
	TenantID  string    `bson:"tenant_id"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}
