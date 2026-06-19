package factories

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	crmentity "github.com/romerito007/chat-smsnet-omnichannel/domain/crmsettings/entity"
	crmservice "github.com/romerito007/chat-smsnet-omnichannel/domain/crmsettings/service"
	productservice "github.com/romerito007/chat-smsnet-omnichannel/domain/products/service"
	productrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/products"
	productctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/products"
)

// ProductService builds the product-catalog service, gated by the tenant's products
// toggle and wired to the audit trail.
func ProductService(c *container.Container) *productservice.Service {
	svc := productservice.New(productrepo.New(c.Mongo.DB), clock)
	svc.SetModuleGate(productsGate{settings: CRMSettingsService(c)})
	svc.SetAuditor(AuditService(c))
	return svc
}

// ProductController builds the product-catalog controller.
func ProductController(c *container.Container) *productctl.Controller {
	return productctl.NewController(ProductService(c))
}

// productsGate adapts the crmsettings service to the products' ModuleGate port.
type productsGate struct{ settings *crmservice.Service }

// ProductsEnabled implements products contracts.ModuleGate.
func (g productsGate) ProductsEnabled(ctx context.Context) (bool, error) {
	return g.settings.IsModuleEnabled(ctx, crmentity.ModuleProducts)
}
