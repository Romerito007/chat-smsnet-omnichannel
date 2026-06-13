package config

import (
	"testing"

	"github.com/joho/godotenv"
)

// TestGetList_StripsLeakedInlineComment guards the real upload-url bug: godotenv
// only strips an inline "# comment" when the value before it is non-empty, so a
// line like `KEY=    # note` yields the comment text AS the value. getList must
// not turn that into a (garbage) non-empty list — it should resolve to the
// default, so an empty allow-list still means "any".
func TestGetList_StripsLeakedInlineComment(t *testing.T) {
	const key = "TEST_ATTACH_ALLOWED"

	cases := []struct {
		name string
		env  string
		want []string
	}{
		{"leaked comment on empty value", "             # comma list (e.g. image/*,application/pdf); empty = any", nil},
		{"truly empty", "", nil},
		{"real values keep, trailing comment stripped", "image/*,application/pdf # note", []string{"image/*", "application/pdf"}},
		{"real values only", "image/jpeg, video/mp4", []string{"image/jpeg", "video/mp4"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(key, tc.env)
			got := getList(key, nil)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// TestGodotenvLeakedCommentBehavior documents the upstream behavior this guards
// against: an inline comment after an empty value becomes the value.
func TestGodotenvLeakedCommentBehavior(t *testing.T) {
	m, err := godotenv.Unmarshal("K=    # empty = any\n")
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["K"] == "" {
		t.Skip("godotenv now strips the leaked comment; getList guard is belt-and-suspenders")
	}
	if m["K"] != "# empty = any" {
		t.Errorf("unexpected godotenv value: %q", m["K"])
	}
}
