package shared

import "testing"

func TestCursorRoundTrip(t *testing.T) {
	in := Cursor{CreatedAt: 1718000000, ID: "abc-123"}
	token := in.Encode()
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	out, err := DecodeCursor(token)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out != in {
		t.Fatalf("round trip mismatch: got %+v want %+v", out, in)
	}
}

func TestDecodeCursorEmpty(t *testing.T) {
	c, err := DecodeCursor("")
	if err != nil {
		t.Fatalf("empty cursor should not error: %v", err)
	}
	if c != (Cursor{}) {
		t.Fatalf("expected zero cursor, got %+v", c)
	}
}

func TestDecodeCursorInvalid(t *testing.T) {
	if _, err := DecodeCursor("!!!not-base64!!!"); err == nil {
		t.Fatal("expected error for malformed cursor")
	}
}

func TestPageRequestNormalize(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{0, DefaultPageSize},
		{-5, DefaultPageSize},
		{10, 10},
		{MaxPageSize + 50, MaxPageSize},
	}
	for _, c := range cases {
		got := PageRequest{Limit: c.in}.Normalize().Limit
		if got != c.want {
			t.Errorf("Normalize(limit=%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestNewPageHasMore(t *testing.T) {
	cursorOf := func(n int) Cursor { return Cursor{CreatedAt: int64(n), ID: "x"} }

	// Over-fetched by one => has_more true, extra trimmed.
	page := NewPage([]int{1, 2, 3}, 2, cursorOf)
	if !page.Page.HasMore {
		t.Error("expected has_more=true")
	}
	if len(page.Data) != 2 {
		t.Errorf("expected 2 items, got %d", len(page.Data))
	}
	if page.Page.NextCursor == "" {
		t.Error("expected a next cursor")
	}

	// Exactly limit => no more.
	page = NewPage([]int{1, 2}, 2, cursorOf)
	if page.Page.HasMore {
		t.Error("expected has_more=false")
	}
	if page.Page.NextCursor != "" {
		t.Error("expected empty next cursor")
	}
}
