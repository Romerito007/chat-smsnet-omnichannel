package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	maintenancesvc "github.com/romerito007/chat-smsnet-omnichannel/domain/maintenance"
	tenantrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/tenant/repository"
	maintenancerepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/maintenance"
	tenantmongo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/tenant"
)

// TenantRepository builds the tenant repository (used by periodic jobs to fan
// work out across tenants).
func TenantRepository(c *container.Container) tenantrepo.TenantRepository {
	return tenantmongo.New(c.Mongo.DB)
}

// MaintenanceService builds the maintenance service (reports snapshot + audit
// compaction).
func MaintenanceService(c *container.Container) *maintenancesvc.Service {
	return maintenancesvc.NewService(maintenancerepo.New(c.Mongo.DB), clock)
}
