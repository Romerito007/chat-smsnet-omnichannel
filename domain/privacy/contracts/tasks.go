// Package contracts holds the privacy domain's ports (FileStore, ExportEnqueuer),
// async task payloads and command inputs.
package contracts

// ExportTask is the privacy.export job payload: assemble a contact's data bundle
// into a file and attach a temporary signed URL to the export request.
type ExportTask struct {
	TenantID  string `json:"tenant_id"`
	RequestID string `json:"request_id"`
	ActorID   string `json:"actor_id"`
}
