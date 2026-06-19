package conversations

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// TestListResponseAlwaysCarriesUnreadCount proves the GET /v1/conversations
// contract: the list builder maps unread_count on every row, for every status,
// independent of any status filter. The filter only narrows which rows Mongo
// returns; the per-row projection is uniform. A read row serializes "unread_count":0
// (no omitempty), an unread row keeps its real count.
func TestListResponseAlwaysCarriesUnreadCount(t *testing.T) {
	// A mixed-status page, exactly what a no-filter list returns.
	items := []*entity.Conversation{
		{ID: "c1", TenantID: "t1", Status: entity.Status("open"), UnreadCount: 7},
		{ID: "c2", TenantID: "t1", Status: entity.Status("closed"), UnreadCount: 0},
	}

	resp := NewConversationResponsesWithLastMessage(items, map[string]shared.DisplayCard{}, map[string]shared.DisplayCard{})
	if got := resp[0].UnreadCount; got != 7 {
		t.Fatalf("row c1 unread_count = %d, want 7", got)
	}
	if got := resp[1].UnreadCount; got != 0 {
		t.Fatalf("row c2 unread_count = %d, want 0", got)
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	js := string(b)
	if !strings.Contains(js, `"unread_count":7`) {
		t.Errorf("unread row must serialize its real count; got %s", js)
	}
	if strings.Count(js, `"unread_count"`) != 2 {
		t.Errorf("every row must carry unread_count (no omitempty); got %s", js)
	}
	t.Logf("list JSON: %s", js)
}
