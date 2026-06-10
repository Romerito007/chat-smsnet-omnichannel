// Package copilot holds infra adapters for the copilot domain: the customer
// data source backed by the contacts repository. Financial and monitoring
// sources are intentionally left unwired in the MVP (their on-demand gateways
// live in the providerhub/monitoring domains); the copilot context builder
// already gates every section behind the tenant's allow_*_data policy.
package copilot

import (
	"context"

	contactrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/repository"
	ccontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
)

// CustomerSource adapts the contacts repository to the copilot
// CustomerDataSource port.
type CustomerSource struct {
	contacts contactrepo.ContactRepository
}

// NewCustomerSource builds the source.
func NewCustomerSource(contacts contactrepo.ContactRepository) *CustomerSource {
	return &CustomerSource{contacts: contacts}
}

// Customer returns the contact's profile subset.
func (s *CustomerSource) Customer(ctx context.Context, contactID string) (*ccontracts.CustomerInfo, error) {
	contact, err := s.contacts.FindByID(ctx, contactID)
	if err != nil {
		return nil, err
	}
	return &ccontracts.CustomerInfo{
		Name:     contact.Name,
		Document: contact.Document,
		Phone:    contact.Phone,
	}, nil
}

var _ ccontracts.CustomerDataSource = (*CustomerSource)(nil)
