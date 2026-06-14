package models

// Holiday is the BSON document for a tenant holiday. An empty ChannelIDs with
// scope "all_channels" applies to every channel.
type Holiday struct {
	Base       `bson:",inline"`
	Date       string   `bson:"date"`
	Name       string   `bson:"name"`
	Scope      string   `bson:"scope"`
	ChannelIDs []string `bson:"channel_ids,omitempty"`
	Recurring  bool     `bson:"recurring"`
}
