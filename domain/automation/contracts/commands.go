package contracts

// CreateIntegration registers an automation (external flow) integration.
type CreateIntegration struct {
	Name      string
	BaseURL   string
	AuthType  string
	Secret    string
	TimeoutMs int
}

// UpdateIntegration carries optional fields; nil pointers mean "leave unchanged".
type UpdateIntegration struct {
	Name      *string
	BaseURL   *string
	AuthType  *string
	Secret    *string
	Enabled   *bool
	TimeoutMs *int
}

// Callback is the payload the external flow posts to apply a result.
type Callback struct {
	ExternalRunID string         `json:"external_run_id"`
	Decision      *Decision      `json:"decision"`
	Output        map[string]any `json:"output"`
	Error         string         `json:"error"`
}
