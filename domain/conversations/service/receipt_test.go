package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// seedMsg stores a pending outbound message and returns its id.
func seedMsg(mr *fakeMsgRepo) string {
	m := &entity.Message{
		ID: "m1", TenantID: "t1", ConversationID: "conv1",
		Direction: entity.DirectionOutbound, DeliveryStatus: entity.DeliveryPending,
		CreatedAt: time.Unix(1700000000, 0).UTC(),
	}
	_ = mr.Create(context.Background(), m)
	return m.ID
}

func TestApplyDeliveryReceipt_AdvancesAndIsIdempotent(t *testing.T) {
	svc, _, mr, _, _ := newService(map[string]string{})
	id := seedMsg(mr)
	ctx := shared.WithTenant(context.Background(), "t1")

	if err := svc.ApplyDeliveryReceipt(ctx, id, "delivered"); err != nil {
		t.Fatalf("delivered: %v", err)
	}
	if got, _ := mr.FindByID(ctx, id); got.DeliveryStatus != entity.DeliveryDelivered {
		t.Fatalf("status = %q, want delivered", got.DeliveryStatus)
	}
	// read advances; a repeat of delivered is a no-op (does not regress).
	if err := svc.ApplyDeliveryReceipt(ctx, id, "read"); err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := svc.ApplyDeliveryReceipt(ctx, id, "delivered"); err != nil {
		t.Fatalf("idempotent delivered: %v", err)
	}
	if got, _ := mr.FindByID(ctx, id); got.DeliveryStatus != entity.DeliveryRead {
		t.Fatalf("status = %q, want read (no regression)", got.DeliveryStatus)
	}
}

func TestApplyDeliveryReceipt_InvalidStatusRejected(t *testing.T) {
	svc, _, mr, _, _ := newService(map[string]string{})
	id := seedMsg(mr)
	ctx := shared.WithTenant(context.Background(), "t1")
	if err := svc.ApplyDeliveryReceipt(ctx, id, "bogus"); apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("expected validation_error, got %v", err)
	}
}
