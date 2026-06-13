// Package entity holds the MCP domain aggregates: the per-tenant MCP server
// connection, the discovered tools, the human-approval record for write actions
// and the (payload-free) call log.
package entity

import "time"

// Transport is the MCP transport. Only streamable HTTP is supported.
type Transport string

const (
	TransportStreamableHTTP Transport = "streamable_http"
)

// Kind classifies a server by side effect: read servers expose query tools (no
// effect), write servers expose operation tools (side effects). The kind, not a
// per-tool flag, decides whether a tool requires human approval — so adding a new
// server is pure configuration.
type Kind string

const (
	KindRead  Kind = "read"
	KindWrite Kind = "write"
)

// Valid reports whether k is a known kind.
func (k Kind) Valid() bool { return k == KindRead || k == KindWrite }

// Well-known SMSNET MCP server names (CONSULTAS = read, OPERACOES = write). The
// host injects the ISP config{type+creds} into tool calls for these servers and
// gates them by the conversation's assistant ISP profile.
const (
	SMSNETConsultasName = "SMSNET_CONSULTAS"
	SMSNETOperacoesName = "SMSNET_OPERACOES"
)

// IsSMSNETServer reports whether a server name is one of the SMSNET MCP servers
// that require server-side ISP config injection.
func IsSMSNETServer(name string) bool {
	return name == SMSNETConsultasName || name == SMSNETOperacoesName
}

// ServerConnection is a per-tenant MCP server registration. AuthToken is held in
// plaintext in memory but stored encrypted at rest and never returned to clients.
type ServerConnection struct {
	ID         string
	TenantID   string
	Name       string
	Transport  Transport
	BaseURL    string
	AuthHeader string // header name to carry the token (e.g. "Authorization")
	AuthToken  string // secret value; encrypted at rest, masked in responses
	Kind       Kind
	Enabled    bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Tool is a tool discovered on a server, annotated with its server and whether it
// is a write tool (derived from the server kind). Names are never hard-coded.
type Tool struct {
	ServerID    string
	ServerName  string
	Name        string
	Description string
	Schema      map[string]any
	Write       bool
}

// ApprovalStatus is the lifecycle of a proposed write action.
type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "pending"
	ApprovalApproved ApprovalStatus = "approved"
	ApprovalRejected ApprovalStatus = "rejected"
	ApprovalExecuted ApprovalStatus = "executed"
	ApprovalFailed   ApprovalStatus = "failed"
)

// Approval is a write action awaiting an agent's explicit confirmation. It is
// created by the copilot (AI proposal) or by a manual write run; it executes only
// after Decide(approve=true). Args are the (non-secret) tool arguments shown on
// the confirmation card.
type Approval struct {
	ID             string
	TenantID       string
	ConversationID string
	ServerID       string
	ServerName     string
	Tool           string
	Args           map[string]any
	Status         ApprovalStatus
	ProposedBy     string // "ai" or a user id (manual)
	DecidedBy      string
	Reason         string
	Result         string
	Error          string
	CreatedAt      time.Time
	DecidedAt      *time.Time
}

// CallStatus is the outcome recorded in a call log.
type CallStatus string

const (
	CallSuccess CallStatus = "success"
	CallError   CallStatus = "error"
)

// CallLog is a minimal, payload-free record of one tool call: who, where, which
// tool, the outcome and latency — never the arguments or result content.
type CallLog struct {
	ID             string
	TenantID       string
	UserID         string
	ConversationID string
	ServerID       string
	ServerName     string
	Tool           string
	Write          bool
	Status         CallStatus
	LatencyMs      int64
	ErrorSummary   string
	CreatedAt      time.Time
}
