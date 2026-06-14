package contracts

import chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"

// CreateConnection registers a channel connection.
type CreateConnection struct {
	Type              chentity.Type
	Name              string
	BaseURL           string
	AuthType          chentity.AuthType
	Secret            string
	DefaultSectorID   string
	BusinessHours     map[string]any
	AutomationEnabled bool
}

// UpdateConnection carries optional fields; nil pointers mean "leave unchanged".
type UpdateConnection struct {
	Name              *string
	Status            *chentity.Status
	BaseURL           *string
	AuthType          *chentity.AuthType
	Secret            *string
	DefaultSectorID   *string
	BusinessHours     *map[string]any
	Enabled           *bool
	AutomationEnabled *bool
}

// TestResult is the outcome of POST /v1/channels/{id}/test.
type TestResult struct {
	OK                bool   `json:"ok"`
	ExternalMessageID string `json:"external_message_id,omitempty"`
	Error             string `json:"error,omitempty"`
}
