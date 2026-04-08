package sections

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/pkg/iostreams"
)

func makeSections() []*asana.Section {
	createdAt := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	s1 := &asana.Section{
		ID:        "S1",
		CreatedAt: &createdAt,
	}
	s1.Name = "To Do"

	s2 := &asana.Section{
		ID: "S2",
	}
	s2.Name = "Done"

	return []*asana.Section{s1, s2}
}

// --- JSON output tests ---

func TestDisplaySections_JSON(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	project := &asana.Project{ID: "P1"}
	project.Name = "Project Alpha"

	sections := makeSections()

	opts := &SectionsOptions{IO: io, JSON: true}
	if err := displaySections(opts, project, sections); err != nil {
		t.Fatalf("displaySections error: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(result))
	}

	assertStr(t, result[0], "id", "S1")
	assertStr(t, result[0], "name", "To Do")
	assertStr(t, result[0], "created_at", "2026-03-01T10:00:00Z")

	assertStr(t, result[1], "id", "S2")
	assertStr(t, result[1], "name", "Done")
}

// --- Text output tests ---

func TestDisplaySections_TextShowsID(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	project := &asana.Project{ID: "P1"}
	project.Name = "Project Alpha"

	sections := makeSections()

	opts := &SectionsOptions{IO: io}
	if err := displaySections(opts, project, sections); err != nil {
		t.Fatalf("displaySections error: %v", err)
	}

	output := out.String()
	mustContain := []string{
		"To Do",
		"S1",
		"Done",
		"S2",
	}

	for _, want := range mustContain {
		if !strings.Contains(output, want) {
			t.Errorf("text output missing %q\nGot:\n%s", want, output)
		}
	}
}

// --- Test Helpers ---

func assertStr(t *testing.T, m map[string]interface{}, key, want string) {
	t.Helper()
	val, ok := m[key]
	if !ok {
		t.Errorf("JSON missing key %q", key)
		return
	}
	got, ok := val.(string)
	if !ok {
		t.Errorf("JSON key %q is %T, not string", key, val)
		return
	}
	if got != want {
		t.Errorf("JSON %q = %q; want %q", key, got, want)
	}
}
