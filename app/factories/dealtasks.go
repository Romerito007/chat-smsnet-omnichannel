package factories

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	crmentity "github.com/romerito007/chat-smsnet-omnichannel/domain/crmsettings/entity"
	crmservice "github.com/romerito007/chat-smsnet-omnichannel/domain/crmsettings/service"
	tcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/dealtasks/contracts"
	taskservice "github.com/romerito007/chat-smsnet-omnichannel/domain/dealtasks/service"
	tlcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/dealtimeline/contracts"
	tlentity "github.com/romerito007/chat-smsnet-omnichannel/domain/dealtimeline/entity"
	timelineservice "github.com/romerito007/chat-smsnet-omnichannel/domain/dealtimeline/service"
	iamservice "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/service"
	dealrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/deals"
	dealtaskrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/dealtasks"
	taskctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/dealtasks"
)

// DealTaskService builds the deal-task service: CRUD of seller follow-ups, gated by
// the tenant's tasks toggle, constrained by the deal's visibility, recording
// task_created/task_completed on the timeline and notifying the assignee.
func DealTaskService(c *container.Container) *taskservice.Service {
	svc := taskservice.New(dealtaskrepo.New(c.Mongo.DB), taskDealLookup{repo: dealrepo.New(c.Mongo.DB)}, clock)
	svc.SetModuleGate(taskGate{settings: CRMSettingsService(c)})
	svc.SetAgentChecker(taskAgentChecker{users: UserService(c)})
	svc.SetDirectory(UserService(c))
	svc.SetTimeline(taskTimelineWriter{tl: DealTimelineService(c)})
	svc.SetNotifier(NotificationEnqueuer(c))
	return svc
}

// DealTaskController builds the deal-task controller (per-deal + consolidated).
func DealTaskController(c *container.Container) *taskctl.Controller {
	return taskctl.NewController(DealTaskService(c))
}

// taskDealLookup adapts the deals repository to the tasks' DealLookup port.
type taskDealLookup struct{ repo *dealrepo.Repository }

// Deal implements tcontracts.DealLookup.
func (a taskDealLookup) Deal(ctx context.Context, dealID string) (*tcontracts.DealRef, error) {
	d, err := a.repo.FindByID(ctx, dealID)
	if err != nil {
		return nil, err
	}
	return &tcontracts.DealRef{TenantID: d.TenantID, SectorID: d.SectorID, AssignedTo: d.AssignedTo}, nil
}

// taskGate adapts the crmsettings service to the tasks' ModuleGate port.
type taskGate struct{ settings *crmservice.Service }

// TasksEnabled implements tcontracts.ModuleGate.
func (g taskGate) TasksEnabled(ctx context.Context) (bool, error) {
	return g.settings.IsModuleEnabled(ctx, crmentity.ModuleTasks)
}

// taskAgentChecker adapts the IAM user service to the tasks' AgentChecker port.
type taskAgentChecker struct{ users *iamservice.UserService }

// AgentExists implements tcontracts.AgentChecker.
func (a taskAgentChecker) AgentExists(ctx context.Context, userID string) (bool, error) {
	_, err := a.users.Get(ctx, userID)
	return existsFromErr(err)
}

// taskTimelineWriter adapts the timeline service to the tasks' TimelineWriter port.
type taskTimelineWriter struct{ tl *timelineservice.Service }

// Record implements tcontracts.TimelineWriter.
func (a taskTimelineWriter) Record(ctx context.Context, ev tcontracts.TimelineEvent) {
	a.tl.Record(ctx, tlcontracts.RecordEvent{
		DealID: ev.DealID, Kind: tlentity.Kind(ev.Kind), ActorID: ev.ActorID, Data: ev.Data,
	})
}
