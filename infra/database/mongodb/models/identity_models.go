package models

import "time"

// Tenant is the top-level isolation boundary. Every other document references
// its id via Base.TenantID.
type Tenant struct {
	ID        string    `bson:"_id"`
	Name      string    `bson:"name"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

// Role is a named permission bundle within a tenant.
type Role struct {
	Base        `bson:",inline"`
	Name        string   `bson:"name"`
	Permissions []string `bson:"permissions"`
}

// User is an operator/agent account scoped to a tenant.
type User struct {
	Base  `bson:",inline"`
	Email string   `bson:"email"`
	Name  string   `bson:"name"`
	Roles []string `bson:"roles"`
}
