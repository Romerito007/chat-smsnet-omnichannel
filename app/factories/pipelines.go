package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	pipelineservice "github.com/romerito007/chat-smsnet-omnichannel/domain/pipelines/service"
	pipelinerepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/pipelines"
	pipelinectl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/pipelines"
)

// PipelineService builds the sales-pipeline service (the configurable Kanban funnel),
// wired to the audit trail. The deal checker (refuse deleting a non-empty stage) is
// wired in a later block when deals exist.
func PipelineService(c *container.Container) *pipelineservice.Service {
	svc := pipelineservice.New(pipelinerepo.New(c.Mongo.DB), clock)
	svc.SetAuditor(AuditService(c))
	return svc
}

// PipelineController builds the pipeline management controller.
func PipelineController(c *container.Container) *pipelinectl.Controller {
	return pipelinectl.NewController(PipelineService(c))
}
