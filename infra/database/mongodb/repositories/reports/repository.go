// Package reports is the Mongo aggregation implementation of the reports
// repository. Each method runs a tenant-scoped aggregation pipeline honoring the
// report filter (period + sector/agent/channel).
package reports

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/reports/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/reports/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
)

var closedStatuses = bson.A{"closed", "resolved", "archived"}

// Repository implements repository.Repository.
type Repository struct {
	conversations *mongo.Collection
	messages      *mongo.Collection
	events        *mongo.Collection
	slaTrackings  *mongo.Collection
	automation    *mongo.Collection
	copilot       *mongo.Collection
	csat          *mongo.Collection
}

// New builds the repository.
func New(db *mongo.Database) *Repository {
	return &Repository{
		conversations: db.Collection("conversations"),
		messages:      db.Collection("messages"),
		events:        db.Collection("conversation_events"),
		slaTrackings:  db.Collection("sla_trackings"),
		automation:    db.Collection("automation_runs"),
		copilot:       db.Collection("copilot_logs"),
		csat:          db.Collection("csat_responses"),
	}
}

func period(f contracts.Filter) bson.M { return bson.M{"$gte": f.From, "$lte": f.To} }

// convMatch builds a conversations match from the filter. When createdInPeriod is
// true the period bounds created_at.
func convMatch(tenant string, f contracts.Filter, createdInPeriod bool) bson.M {
	m := bson.M{"tenant_id": tenant}
	if createdInPeriod {
		m["created_at"] = period(f)
	}
	if f.SectorID != "" {
		m["sector_id"] = f.SectorID
	}
	if f.AssignedTo != "" {
		m["assigned_to"] = f.AssignedTo
	}
	if f.Channel != "" {
		m["channel"] = f.Channel
	}
	return m
}

func (r *Repository) CountConversations(ctx context.Context, f contracts.Filter) (int, error) {
	tenant, err := shared.RequireTenant(ctx)
	if err != nil {
		return 0, err
	}
	n, err := r.conversations.CountDocuments(ctx, convMatch(tenant, f, true))
	return int(n), mongodb.MapError(err)
}

func (r *Repository) OpenByStatus(ctx context.Context, f contracts.Filter) ([]contracts.Bucket, error) {
	tenant, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	m := convMatch(tenant, f, false)
	m["status"] = bson.M{"$nin": closedStatuses}
	return r.groupCount(ctx, r.conversations, m, "$status")
}

func (r *Repository) ConversationsByStatus(ctx context.Context, f contracts.Filter) ([]contracts.Bucket, error) {
	tenant, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	return r.groupCount(ctx, r.conversations, convMatch(tenant, f, true), "$status")
}

func (r *Repository) ConversationsBySector(ctx context.Context, f contracts.Filter) ([]contracts.Bucket, error) {
	tenant, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	return r.groupCount(ctx, r.conversations, convMatch(tenant, f, true), "$sector_id")
}

func (r *Repository) ConversationsDaily(ctx context.Context, f contracts.Filter) ([]contracts.DateCount, error) {
	tenant, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	pipe := mongo.Pipeline{
		{{Key: "$match", Value: convMatch(tenant, f, true)}},
		{{Key: "$group", Value: bson.M{
			"_id":   bson.M{"$dateToString": bson.M{"format": "%Y-%m-%d", "date": "$created_at"}},
			"count": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.M{"_id": 1}}},
	}
	cur, err := r.conversations.Aggregate(ctx, pipe)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var out []contracts.DateCount
	for cur.Next(ctx) {
		var row struct {
			ID    string `bson:"_id"`
			Count int    `bson:"count"`
		}
		if err := cur.Decode(&row); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, contracts.DateCount{Date: row.ID, Count: row.Count})
	}
	return out, mongodb.MapError(cur.Err())
}

func (r *Repository) ClosedByReason(ctx context.Context, f contracts.Filter) ([]contracts.Bucket, error) {
	tenant, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	m := bson.M{"tenant_id": tenant, "type": "conversation.closed", "created_at": period(f)}
	return r.groupCount(ctx, r.events, m, "$data.close_reason_id")
}

func (r *Repository) MessagesByChannel(ctx context.Context, f contracts.Filter) ([]contracts.Bucket, error) {
	tenant, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	pipe := r.messageChannelPipeline(tenant, f, false)
	return r.runGroup(ctx, r.messages, pipe)
}

func (r *Repository) CountMessages(ctx context.Context, f contracts.Filter) (int, error) {
	tenant, err := shared.RequireTenant(ctx)
	if err != nil {
		return 0, err
	}
	pipe := r.messageChannelPipeline(tenant, f, true)
	pipe = append(pipe, bson.D{{Key: "$count", Value: "n"}})
	cur, err := r.messages.Aggregate(ctx, pipe)
	if err != nil {
		return 0, mongodb.MapError(err)
	}
	defer func() { _ = cur.Close(ctx) }()
	if cur.Next(ctx) {
		var row struct {
			N int `bson:"n"`
		}
		if err := cur.Decode(&row); err != nil {
			return 0, mongodb.MapError(err)
		}
		return row.N, nil
	}
	return 0, mongodb.MapError(cur.Err())
}

// messageChannelPipeline matches messages in the period and joins the
// conversation to resolve channel/sector/assignee filters. When countOnly is
// true the grouping stage is omitted (the caller adds $count).
func (r *Repository) messageChannelPipeline(tenant string, f contracts.Filter, countOnly bool) mongo.Pipeline {
	pipe := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"tenant_id": tenant, "created_at": period(f)}}},
		{{Key: "$lookup", Value: bson.M{
			"from": "conversations", "localField": "conversation_id", "foreignField": "_id", "as": "conv",
		}}},
		{{Key: "$unwind", Value: "$conv"}},
	}
	cm := bson.M{}
	if f.Channel != "" {
		cm["conv.channel"] = f.Channel
	}
	if f.SectorID != "" {
		cm["conv.sector_id"] = f.SectorID
	}
	if f.AssignedTo != "" {
		cm["conv.assigned_to"] = f.AssignedTo
	}
	if len(cm) > 0 {
		pipe = append(pipe, bson.D{{Key: "$match", Value: cm}})
	}
	if !countOnly {
		pipe = append(pipe,
			bson.D{{Key: "$group", Value: bson.M{"_id": "$conv.channel", "count": bson.M{"$sum": 1}}}},
			bson.D{{Key: "$sort", Value: bson.M{"count": -1}}},
		)
	}
	return pipe
}

func (r *Repository) FirstResponseAvgSeconds(ctx context.Context, f contracts.Filter) (float64, error) {
	tenant, err := shared.RequireTenant(ctx)
	if err != nil {
		return 0, err
	}
	m := bson.M{"tenant_id": tenant, "created_at": period(f), "first_response_at": bson.M{"$ne": nil}}
	if f.SectorID != "" {
		m["sector_id"] = f.SectorID
	}
	return r.avgSecondsBetween(ctx, r.slaTrackings, m, "$first_response_at", "$created_at")
}

func (r *Repository) ResolutionAvgSeconds(ctx context.Context, f contracts.Filter) (float64, error) {
	tenant, err := shared.RequireTenant(ctx)
	if err != nil {
		return 0, err
	}
	m := convMatch(tenant, f, false)
	m["status"] = bson.M{"$in": closedStatuses}
	m["closed_at"] = period(f)
	return r.avgSecondsBetween(ctx, r.conversations, m, "$closed_at", "$created_at")
}

func (r *Repository) AgentStats(ctx context.Context, f contracts.Filter) ([]contracts.AgentStat, error) {
	tenant, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	m := convMatch(tenant, f, false)
	m["status"] = bson.M{"$in": closedStatuses}
	m["closed_at"] = period(f)
	m["assigned_to"] = bson.M{"$nin": bson.A{"", nil}}
	pipe := mongo.Pipeline{
		{{Key: "$match", Value: m}},
		{{Key: "$group", Value: bson.M{
			"_id":     "$assigned_to",
			"count":   bson.M{"$sum": 1},
			"avgSecs": bson.M{"$avg": bson.M{"$divide": bson.A{bson.M{"$subtract": bson.A{"$closed_at", "$created_at"}}, 1000}}},
		}}},
		{{Key: "$sort", Value: bson.M{"count": -1}}},
	}
	cur, err := r.conversations.Aggregate(ctx, pipe)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var out []contracts.AgentStat
	for cur.Next(ctx) {
		var row struct {
			ID      string  `bson:"_id"`
			Count   int     `bson:"count"`
			AvgSecs float64 `bson:"avgSecs"`
		}
		if err := cur.Decode(&row); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, contracts.AgentStat{AgentID: row.ID, Conversations: row.Count, AvgResolutionSeconds: round2(row.AvgSecs)})
	}
	return out, mongodb.MapError(cur.Err())
}

func (r *Repository) SectorStats(ctx context.Context, f contracts.Filter) ([]contracts.SectorStat, error) {
	buckets, err := r.ConversationsBySector(ctx, f)
	if err != nil {
		return nil, err
	}
	out := make([]contracts.SectorStat, 0, len(buckets))
	for _, b := range buckets {
		out = append(out, contracts.SectorStat{SectorID: b.Key, Conversations: b.Count})
	}
	return out, nil
}

func (r *Repository) AutomationByStatus(ctx context.Context, f contracts.Filter) ([]contracts.Bucket, error) {
	tenant, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	return r.groupCount(ctx, r.automation, bson.M{"tenant_id": tenant, "created_at": period(f)}, "$status")
}

func (r *Repository) CopilotUsage(ctx context.Context, f contracts.Filter) (contracts.CopilotReport, error) {
	tenant, err := shared.RequireTenant(ctx)
	if err != nil {
		return contracts.CopilotReport{}, err
	}
	match := bson.M{"tenant_id": tenant, "created_at": period(f)}
	byAction, err := r.groupCount(ctx, r.copilot, match, "$action")
	if err != nil {
		return contracts.CopilotReport{}, err
	}
	pipe := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.M{
			"_id":    nil,
			"calls":  bson.M{"$sum": 1},
			"tokIn":  bson.M{"$sum": "$tokens_input"},
			"tokOut": bson.M{"$sum": "$tokens_output"},
			"cost":   bson.M{"$sum": "$estimated_cost"},
		}}},
	}
	cur, err := r.copilot.Aggregate(ctx, pipe)
	if err != nil {
		return contracts.CopilotReport{}, mongodb.MapError(err)
	}
	defer func() { _ = cur.Close(ctx) }()
	rep := contracts.CopilotReport{ByAction: byAction}
	if cur.Next(ctx) {
		var row struct {
			Calls  int     `bson:"calls"`
			TokIn  int     `bson:"tokIn"`
			TokOut int     `bson:"tokOut"`
			Cost   float64 `bson:"cost"`
		}
		if err := cur.Decode(&row); err != nil {
			return contracts.CopilotReport{}, mongodb.MapError(err)
		}
		rep.TotalCalls = row.Calls
		rep.TokensInput = row.TokIn
		rep.TokensOutput = row.TokOut
		rep.EstimatedCost = round4(row.Cost)
	}
	return rep, mongodb.MapError(cur.Err())
}

func (r *Repository) SLACounts(ctx context.Context, f contracts.Filter) (contracts.SLAReport, error) {
	tenant, err := shared.RequireTenant(ctx)
	if err != nil {
		return contracts.SLAReport{}, err
	}
	m := bson.M{"tenant_id": tenant, "created_at": period(f)}
	if f.SectorID != "" {
		m["sector_id"] = f.SectorID
	}
	pipe := mongo.Pipeline{
		{{Key: "$match", Value: m}},
		{{Key: "$group", Value: bson.M{
			"_id":     nil,
			"tracked": bson.M{"$sum": 1},
			"fr":      bson.M{"$sum": bson.M{"$cond": bson.A{"$first_response_breached", 1, 0}}},
			"res":     bson.M{"$sum": bson.M{"$cond": bson.A{"$resolution_breached", 1, 0}}},
			"met":     bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "met"}}, 1, 0}}},
		}}},
	}
	cur, err := r.slaTrackings.Aggregate(ctx, pipe)
	if err != nil {
		return contracts.SLAReport{}, mongodb.MapError(err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var rep contracts.SLAReport
	if cur.Next(ctx) {
		var row struct {
			Tracked int `bson:"tracked"`
			FR      int `bson:"fr"`
			Res     int `bson:"res"`
			Met     int `bson:"met"`
		}
		if err := cur.Decode(&row); err != nil {
			return contracts.SLAReport{}, mongodb.MapError(err)
		}
		rep.Tracked, rep.FirstResponseBreached, rep.ResolutionBreached, rep.Met = row.Tracked, row.FR, row.Res, row.Met
	}
	return rep, mongodb.MapError(cur.Err())
}

func (r *Repository) CSAT(ctx context.Context, f contracts.Filter) (repository.CSATRaw, error) {
	tenant, err := shared.RequireTenant(ctx)
	if err != nil {
		return repository.CSATRaw{}, err
	}
	match := bson.M{"tenant_id": tenant, "created_at": period(f)}
	pipe := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.M{
			"_id":        nil,
			"sent":       bson.M{"$sum": 1},
			"responded":  bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "responded"}}, 1, 0}}},
			"expired":    bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "expired"}}, 1, 0}}},
			"scoreSum":   bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$isNumber": "$score"}, "$score", 0}}},
			"scoreCount": bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$isNumber": "$score"}, 1, 0}}},
		}}},
	}
	cur, err := r.csat.Aggregate(ctx, pipe)
	if err != nil {
		return repository.CSATRaw{}, mongodb.MapError(err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var raw repository.CSATRaw
	if cur.Next(ctx) {
		var row struct {
			Sent       int `bson:"sent"`
			Responded  int `bson:"responded"`
			Expired    int `bson:"expired"`
			ScoreSum   int `bson:"scoreSum"`
			ScoreCount int `bson:"scoreCount"`
		}
		if err := cur.Decode(&row); err != nil {
			return repository.CSATRaw{}, mongodb.MapError(err)
		}
		raw.Sent, raw.Responded, raw.Expired = row.Sent, row.Responded, row.Expired
		raw.ScoreSum, raw.ScoreCount = row.ScoreSum, row.ScoreCount
	}
	// Score distribution.
	scorePipe := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"tenant_id": tenant, "created_at": period(f), "score": bson.M{"$ne": nil}}}},
		{{Key: "$group", Value: bson.M{"_id": "$score", "count": bson.M{"$sum": 1}}}},
		{{Key: "$sort", Value: bson.M{"_id": 1}}},
	}
	raw.ByScore, err = r.runGroup(ctx, r.csat, scorePipe)
	if err != nil {
		return repository.CSATRaw{}, err
	}
	return raw, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

// groupCount groups a collection by a field and counts, returning Buckets.
func (r *Repository) groupCount(ctx context.Context, coll *mongo.Collection, match bson.M, groupBy string) ([]contracts.Bucket, error) {
	pipe := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.M{"_id": groupBy, "count": bson.M{"$sum": 1}}}},
		{{Key: "$sort", Value: bson.M{"count": -1}}},
	}
	return r.runGroup(ctx, coll, pipe)
}

// runGroup runs a pipeline whose final stage yields {_id, count} and maps it to
// Buckets, stringifying the (possibly numeric/null) key.
func (r *Repository) runGroup(ctx context.Context, coll *mongo.Collection, pipe mongo.Pipeline) ([]contracts.Bucket, error) {
	cur, err := coll.Aggregate(ctx, pipe)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var out []contracts.Bucket
	for cur.Next(ctx) {
		var row struct {
			ID    any `bson:"_id"`
			Count int `bson:"count"`
		}
		if err := cur.Decode(&row); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, contracts.Bucket{Key: keyString(row.ID), Count: row.Count})
	}
	return out, mongodb.MapError(cur.Err())
}

// avgSecondsBetween computes the average of (a - b) in seconds over the matched
// documents.
func (r *Repository) avgSecondsBetween(ctx context.Context, coll *mongo.Collection, match bson.M, a, b string) (float64, error) {
	pipe := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.M{
			"_id": nil,
			"avg": bson.M{"$avg": bson.M{"$divide": bson.A{bson.M{"$subtract": bson.A{a, b}}, 1000}}},
		}}},
	}
	cur, err := coll.Aggregate(ctx, pipe)
	if err != nil {
		return 0, mongodb.MapError(err)
	}
	defer func() { _ = cur.Close(ctx) }()
	if cur.Next(ctx) {
		var row struct {
			Avg float64 `bson:"avg"`
		}
		if err := cur.Decode(&row); err != nil {
			return 0, mongodb.MapError(err)
		}
		return round2(row.Avg), nil
	}
	return 0, mongodb.MapError(cur.Err())
}

func keyString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case int32:
		return fmt.Sprintf("%d", x)
	case int64:
		return fmt.Sprintf("%d", x)
	case float64:
		return fmt.Sprintf("%g", x)
	default:
		return fmt.Sprintf("%v", x)
	}
}

func round2(v float64) float64 { return float64(int64(v*100+0.5)) / 100 }
func round4(v float64) float64 { return float64(int64(v*10000+0.5)) / 10000 }

var _ repository.Repository = (*Repository)(nil)
