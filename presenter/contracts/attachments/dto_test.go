package attachments

import "testing"

// An avatar upload-url request needs no conversation_id; ToCommand maps the
// avatar target through to the service command.
func TestCreateUploadURLRequest_ToCommand_Avatar(t *testing.T) {
	req := CreateUploadURLRequest{
		Filename:    "face.png",
		ContentType: "image/png",
		Size:        1234,
		Avatar:      &AvatarTargetRequest{OwnerType: "contacts", OwnerID: "c1"},
	}
	cmd := req.ToCommand()
	if cmd.ConversationID != "" {
		t.Errorf("avatar request must carry no conversation_id, got %q", cmd.ConversationID)
	}
	if cmd.Avatar == nil || cmd.Avatar.OwnerType != "contacts" || cmd.Avatar.OwnerID != "c1" {
		t.Fatalf("avatar target not mapped: %+v", cmd.Avatar)
	}
}

// A conversation attachment request maps conversation_id and leaves Avatar nil.
func TestCreateUploadURLRequest_ToCommand_Conversation(t *testing.T) {
	req := CreateUploadURLRequest{ConversationID: "cv1", Filename: "a.png", ContentType: "image/png", Size: 1}
	cmd := req.ToCommand()
	if cmd.ConversationID != "cv1" || cmd.Avatar != nil {
		t.Fatalf("conversation request mapped wrong: %+v", cmd)
	}
}
