package models

// ChannelIdentity is the BSON sub-document linking a contact to a channel id.
type ChannelIdentity struct {
	Channel    string `bson:"channel"`
	ExternalID string `bson:"external_id"`
}

// Contact is the BSON document for a contact. Only basic, locally-provided
// fields are stored.
type Contact struct {
	Base       `bson:",inline"`
	Name       string            `bson:"name,omitempty"`
	Phone      string            `bson:"phone,omitempty"`
	Document   string            `bson:"document,omitempty"`
	Identities []ChannelIdentity `bson:"identities,omitempty"`
	Tags       []string          `bson:"tags,omitempty"`
	Notes      string            `bson:"notes,omitempty"`
}
