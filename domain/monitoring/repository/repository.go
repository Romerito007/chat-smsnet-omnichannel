// Package repository declares the monitoring persistence contracts: the config
// and the minimal query log (no external payloads).
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/monitoring/entity"
)

// ConfigRepository persists the monitoring integration config (one active per
// tenant; the service uses the enabled one).
type ConfigRepository interface {
	Create(ctx context.Context, c *entity.MonitoringIntegrationConfig) error
	Update(ctx context.Context, c *entity.MonitoringIntegrationConfig) error
	FindEnabled(ctx context.Context) (*entity.MonitoringIntegrationConfig, error)
}

// QueryLogRepository persists the minimal technical query log.
type QueryLogRepository interface {
	Create(ctx context.Context, l *entity.MonitoringQueryLog) error
}
