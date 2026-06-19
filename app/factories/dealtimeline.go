package factories

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	crmentity "github.com/romerito007/chat-smsnet-omnichannel/domain/crmsettings/entity"
	crmservice "github.com/romerito007/chat-smsnet-omnichannel/domain/crmsettings/service"
	dcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/deals/contracts"
	tlcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/dealtimeline/contracts"
	tlentity "github.com/romerito007/chat-smsnet-omnichannel/domain/dealtimeline/entity"
	timelineservice "github.com/romerito007/chat-smsnet-omnichannel/domain/dealtimeline/service"
	dealrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/deals"
	tlrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/dealtimeline"
	tlctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/dealtimeline"
)

// DealTimelineService builds the deal-timeline service: the feed reader + comment
// writer + the automatic-event Writer used by the deals service. It is wired with the
// deal lookup (visibility), the crmsettings gate (the timeline_enabled toggle) and the
// agent/pipeline directories (actor + stage names).
func DealTimelineService(c *container.Container) *timelineservice.Service {
	svc := timelineservice.New(tlrepo.New(c.Mongo.DB), dealLookup{repo: dealrepo.New(c.Mongo.DB)}, clock)
	svc.SetModuleGate(timelineGate{settings: CRMSettingsService(c)})
	svc.SetDirectories(UserService(c), PipelineService(c))
	return svc
}

// DealTimelineController builds the timeline feed/comment controller.
func DealTimelineController(c *container.Container) *tlctl.Controller {
	return tlctl.NewController(DealTimelineService(c))
}

// dealTimelineWriter adapts the timeline service to the deals TimelineWriter port,
// translating the deals event into the timeline record input.
type dealTimelineWriter struct{ tl *timelineservice.Service }

// Record implements dcontracts.TimelineWriter.
func (a dealTimelineWriter) Record(ctx context.Context, ev dcontracts.TimelineEvent) {
	a.tl.Record(ctx, tlcontracts.RecordEvent{
		DealID: ev.DealID, Kind: tlentity.Kind(ev.Kind), ActorID: ev.ActorID, Data: ev.Data,
	})
}

// dealLookup adapts the deals repository to the timeline's DealLookup port (the deal's
// tenant/sector/assignee/pipeline for the visibility check + stage names).
type dealLookup struct{ repo *dealrepo.Repository }

// Deal implements tlcontracts.DealLookup.
func (a dealLookup) Deal(ctx context.Context, dealID string) (*tlcontracts.DealRef, error) {
	d, err := a.repo.FindByID(ctx, dealID)
	if err != nil {
		return nil, err
	}
	return &tlcontracts.DealRef{
		TenantID: d.TenantID, SectorID: d.SectorID, AssignedTo: d.AssignedTo, PipelineID: d.PipelineID,
	}, nil
}

// timelineGate adapts the crmsettings service to the timeline's ModuleGate port.
type timelineGate struct{ settings *crmservice.Service }

// TimelineEnabled implements tlcontracts.ModuleGate.
func (g timelineGate) TimelineEnabled(ctx context.Context) (bool, error) {
	return g.settings.IsModuleEnabled(ctx, crmentity.ModuleTimeline)
}
