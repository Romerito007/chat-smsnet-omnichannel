package contracts

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
)

// OutboundDispatcher hands a freshly-persisted outbound message off to the
// channels domain for delivery. It is best-effort: a channel failure must never
// fail the agent's send ("falha de canal não trava o atendimento"), so Dispatch
// returns nothing.
type OutboundDispatcher interface {
	Dispatch(ctx context.Context, conv *entity.Conversation, msg *entity.Message)
}
