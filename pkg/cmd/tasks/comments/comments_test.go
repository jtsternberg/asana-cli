package comments

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/pkg/iostreams"
)

// makeTime creates a *time.Time from an RFC3339 string.
func makeTime(s string) *time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return &t
}

// sampleComments returns a slice of comment-type stories for testing.
func sampleComments() []*asana.Story {
	first := &asana.Story{
		ID:              "c1",
		CreatedAt:       makeTime("2026-06-19T15:17:50Z"),
		CreatedBy:       &asana.User{ID: "u1", Name: "Tom McFarlin"},
		ResourceSubtype: "comment_added",
	}
	first.Text = "First line\nSecond line"

	second := &asana.Story{
		ID:              "c2",
		CreatedAt:       makeTime("2026-06-19T16:00:00Z"),
		CreatedBy:       &asana.User{ID: "u2", Name: "Justin Sternberg"},
		ResourceSubtype: "comment_added",
	}
	second.Text = "Sounds good"
	second.IsEdited = true

	return []*asana.Story{first, second}
}

func TestFilterComments_KeepsOnlyCommentStories(t *testing.T) {
	comment := &asana.Story{ID: "c1", ResourceSubtype: "comment_added"}
	comment.Text = "a real comment"

	assigned := &asana.Story{ID: "s1", ResourceSubtype: "assigned"}
	assigned.Text = "assigned to Tom"

	completed := &asana.Story{ID: "s2", ResourceSubtype: "marked_complete"}

	got := filterComments([]*asana.Story{assigned, comment, completed})
	if len(got) != 1 {
		t.Fatalf("filterComments length = %d; want 1", len(got))
	}
	if got[0].ID != "c1" {
		t.Errorf("filterComments kept %q; want c1", got[0].ID)
	}
}

func TestDisplayComments_JSON(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	if err := displayComments(&asana.Task{ID: "999"}, sampleComments(), io, true); err != nil {
		t.Fatalf("displayComments error: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}

	if len(result) != 2 {
		t.Fatalf("comments length = %d; want 2", len(result))
	}

	if result[0]["author"] != "Tom McFarlin" {
		t.Errorf("author = %v; want Tom McFarlin", result[0]["author"])
	}
	if result[0]["text"] != "First line\nSecond line" {
		t.Errorf("text = %q; want multi-line text", result[0]["text"])
	}
	if result[0]["created_at"] != "2026-06-19T15:17:50Z" {
		t.Errorf("created_at = %v; want RFC3339 timestamp", result[0]["created_at"])
	}
}

func TestDisplayComments_Text(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	if err := displayComments(&asana.Task{ID: "999", TaskBase: asana.TaskBase{Name: "My Task"}}, sampleComments(), io, false); err != nil {
		t.Fatalf("displayComments error: %v", err)
	}

	got := out.String()
	for _, want := range []string{"Tom McFarlin", "First line", "Second line", "Justin Sternberg", "Sounds good", "(edited)"} {
		if !strings.Contains(got, want) {
			t.Errorf("text output missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestDisplayComments_TextEmpty(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	if err := displayComments(&asana.Task{ID: "999", TaskBase: asana.TaskBase{Name: "My Task"}}, nil, io, false); err != nil {
		t.Fatalf("displayComments error: %v", err)
	}

	if !strings.Contains(out.String(), "No comments") {
		t.Errorf("expected empty-state message, got: %s", out.String())
	}
}

func TestDisplayComments_JSONEmpty(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	if err := displayComments(&asana.Task{ID: "999"}, nil, io, true); err != nil {
		t.Fatalf("displayComments error: %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != "[]" {
		t.Errorf("empty JSON output = %q; want []", got)
	}
}
