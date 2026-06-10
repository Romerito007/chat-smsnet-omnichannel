package csat

import (
	"context"

	convservice "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/service"
	ccontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/csat/contracts"
)

// ChannelSender adapts the conversations service to the CSAT ChannelSender port:
// it delivers the survey question as a system outbound message, which the
// conversations service hands to the channels outbound path for real delivery
// (Prompt 9). This reuses the existing channel adapters (mock/whatsapp/webchat).
type ChannelSender struct {
	conversations *convservice.Service
}

// NewChannelSender builds the adapter.
func NewChannelSender(conversations *convservice.Service) *ChannelSender {
	return &ChannelSender{conversations: conversations}
}

// SendToConversation delivers text to the conversation's channel.
func (s *ChannelSender) SendToConversation(ctx context.Context, conversationID, text string) error {
	_, err := s.conversations.SendSystemMessage(ctx, conversationID, text)
	return err
}

var _ ccontracts.ChannelSender = (*ChannelSender)(nil)
