package models

import "time"

// DealTask is the BSON document for a seller follow-up attached to a deal.
type DealTask struct {
	ID          string     `bson:"_id"`
	TenantID    string     `bson:"tenant_id"`
	DealID      string     `bson:"deal_id"`
	Title       string     `bson:"title"`
	Description string     `bson:"description,omitempty"`
	DueDate     *time.Time `bson:"due_date,omitempty"`
	AssignedTo  string     `bson:"assigned_to,omitempty"`
	Status      string     `bson:"status"`
	CompletedAt *time.Time `bson:"completed_at,omitempty"`
	CreatedBy   string     `bson:"created_by,omitempty"`
	CreatedAt   time.Time  `bson:"created_at"`
	UpdatedAt   time.Time  `bson:"updated_at"`
}
