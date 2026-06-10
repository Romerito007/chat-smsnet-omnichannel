package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/queues/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/queues/entity"
	sectorentity "github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/entity"
	sectorrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fakeQueueRepo struct {
	queues map[string]*entity.Queue
}

func (r *fakeQueueRepo) Create(_ context.Context, q *entity.Queue) error {
	cp := *q
	r.queues[q.ID] = &cp
	return nil
}
func (r *fakeQueueRepo) Update(_ context.Context, q *entity.Queue) error {
	if _, ok := r.queues[q.ID]; !ok {
		return apperror.NotFound("resource not found")
	}
	cp := *q
	r.queues[q.ID] = &cp
	return nil
}
func (r *fakeQueueRepo) Delete(_ context.Context, id string) error { delete(r.queues, id); return nil }
func (r *fakeQueueRepo) FindByID(ctx context.Context, id string) (*entity.Queue, error) {
	tenant, _ := shared.TenantFrom(ctx)
	if q, ok := r.queues[id]; ok && q.TenantID == tenant {
		cp := *q
		return &cp, nil
	}
	return nil, apperror.NotFound("resource not found")
}
func (r *fakeQueueRepo) List(ctx context.Context, _ shared.PageRequest) ([]*entity.Queue, error) {
	tenant, _ := shared.TenantFrom(ctx)
	var out []*entity.Queue
	for _, q := range r.queues {
		if q.TenantID == tenant {
			cp := *q
			out = append(out, &cp)
		}
	}
	return out, nil
}

// fakeSectorRepo only needs FindByID; the rest is satisfied by embedding.
type fakeSectorRepo struct {
	sectorrepo.SectorRepository
	existing map[string]string // sectorID -> tenantID
}

func (r *fakeSectorRepo) FindByID(ctx context.Context, id string) (*sectorentity.Sector, error) {
	tenant, _ := shared.TenantFrom(ctx)
	if tid, ok := r.existing[id]; ok && tid == tenant {
		return &sectorentity.Sector{ID: id, TenantID: tenant}, nil
	}
	return nil, apperror.NotFound("resource not found")
}

func tenantCtx(tenant string) context.Context {
	return shared.WithTenant(context.Background(), tenant)
}

func newQueueService(sectors map[string]string) *Service {
	return New(
		&fakeQueueRepo{queues: map[string]*entity.Queue{}},
		&fakeSectorRepo{existing: sectors},
		fixedClock{t: time.Unix(1700000000, 0).UTC()},
	)
}

func TestCreateQueue_Success(t *testing.T) {
	svc := newQueueService(map[string]string{"s1": "t1"})
	q, err := svc.Create(tenantCtx("t1"), contracts.CreateQueue{
		SectorID: "s1", Name: "Inbound", Strategy: entity.StrategyRoundRobin,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if q.Strategy != entity.StrategyRoundRobin {
		t.Errorf("strategy = %q", q.Strategy)
	}
	if !q.Enabled {
		t.Error("expected enabled default true")
	}
}

func TestCreateQueue_DefaultsStrategyManual(t *testing.T) {
	svc := newQueueService(map[string]string{"s1": "t1"})
	q, err := svc.Create(tenantCtx("t1"), contracts.CreateQueue{SectorID: "s1", Name: "Q"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if q.Strategy != entity.StrategyManual {
		t.Errorf("expected manual default, got %q", q.Strategy)
	}
}

func TestCreateQueue_InvalidStrategy(t *testing.T) {
	svc := newQueueService(map[string]string{"s1": "t1"})
	_, err := svc.Create(tenantCtx("t1"), contracts.CreateQueue{SectorID: "s1", Name: "Q", Strategy: "wat"})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation_error, got %v", err)
	}
}

func TestCreateQueue_UnknownSector(t *testing.T) {
	svc := newQueueService(map[string]string{"s1": "t1"})
	_, err := svc.Create(tenantCtx("t1"), contracts.CreateQueue{SectorID: "ghost", Name: "Q"})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation_error for missing sector, got %v", err)
	}
}

func TestCreateQueue_SectorFromAnotherTenantRejected(t *testing.T) {
	// Sector s1 belongs to t1; t2 must not be able to reference it.
	svc := newQueueService(map[string]string{"s1": "t1"})
	_, err := svc.Create(tenantCtx("t2"), contracts.CreateQueue{SectorID: "s1", Name: "Q"})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation_error cross-tenant sector, got %v", err)
	}
}
