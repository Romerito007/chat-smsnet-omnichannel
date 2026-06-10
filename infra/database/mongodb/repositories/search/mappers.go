package search

import (
	"strings"

	contactentity "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

func normalize(q string) string { return strings.TrimSpace(q) }

func toConversation(m *models.Conversation) *conventity.Conversation {
	return &conventity.Conversation{
		ID:            m.ID,
		TenantID:      m.TenantID,
		ContactID:     m.ContactID,
		Channel:       m.Channel,
		SectorID:      m.SectorID,
		QueueID:       m.QueueID,
		Status:        conventity.Status(m.Status),
		AssignedTo:    m.AssignedTo,
		Priority:      conventity.Priority(m.Priority),
		Tags:          m.Tags,
		LastMessageAt: m.LastMessageAt,
		ClosedAt:      m.ClosedAt,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}

func toContact(m *models.Contact) *contactentity.Contact {
	ids := make([]contactentity.ChannelIdentity, 0, len(m.Identities))
	for _, id := range m.Identities {
		ids = append(ids, contactentity.ChannelIdentity{Channel: id.Channel, ExternalID: id.ExternalID})
	}
	return &contactentity.Contact{
		ID:         m.ID,
		TenantID:   m.TenantID,
		Name:       m.Name,
		Phone:      m.Phone,
		Document:   m.Document,
		Identities: ids,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}
}

func toMessage(m *models.Message) *conventity.Message {
	return &conventity.Message{
		ID:             m.ID,
		TenantID:       m.TenantID,
		ConversationID: m.ConversationID,
		SenderType:     conventity.SenderType(m.SenderType),
		SenderID:       m.SenderID,
		Direction:      conventity.Direction(m.Direction),
		MessageType:    conventity.MessageType(m.MessageType),
		Text:           m.Text,
		CreatedAt:      m.CreatedAt,
		DeletedAt:      m.DeletedAt,
	}
}
