package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/csat/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/csat/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/csat/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Service drives the CSAT flow: it starts a survey on an eligible close, sends
// the question via the channel, records the public answer and expires unanswered
// surveys.
type Service struct {
	surveys     repository.SurveyRepository
	responses   repository.ResponseRepository
	sender      contracts.ChannelSender
	enqueuer    contracts.Enqueuer
	clock       shared.Clock
	expireAfter int    // seconds until an unanswered survey expires
	publicBase  string // base URL for the public answer link
}

// NewService builds the service.
func NewService(
	surveys repository.SurveyRepository,
	responses repository.ResponseRepository,
	sender contracts.ChannelSender,
	enqueuer contracts.Enqueuer,
	clock shared.Clock,
	expireAfterSeconds int,
	publicBaseURL string,
) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	if expireAfterSeconds <= 0 {
		expireAfterSeconds = 72 * 3600
	}
	return &Service{
		surveys: surveys, responses: responses, sender: sender, enqueuer: enqueuer,
		clock: clock, expireAfter: expireAfterSeconds, publicBase: strings.TrimRight(publicBaseURL, "/"),
	}
}

// OnConversationClosed (conversations CSATTrigger) starts a survey for an
// eligible closed conversation: it selects a matching enabled survey, creates a
// CSATResponse (status sent) and enqueues csat.send. It never re-sends to the
// same conversation. Best-effort: failures are swallowed.
func (s *Service) OnConversationClosed(ctx context.Context, conv *conventity.Conversation) {
	if conv == nil {
		return
	}
	// Never re-send for the same conversation.
	if existing, _ := s.responses.FindByConversation(ctx, conv.ID); existing != nil {
		return
	}
	surveys, err := s.surveys.ListEnabled(ctx)
	if err != nil {
		return
	}
	survey := selectSurvey(surveys, conv.SectorID)
	if survey == nil {
		return
	}

	now := s.clock.Now()
	resp := &entity.CSATResponse{
		ID:             shared.NewID(),
		TenantID:       conv.TenantID,
		ConversationID: conv.ID,
		ContactID:      conv.ContactID,
		SurveyID:       survey.ID,
		AgentID:        conv.AssignedTo,
		Token:          randomToken(),
		Status:         entity.StatusSent,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.responses.Create(ctx, resp); err != nil {
		return
	}
	if s.enqueuer != nil {
		_ = s.enqueuer.EnqueueSend(contracts.SendTask{TenantID: conv.TenantID, ResponseID: resp.ID}, survey.DelaySeconds)
	}
}

// Send (csat.send handler) delivers the survey question to the conversation's
// channel and schedules expiry.
func (s *Service) Send(ctx context.Context, task contracts.SendTask) error {
	resp, err := s.responses.FindByID(ctx, task.ResponseID)
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return nil // orphan task (response deleted or DB reset) — nothing to send
		}
		return err
	}
	if resp.Status != entity.StatusSent || resp.SentAt != nil {
		return nil // already delivered or no longer pending
	}
	survey, err := s.surveys.FindByID(ctx, resp.SurveyID)
	if err != nil {
		return err
	}

	text := survey.QuestionText + "\n" + s.answerLink(resp.Token)
	if err := s.sender.SendToConversation(ctx, resp.ConversationID, text); err != nil {
		return err // let Asynq retry the delivery
	}

	now := s.clock.Now()
	resp.SentAt = &now
	resp.UpdatedAt = now
	if err := s.responses.Update(ctx, resp); err != nil {
		return err
	}
	if s.enqueuer != nil {
		_ = s.enqueuer.EnqueueExpire(contracts.ExpireTask{TenantID: resp.TenantID, ResponseID: resp.ID}, s.expireAfter)
	}
	return nil
}

// Expire (csat.expire handler) marks a still-unanswered response expired.
//
// A missing response is NOT an error: the survey may have been answered and
// cleaned up, the conversation deleted, or the database reset while an orphan
// task lingered in Redis. Expiring something that no longer exists is a no-op, so
// we succeed silently instead of failing — otherwise Asynq retries to exhaustion
// and floods the logs with ERRORs for every orphan task.
func (s *Service) Expire(ctx context.Context, task contracts.ExpireTask) error {
	resp, err := s.responses.FindByID(ctx, task.ResponseID)
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return nil // orphan task — nothing to expire
		}
		return err
	}
	if resp.Status != entity.StatusSent {
		return nil // already responded or expired
	}
	resp.Status = entity.StatusExpired
	resp.UpdatedAt = s.clock.Now()
	return s.responses.Update(ctx, resp)
}

// SubmitByToken records a public answer. It validates ONLY the token (never
// exposing the conversation) and the score against the survey scale.
func (s *Service) SubmitByToken(ctx context.Context, token string, in contracts.SubmitResponse) error {
	resp, err := s.responses.FindByToken(ctx, strings.TrimSpace(token))
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return apperror.NotFound("survey not found")
		}
		return err
	}
	if !resp.Answerable() {
		return apperror.Conflict("this survey can no longer be answered")
	}

	// Load the survey within the response's tenant to validate the score.
	tctx := shared.WithTenant(ctx, resp.TenantID)
	survey, err := s.surveys.FindByID(tctx, resp.SurveyID)
	if err != nil {
		return err
	}
	if !survey.Scale.ValidScore(in.Score) {
		return apperror.Validation("invalid score for this survey").
			WithDetails(map[string]any{"score": "out of range for scale " + string(survey.Scale)})
	}

	now := s.clock.Now()
	score := in.Score
	resp.Score = &score
	resp.Comment = strings.TrimSpace(in.Comment)
	resp.RespondedAt = &now
	resp.Status = entity.StatusResponded
	resp.UpdatedAt = now
	return s.responses.Update(tctx, resp)
}

// ListResponses returns the tenant's CSAT responses (for reporting).
func (s *Service) ListResponses(ctx context.Context, page shared.PageRequest) ([]*entity.CSATResponse, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.responses.List(ctx, page.Normalize())
}

func (s *Service) answerLink(token string) string {
	return s.publicBase + "/csat/" + token
}

// selectSurvey returns the most specific matching enabled survey for a sector
// (a sector-scoped survey beats a global one), or nil.
func selectSurvey(surveys []*entity.CSATSurvey, sectorID string) *entity.CSATSurvey {
	var best *entity.CSATSurvey
	for _, sv := range surveys {
		if !sv.Enabled || sv.SendOn != entity.SendOnConversationClosed || !sv.Matches(sectorID) {
			continue
		}
		if best == nil || (len(sv.SectorIDs) > 0 && len(best.SectorIDs) == 0) {
			best = sv
		}
	}
	return best
}

func randomToken() string {
	buf := make([]byte, 24)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
