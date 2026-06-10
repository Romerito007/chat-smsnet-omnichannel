// Package providerhub is the HTTP client for the standardized provider API. It
// is strictly consulta sob demanda: no sync, no real-time ingestion, no
// persistence of the full external payload. Callers fetch what they need, when
// they need it, and map only the fields the domain requires.
//
// Placeholder in the foundation; the client is implemented when a domain needs
// it (uses infra/http_client).
package providerhub
