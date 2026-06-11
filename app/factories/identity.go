package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	authservice "github.com/romerito007/chat-smsnet-omnichannel/domain/auth/service"
	iamservice "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	tenantservice "github.com/romerito007/chat-smsnet-omnichannel/domain/tenant/service"
	authrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/auth"
	iamrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/iam"
	tenantrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/tenant"
	authctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/auth"
	iamctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/iam"
	tenantctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/tenant"
)

// clock is the shared system clock used by services in production wiring.
var clock = shared.SystemClock{}

// UserService builds the IAM user service.
func UserService(c *container.Container) *iamservice.UserService {
	svc := iamservice.NewUserService(iamrepo.NewUserRepository(c.Mongo.DB), c.Hasher, clock)
	svc.SetAuditor(AuditService(c))
	return svc
}

// RoleService builds the IAM role service.
func RoleService(c *container.Container) *iamservice.RoleService {
	svc := iamservice.NewRoleService(iamrepo.NewRoleRepository(c.Mongo.DB), clock)
	svc.SetAuditor(AuditService(c))
	return svc
}

// AuthService builds the auth service.
func AuthService(c *container.Container) *authservice.Service {
	svc := authservice.New(
		iamrepo.NewUserRepository(c.Mongo.DB),
		iamrepo.NewRoleRepository(c.Mongo.DB),
		authrepo.NewRefreshTokenRepository(c.Mongo.DB),
		c.Hasher,
		c.Tokens,
		clock,
	)
	svc.SetAuditor(AuditService(c))
	return svc
}

// TenantService builds the tenant service.
func TenantService(c *container.Container) *tenantservice.Service {
	return tenantservice.New(tenantrepo.New(c.Mongo.DB), clock)
}

// AuthController builds the auth controller (login/refresh/logout/me).
func AuthController(c *container.Container) *authctl.Controller {
	return authctl.NewController(AuthService(c), UserService(c))
}

// UserController builds the IAM user controller.
func UserController(c *container.Container) *iamctl.UserController {
	return iamctl.NewUserController(UserService(c))
}

// RoleController builds the IAM role controller.
func RoleController(c *container.Container) *iamctl.RoleController {
	return iamctl.NewRoleController(RoleService(c))
}

// TenantController builds the tenant controller.
func TenantController(c *container.Container) *tenantctl.Controller {
	return tenantctl.NewController(TenantService(c))
}
