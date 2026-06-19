// Package service holds the sales-deal business logic: the opportunity CRUD, the
// manual stage move (the Kanban drag-and-drop), conversation linking and lost
// marking. Automation/copilot come in later blocks.
package service

import (
	"context"
	"strings"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/deals/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/deals/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/deals/repository"
	pipelineentity "github.com/romerito007/chat-smsnet-omnichannel/domain/pipelines/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Service manages sales deals.
type Service struct {
	repo          repository.DealRepository
	pipelines     contracts.PipelineLookup
	conversations contracts.ConversationLookup
	contacts      contracts.ContactChecker
	tags          contracts.TagCatalog
	auditor       shared.Auditor
	notifier      shared.Notifier
	audience      contracts.DealAudience
	publisher     shared.EventPublisher
	timeline      contracts.TimelineWriter
	products      contracts.ProductLookup
	productsGate  contracts.ProductsGate
	clock         shared.Clock
}

// New builds the service. The conversation/contact lookups are optional (a nil
// conversation lookup disables CreateFromConversation; a nil contact checker skips
// contact validation).
func New(repo repository.DealRepository, pipelines contracts.PipelineLookup, clock shared.Clock) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Service{repo: repo, pipelines: pipelines, auditor: shared.NoopAuditor{}, notifier: shared.NoopNotifier{}, publisher: shared.NoopPublisher{}, clock: clock}
}

// SetAuditor wires the audit trail (optional).
func (s *Service) SetAuditor(a shared.Auditor) {
	if a != nil {
		s.auditor = a
	}
}

// SetConversationLookup wires the conversation resolver (CreateFromConversation).
func (s *Service) SetConversationLookup(c contracts.ConversationLookup) {
	if c != nil {
		s.conversations = c
	}
}

// SetContactChecker wires the contact-existence guard.
func (s *Service) SetContactChecker(c contracts.ContactChecker) {
	if c != nil {
		s.contacts = c
	}
}

// SetTagCatalog wires the tag catalog (the /v1/tags registry shared with
// conversations) used to validate tag ids on a deal. Optional: when unset, tags are
// accepted as-is.
func (s *Service) SetTagCatalog(t contracts.TagCatalog) {
	if t != nil {
		s.tags = t
	}
}

// SetNotifier wires the in-app notifier used to alert a deal's seller when an
// automation moves their card. Optional: when unset, no notification is sent.
func (s *Service) SetNotifier(n shared.Notifier) {
	if n != nil {
		s.notifier = n
	}
}

// SetPublisher wires the realtime publisher so the Kanban reacts live to deal
// create/update/stage-change. Optional: when unset, no realtime event is emitted.
func (s *Service) SetPublisher(p shared.EventPublisher) {
	if p != nil {
		s.publisher = p
	}
}

// SetAudience wires the sector-agents resolver, so an automated move on an unowned
// deal still notifies the sector's team. Optional: without it, an unowned (but
// sectored) deal's move is not notified.
func (s *Service) SetAudience(a contracts.DealAudience) {
	if a != nil {
		s.audience = a
	}
}

// SetTimeline wires the deal-timeline writer so every relevant action records a
// user-facing event (create/stage move/won/lost/value & assignee edits). Optional:
// when unset, no timeline event is written. Best-effort and never blocks the action.
func (s *Service) SetTimeline(w contracts.TimelineWriter) {
	if w != nil {
		s.timeline = w
	}
}

// SetProductCatalog wires the product lookup (snapshot) and the products toggle used
// by the deal-item endpoints. Optional: without them items cannot be managed.
func (s *Service) SetProductCatalog(lookup contracts.ProductLookup, gate contracts.ProductsGate) {
	if lookup != nil {
		s.products = lookup
	}
	if gate != nil {
		s.productsGate = gate
	}
}

// Create stores a new opportunity. Pipeline/stage default to the tenant's default
// pipeline and its first stage. The status follows the target stage (won/lost when
// terminal).
func (s *Service) Create(ctx context.Context, cmd contracts.CreateDeal) (*entity.Deal, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cmd.Title) == "" {
		return nil, apperror.Validation("title is required")
	}
	if cmd.Value < 0 {
		return nil, apperror.Validation("value must be >= 0")
	}
	pl, stage, err := s.resolvePipelineStage(ctx, cmd.PipelineID, cmd.StageID)
	if err != nil {
		return nil, err
	}
	if err := s.requireContact(ctx, cmd.ContactID); err != nil {
		return nil, err
	}
	tags, err := s.validateTags(ctx, cmd.Tags)
	if err != nil {
		return nil, err
	}

	now := s.clock.Now()
	d := &entity.Deal{
		ID:                shared.NewID(),
		TenantID:          tenantID,
		PipelineID:        pl.ID,
		Title:             strings.TrimSpace(cmd.Title),
		Value:             cmd.Value,
		Currency:          currencyOr(cmd.Currency),
		ContactID:         strings.TrimSpace(cmd.ContactID),
		AssignedTo:        strings.TrimSpace(cmd.AssignedTo),
		SectorID:          strings.TrimSpace(cmd.SectorID),
		Source:            strings.TrimSpace(cmd.Source),
		ExpectedCloseDate: cmd.ExpectedCloseDate,
		Tags:              tags,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	applyStage(d, stage, now)
	if err := s.repo.Create(ctx, d); err != nil {
		return nil, err
	}
	s.audit(ctx, "deal.created", d.ID)
	s.publishDeal(ctx, d, contracts.RealtimeDealCreated, "", "")
	s.recordTimeline(ctx, d.ID, contracts.TimelineDealCreated, dealActor(ctx),
		map[string]any{"title": d.Title, "value": d.Value, "stage_id": d.StageID})
	return d, nil
}

// CreateFromConversation creates a deal pre-linked to a conversation and its contact,
// placed in the tenant's default pipeline / first stage. Used by the "create
// opportunity" button inside a conversation.
func (s *Service) CreateFromConversation(ctx context.Context, cmd contracts.CreateFromConversation) (*entity.Deal, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	if s.conversations == nil {
		return nil, apperror.Integration("conversation lookup is not configured")
	}
	convID := strings.TrimSpace(cmd.ConversationID)
	if convID == "" {
		return nil, apperror.Validation("conversation_id is required")
	}
	conv, err := s.conversations.Conversation(ctx, convID)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(cmd.Title)
	if title == "" {
		title = "Oportunidade"
	}
	d, err := s.Create(ctx, contracts.CreateDeal{
		Title:      title,
		Value:      cmd.Value,
		Currency:   cmd.Currency,
		ContactID:  conv.ContactID,
		SectorID:   conv.SectorID,
		AssignedTo: conv.AssignedTo,
	})
	if err != nil {
		return nil, err
	}
	return s.LinkConversation(ctx, d.ID, convID)
}

// Get returns a deal by id (tenant-scoped).
func (s *Service) Get(ctx context.Context, id string) (*entity.Deal, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// List returns a page of deals matching the filter, constrained by the actor's
// visibility (assigned-to-me or my sectors when not all-scope).
func (s *Service) List(ctx context.Context, f contracts.ListFilter, page shared.PageRequest) ([]*entity.Deal, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	vis, err := s.visibility(ctx)
	if err != nil {
		return nil, err
	}
	return s.repo.List(ctx, f, vis, page.Normalize())
}

// Update edits the editable fields (title/value/currency/seller/sector/source/date).
func (s *Service) Update(ctx context.Context, id string, cmd contracts.UpdateDeal) (*entity.Deal, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	d, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	oldValue, oldAssigned := d.Value, d.AssignedTo
	if cmd.Title != nil {
		if strings.TrimSpace(*cmd.Title) == "" {
			return nil, apperror.Validation("title is required")
		}
		d.Title = strings.TrimSpace(*cmd.Title)
	}
	if cmd.Value != nil {
		if *cmd.Value < 0 {
			return nil, apperror.Validation("value must be >= 0")
		}
		// With product items present, Value is their sum (authoritative) — a manual
		// value edit is ignored. With no items it stays manually editable (compatible).
		if len(d.Items) == 0 {
			d.Value = *cmd.Value
		}
	}
	if cmd.Currency != nil {
		d.Currency = currencyOr(*cmd.Currency)
	}
	if cmd.ContactID != nil {
		contactID := strings.TrimSpace(*cmd.ContactID)
		if err := s.requireContact(ctx, contactID); err != nil {
			return nil, err
		}
		d.ContactID = contactID
	}
	if cmd.AssignedTo != nil {
		d.AssignedTo = strings.TrimSpace(*cmd.AssignedTo)
	}
	if cmd.SectorID != nil {
		d.SectorID = strings.TrimSpace(*cmd.SectorID)
	}
	if cmd.Source != nil {
		d.Source = strings.TrimSpace(*cmd.Source)
	}
	if cmd.ClearExpectedDate {
		d.ExpectedCloseDate = nil
	} else if cmd.ExpectedCloseDate != nil {
		d.ExpectedCloseDate = cmd.ExpectedCloseDate
	}
	if cmd.Tags != nil {
		tags, err := s.validateTags(ctx, *cmd.Tags)
		if err != nil {
			return nil, err
		}
		d.Tags = tags
	}
	d.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, d); err != nil {
		return nil, err
	}
	s.audit(ctx, "deal.updated", d.ID)
	s.publishDeal(ctx, d, contracts.RealtimeDealUpdated, "", "")
	actor := dealActor(ctx)
	if cmd.Value != nil && d.Value != oldValue {
		s.recordTimeline(ctx, d.ID, contracts.TimelineValueChanged, actor,
			map[string]any{"from": oldValue, "to": d.Value})
	}
	if cmd.AssignedTo != nil && d.AssignedTo != oldAssigned {
		s.recordTimeline(ctx, d.ID, contracts.TimelineAssignedChanged, actor,
			map[string]any{"from": oldAssigned, "to": d.AssignedTo})
	}
	return d, nil
}

// MoveStage moves a deal to another stage of its pipeline (the Kanban drag-and-drop).
// It bumps StageChangedAt, and sets won/lost+ClosedAt when the target is terminal or
// reopens (status=open, ClosedAt cleared) when it is not.
func (s *Service) MoveStage(ctx context.Context, dealID, stageID string) (*entity.Deal, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	d, err := s.repo.FindByID(ctx, dealID)
	if err != nil {
		return nil, err
	}
	pl, err := s.pipelines.Get(ctx, d.PipelineID)
	if err != nil {
		return nil, err
	}
	stage := findStage(pl, stageID)
	if stage == nil {
		return nil, apperror.Validation("the stage does not belong to the deal's pipeline")
	}
	fromStageID, fromStatus := d.StageID, d.Status
	applyStage(d, stage, s.clock.Now())
	d.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, d); err != nil {
		return nil, err
	}
	s.auditMove(ctx, d.ID, "user")
	s.publishDeal(ctx, d, contracts.RealtimeDealStageChanged, fromStageID, "user")
	s.recordStageMove(ctx, d, fromStageID, fromStatus, dealActor(ctx))
	return d, nil
}

// AutomationMoveDealStage moves every deal linked to the conversation into stageID of
// pipelineID, as the automation engine (the interactive_reply_received trigger). It
// is best-effort and idempotent: a deal already in the target stage — or belonging to
// a different pipeline — is left untouched, and no deal is ever created. Each moved
// deal's seller is notified in-app. The ctx is tenant-scoped (origin=automation).
//
// Possible extension (intentionally NOT implemented): when the conversation has no
// linked deal and the chosen intent implies buying, auto-create a deal in the first
// stage instead of doing nothing. Left out for now — this action only moves existing
// deals.
func (s *Service) AutomationMoveDealStage(ctx context.Context, conversationID, pipelineID, stageID string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	conversationID = strings.TrimSpace(conversationID)
	pipelineID = strings.TrimSpace(pipelineID)
	stageID = strings.TrimSpace(stageID)
	if conversationID == "" {
		return apperror.Validation("conversation_id is required")
	}
	if stageID == "" {
		return apperror.Validation("stage_id is required")
	}
	deals, err := s.repo.FindByConversation(ctx, conversationID)
	if err != nil {
		return err
	}
	for _, d := range deals {
		// The target stage belongs to pipelineID; skip deals in another pipeline.
		if pipelineID != "" && d.PipelineID != pipelineID {
			continue
		}
		if d.StageID == stageID {
			continue // idempotent: already there
		}
		pl, err := s.pipelines.Get(ctx, d.PipelineID)
		if err != nil {
			return err
		}
		stage := findStage(pl, stageID)
		if stage == nil {
			return apperror.Validation("the stage does not belong to the deal's pipeline")
		}
		fromStageID, fromStatus := d.StageID, d.Status
		now := s.clock.Now()
		applyStage(d, stage, now)
		d.UpdatedAt = now
		if err := s.repo.Update(ctx, d); err != nil {
			return err
		}
		s.auditMove(ctx, d.ID, "automation")
		s.publishDeal(ctx, d, contracts.RealtimeDealStageChanged, fromStageID, "automation")
		s.recordStageMove(ctx, d, fromStageID, fromStatus, "automation")
		s.notifyMoved(ctx, d, stage.Name)
	}
	return nil
}

// notifyMoved alerts the audience that an automated mover (automation rule or
// copilot) advanced a card: the seller when the deal has one; otherwise the deal's
// sector team; otherwise nobody (conservative — an unowned, sector-less deal does not
// spam the whole tenant). Each recipient gets an in-app notification on their own
// personal topic (TopicUser), via the existing notifier. Best-effort.
//
// The link deep-links to the card on the Kanban and carries the deal/pipeline/stage
// ids (same pattern as the channel-templates notification), so the front navigates
// and reloads the right board/column without a full refresh.
func (s *Service) notifyMoved(ctx context.Context, d *entity.Deal, stageName string) {
	recipients := s.moveAudience(ctx, d)
	if len(recipients) == 0 {
		return
	}
	link := "/crm?deal=" + d.ID + "&pipeline=" + d.PipelineID + "&stage=" + d.StageID
	body := d.Title + " → " + stageName
	for _, uid := range recipients {
		s.notifier.Notify(ctx, shared.NotifyInput{
			TenantID: d.TenantID,
			UserID:   uid,
			Type:     "deal.stage_moved_by_automation",
			Title:    "Um card do CRM foi movido automaticamente",
			Body:     body,
			Link:     link,
		})
	}
}

// moveAudience picks who hears about an automated move: the owner when assigned,
// else the deal's sector team, else nobody.
func (s *Service) moveAudience(ctx context.Context, d *entity.Deal) []string {
	if d.AssignedTo != "" {
		return []string{d.AssignedTo}
	}
	if d.SectorID != "" && s.audience != nil {
		ids, err := s.audience.SectorAgents(ctx, d.SectorID)
		if err != nil {
			return nil
		}
		return ids
	}
	return nil
}

// AssignTo sets (or clears with "") the deal's seller.
func (s *Service) AssignTo(ctx context.Context, dealID, userID string) (*entity.Deal, error) {
	return s.Update(ctx, dealID, contracts.UpdateDeal{AssignedTo: ptr(strings.TrimSpace(userID))})
}

// LinkConversation links a conversation to the deal (idempotent).
func (s *Service) LinkConversation(ctx context.Context, dealID, conversationID string) (*entity.Deal, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, apperror.Validation("conversation_id is required")
	}
	d, err := s.repo.FindByID(ctx, dealID)
	if err != nil {
		return nil, err
	}
	if d.HasConversation(conversationID) {
		return d, nil // idempotent
	}
	d.ConversationIDs = append(d.ConversationIDs, conversationID)
	d.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, d); err != nil {
		return nil, err
	}
	s.audit(ctx, "deal.conversation_linked", d.ID)
	return d, nil
}

// AddTag adds a tag (by id from the tenant's tag catalog) to a deal — idempotent. It
// validates the tag and records tag_added on the timeline.
func (s *Service) AddTag(ctx context.Context, dealID, tagID string) (*entity.Deal, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	tagID = strings.TrimSpace(tagID)
	if tagID == "" {
		return nil, apperror.Validation("tag_id is required")
	}
	d, err := s.repo.FindByID(ctx, dealID)
	if err != nil {
		return nil, err
	}
	if d.HasTag(tagID) {
		return d, nil // idempotent
	}
	if _, err := s.validateTags(ctx, []string{tagID}); err != nil {
		return nil, err
	}
	d.Tags = append(d.Tags, tagID)
	d.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, d); err != nil {
		return nil, err
	}
	s.audit(ctx, "deal.tagged", d.ID)
	s.publishDeal(ctx, d, contracts.RealtimeDealUpdated, "", "")
	s.recordTimeline(ctx, d.ID, contracts.TimelineTagAdded, dealActor(ctx), map[string]any{"tag_id": tagID})
	return d, nil
}

// RemoveTag strips a tag from a deal — idempotent and lenient (a stale id is still
// removed). It records tag_removed on the timeline.
func (s *Service) RemoveTag(ctx context.Context, dealID, tagID string) (*entity.Deal, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	d, err := s.repo.FindByID(ctx, dealID)
	if err != nil {
		return nil, err
	}
	if !d.HasTag(tagID) {
		return d, nil // idempotent: nothing to remove
	}
	out := d.Tags[:0]
	for _, id := range d.Tags {
		if id != tagID {
			out = append(out, id)
		}
	}
	d.Tags = out
	d.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, d); err != nil {
		return nil, err
	}
	s.audit(ctx, "deal.untagged", d.ID)
	s.publishDeal(ctx, d, contracts.RealtimeDealUpdated, "", "")
	s.recordTimeline(ctx, d.ID, contracts.TimelineTagRemoved, dealActor(ctx), map[string]any{"tag_id": tagID})
	return d, nil
}

// validateTags de-duplicates the tag ids and validates them against the catalog (when
// wired). Returns the cleaned id list (nil for empty).
func (s *Service) validateTags(ctx context.Context, tagIDs []string) ([]string, error) {
	cleaned := dedupeNonEmpty(tagIDs)
	if len(cleaned) == 0 {
		return nil, nil
	}
	if s.tags != nil {
		if err := s.tags.ValidateTags(ctx, cleaned); err != nil {
			return nil, err
		}
	}
	return cleaned, nil
}

func dedupeNonEmpty(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// MarkLost marks the deal lost: it moves it to the pipeline's lost stage when one
// exists, and records the reason. Always sets status=lost + ClosedAt.
func (s *Service) MarkLost(ctx context.Context, dealID, reason string) (*entity.Deal, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	d, err := s.repo.FindByID(ctx, dealID)
	if err != nil {
		return nil, err
	}
	pl, err := s.pipelines.Get(ctx, d.PipelineID)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	fromStageID, fromStatus := d.StageID, d.Status
	if lost := lostStage(pl); lost != nil {
		applyStage(d, lost, now)
	} else {
		d.Status = entity.StatusLost
		d.ClosedAt = &now
	}
	d.LostReason = strings.TrimSpace(reason)
	d.UpdatedAt = now
	if err := s.repo.Update(ctx, d); err != nil {
		return nil, err
	}
	s.audit(ctx, "deal.marked_lost", d.ID)
	s.publishDeal(ctx, d, contracts.RealtimeDealStageChanged, fromStageID, "user")
	s.recordStageMove(ctx, d, fromStageID, fromStatus, dealActor(ctx))
	return d, nil
}

// AddItem adds a product line to a deal: it snapshots the catalog product's name and
// price NOW (a later catalog change won't alter this line), recomputes the deal value
// from the items, and records product_added on the timeline.
func (s *Service) AddItem(ctx context.Context, dealID string, cmd contracts.AddItem) (*entity.Deal, error) {
	if err := s.requireProducts(ctx); err != nil {
		return nil, err
	}
	if cmd.Quantity <= 0 {
		return nil, apperror.Validation("quantity must be > 0")
	}
	if s.products == nil {
		return nil, apperror.Integration("product catalog is not configured")
	}
	d, err := s.repo.FindByID(ctx, dealID)
	if err != nil {
		return nil, err
	}
	p, err := s.products.Product(ctx, strings.TrimSpace(cmd.ProductID))
	if err != nil {
		return nil, err
	}
	if !p.Active {
		return nil, apperror.Validation("the product is not active")
	}
	item := entity.DealItem{
		ID: shared.NewID(), ProductID: strings.TrimSpace(cmd.ProductID),
		Name: p.Name, Quantity: cmd.Quantity, UnitPrice: p.Price,
	}
	d.Items = append(d.Items, item)
	if err := s.saveItems(ctx, d); err != nil {
		return nil, err
	}
	s.recordItem(ctx, d, contracts.TimelineProductAdded, item)
	return d, nil
}

// UpdateItem edits a line's quantity and/or negotiated unit price and recomputes the
// deal value.
func (s *Service) UpdateItem(ctx context.Context, dealID, itemID string, cmd contracts.UpdateItem) (*entity.Deal, error) {
	if err := s.requireProducts(ctx); err != nil {
		return nil, err
	}
	d, err := s.repo.FindByID(ctx, dealID)
	if err != nil {
		return nil, err
	}
	i := d.FindItem(itemID)
	if i < 0 {
		return nil, apperror.NotFound("item not found")
	}
	if cmd.Quantity != nil {
		if *cmd.Quantity <= 0 {
			return nil, apperror.Validation("quantity must be > 0")
		}
		d.Items[i].Quantity = *cmd.Quantity
	}
	if cmd.UnitPrice != nil {
		if *cmd.UnitPrice < 0 {
			return nil, apperror.Validation("unit_price must be >= 0")
		}
		d.Items[i].UnitPrice = *cmd.UnitPrice
	}
	if err := s.saveItems(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

// RemoveItem removes a line, recomputes the deal value, and records product_removed.
func (s *Service) RemoveItem(ctx context.Context, dealID, itemID string) (*entity.Deal, error) {
	if err := s.requireProducts(ctx); err != nil {
		return nil, err
	}
	d, err := s.repo.FindByID(ctx, dealID)
	if err != nil {
		return nil, err
	}
	i := d.FindItem(itemID)
	if i < 0 {
		return nil, apperror.NotFound("item not found")
	}
	removed := d.Items[i]
	d.Items = append(d.Items[:i], d.Items[i+1:]...)
	if err := s.saveItems(ctx, d); err != nil {
		return nil, err
	}
	s.recordItem(ctx, d, contracts.TimelineProductRemoved, removed)
	return d, nil
}

// saveItems recomputes the deal value from its items and persists, publishing the
// realtime deal.updated so the Kanban reflects the new total.
func (s *Service) saveItems(ctx context.Context, d *entity.Deal) error {
	d.RecalcValue()
	d.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, d); err != nil {
		return err
	}
	s.audit(ctx, "deal.updated", d.ID)
	s.publishDeal(ctx, d, contracts.RealtimeDealUpdated, "", "")
	return nil
}

// requireProducts enforces the tenant and that the products module is enabled (409
// when off), so deal items can only be managed while the module is on.
func (s *Service) requireProducts(ctx context.Context) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	if s.productsGate == nil {
		return nil
	}
	on, err := s.productsGate.ProductsEnabled(ctx)
	if err != nil {
		return err
	}
	if !on {
		return apperror.Conflict("the products module is disabled for this tenant")
	}
	return nil
}

// recordItem writes a product_added/product_removed event to the deal's timeline.
func (s *Service) recordItem(ctx context.Context, d *entity.Deal, kind string, item entity.DealItem) {
	s.recordTimeline(ctx, d.ID, kind, dealActor(ctx), map[string]any{
		"product_id": item.ProductID, "name": item.Name,
		"quantity": item.Quantity, "total": item.Total,
	})
}

// Delete removes a deal.
func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.audit(ctx, "deal.deleted", id)
	return nil
}

// StageHasDeals implements the pipelines domain's StageDealChecker, so deleting a
// stage that still holds deals is refused.
func (s *Service) StageHasDeals(ctx context.Context, pipelineID, stageID string) (bool, error) {
	n, err := s.repo.CountByStage(ctx, pipelineID, stageID)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

// resolvePipelineStage resolves the target pipeline (explicit or default) and stage
// (explicit or first), validating the stage belongs to the pipeline.
func (s *Service) resolvePipelineStage(ctx context.Context, pipelineID, stageID string) (*pipelineentity.Pipeline, *pipelineentity.Stage, error) {
	var (
		pl  *pipelineentity.Pipeline
		err error
	)
	if strings.TrimSpace(pipelineID) != "" {
		pl, err = s.pipelines.Get(ctx, pipelineID)
	} else {
		pl, err = s.pipelines.Default(ctx)
	}
	if err != nil {
		return nil, nil, err
	}
	if len(pl.Stages) == 0 {
		return nil, nil, apperror.Validation("the pipeline has no stages")
	}
	pl.SortStages()
	if strings.TrimSpace(stageID) == "" {
		return pl, &pl.Stages[0], nil
	}
	stage := findStage(pl, stageID)
	if stage == nil {
		return nil, nil, apperror.Validation("the stage does not belong to the pipeline")
	}
	return pl, stage, nil
}

func (s *Service) requireContact(ctx context.Context, contactID string) error {
	contactID = strings.TrimSpace(contactID)
	if contactID == "" || s.contacts == nil {
		return nil
	}
	ok, err := s.contacts.ContactExists(ctx, contactID)
	if err != nil {
		return err
	}
	if !ok {
		return apperror.Validation("contact not found")
	}
	return nil
}

func (s *Service) visibility(ctx context.Context) (contracts.Visibility, error) {
	ac, ok := authz.FromContext(ctx)
	if !ok {
		return contracts.Visibility{}, apperror.Unauthorized("authentication required")
	}
	return contracts.Visibility{All: ac.SectorScope == authz.ScopeAll, SectorIDs: ac.SectorIDs, UserID: ac.UserID}, nil
}

func (s *Service) audit(ctx context.Context, action, id string) {
	_ = s.auditor.Record(ctx, shared.AuditEntry{Action: action, ResourceType: "deal", ResourceID: id})
}

// auditMove records a stage move, tagging who moved it (user|automation) so the
// trail distinguishes a manual Kanban drag from an automation-driven advance.
func (s *Service) auditMove(ctx context.Context, id, movedBy string) {
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "deal.stage_moved", ResourceType: "deal", ResourceID: id,
		Data: map[string]any{"moved_by": movedBy},
	})
}

// publishDeal emits a realtime deal event to the rooms that mirror the deal's
// visibility (all-scope managers, its sector, its seller), so an open Kanban reacts
// live. fromStageID/movedBy are set only for a stage change. Best-effort.
func (s *Service) publishDeal(ctx context.Context, d *entity.Deal, event, fromStageID, movedBy string) {
	payload := contracts.DealEvent{
		DealID:      d.ID,
		PipelineID:  d.PipelineID,
		FromStageID: fromStageID,
		ToStageID:   d.StageID,
		Status:      string(d.Status),
		MovedBy:     movedBy,
		AssignedTo:  d.AssignedTo,
	}
	events := make([]shared.PublishEvent, 0, 3)
	for _, topic := range shared.DealTopicsFor(d.TenantID, d.SectorID, d.AssignedTo) {
		events = append(events, shared.PublishEvent{Topic: topic, Event: event, Data: payload})
	}
	shared.PublishAll(ctx, s.publisher, events...)
}

// recordTimeline appends an automatic event to the deal's timeline (best-effort).
func (s *Service) recordTimeline(ctx context.Context, dealID, kind, actor string, data map[string]any) {
	if s.timeline == nil {
		return
	}
	s.timeline.Record(ctx, contracts.TimelineEvent{DealID: dealID, Kind: kind, ActorID: actor, Data: data})
}

// recordStageMove writes the timeline events for a stage move: stage_changed when the
// stage actually changed, plus won/lost when the deal TRANSITIONED into that terminal
// status (not on a repeat).
func (s *Service) recordStageMove(ctx context.Context, d *entity.Deal, fromStageID string, fromStatus entity.Status, actor string) {
	if fromStageID != d.StageID {
		s.recordTimeline(ctx, d.ID, contracts.TimelineStageChanged, actor,
			map[string]any{"from_stage_id": fromStageID, "to_stage_id": d.StageID})
	}
	switch {
	case d.Status == entity.StatusWon && fromStatus != entity.StatusWon:
		s.recordTimeline(ctx, d.ID, contracts.TimelineWon, actor, map[string]any{"stage_id": d.StageID})
	case d.Status == entity.StatusLost && fromStatus != entity.StatusLost:
		s.recordTimeline(ctx, d.ID, contracts.TimelineLost, actor, map[string]any{"stage_id": d.StageID})
	}
}

// dealActor is the user who caused a deal action, or "system" when none is on ctx.
func dealActor(ctx context.Context) string {
	if ac, ok := authz.FromContext(ctx); ok && ac.UserID != "" {
		return ac.UserID
	}
	return "system"
}

// applyStage moves a deal into a stage and reconciles its status + closed_at:
// won/lost (with ClosedAt) when the stage is terminal, otherwise open (ClosedAt
// cleared — a reopen). StageChangedAt is always bumped.
func applyStage(d *entity.Deal, st *pipelineentity.Stage, now time.Time) {
	d.StageID = st.ID
	d.StageChangedAt = now
	switch {
	case st.IsWon:
		d.Status = entity.StatusWon
		closed := now
		d.ClosedAt = &closed
	case st.IsLost:
		d.Status = entity.StatusLost
		closed := now
		d.ClosedAt = &closed
	default:
		d.Status = entity.StatusOpen
		d.ClosedAt = nil
	}
}

func findStage(pl *pipelineentity.Pipeline, stageID string) *pipelineentity.Stage {
	i := pl.StageIndex(stageID)
	if i < 0 {
		return nil
	}
	return &pl.Stages[i]
}

func lostStage(pl *pipelineentity.Pipeline) *pipelineentity.Stage {
	for i := range pl.Stages {
		if pl.Stages[i].IsLost {
			return &pl.Stages[i]
		}
	}
	return nil
}

func currencyOr(c string) string {
	c = strings.TrimSpace(c)
	if c == "" {
		return entity.DefaultCurrency
	}
	return c
}

func ptr[T any](v T) *T { return &v }
