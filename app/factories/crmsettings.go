package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	crmservice "github.com/romerito007/chat-smsnet-omnichannel/domain/crmsettings/service"
	crmrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/crmsettings"
	crmctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/crmsettings"
)

// CRMSettingsService builds the per-tenant CRM-settings service (the optional-module
// toggles), wired to the audit trail. Future CRM modules consult its IsModuleEnabled
// checkpoint before serving.
func CRMSettingsService(c *container.Container) *crmservice.Service {
	svc := crmservice.New(crmrepo.New(c.Mongo.DB), clock)
	svc.SetAuditor(AuditService(c))
	return svc
}

// CRMSettingsController builds the CRM-settings controller.
func CRMSettingsController(c *container.Container) *crmctl.Controller {
	return crmctl.NewController(CRMSettingsService(c))
}
