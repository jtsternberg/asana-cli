package list

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/iostreams"
)

// --- Text Output Tests ---

func TestDisplayTags_TextOutputShowsColorAndID(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	tags := []*asana.Tag{
		makeTag("111", "Bug", "dark-red", "Bug-related items", "2026-01-15T10:00:00Z"),
		makeTag("222", "Feature", "dark-blue", "Feature requests", "2026-02-20T14:30:00Z"),
		makeTag("333", "Chore", "", "Maintenance stuff", "2026-03-01T09:00:00Z"),
	}

	if err := displayTags(tags, io, false, "Test Workspace"); err != nil {
		t.Fatalf("displayTags error: %v", err)
	}

	output := out.String()

	// Each tag should show Name | Color | ID
	mustContain := []string{
		"Bug",
		"dark-red",
		"111",
		"Feature",
		"dark-blue",
		"222",
		"Chore",
		"333",
		"Test Workspace",
	}

	for _, want := range mustContain {
		if !strings.Contains(output, want) {
			t.Errorf("text output missing %q\nGot:\n%s", want, output)
		}
	}
}

func TestDisplayTags_TextOutputEmptyColor(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	tags := []*asana.Tag{
		makeTag("444", "NoColor", "", "", "2026-01-01T00:00:00Z"),
	}

	if err := displayTags(tags, io, false, "Workspace"); err != nil {
		t.Fatalf("displayTags error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "NoColor") {
		t.Errorf("text output missing tag name\nGot:\n%s", output)
	}
	if !strings.Contains(output, "444") {
		t.Errorf("text output missing tag ID\nGot:\n%s", output)
	}
}

func TestDisplayTags_TextOutputNoTags(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	if err := displayTags([]*asana.Tag{}, io, false, "Empty Workspace"); err != nil {
		t.Fatalf("displayTags error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "No tags found") {
		t.Errorf("expected 'No tags found' message\nGot:\n%s", output)
	}
}

// --- JSON Output Tests ---

func TestDisplayTags_JSONAllFields(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	createdAt := parseTime("2026-01-15T10:00:00Z")
	tags := []*asana.Tag{
		{
			ID:        "111",
			TagBase:   asana.TagBase{Name: "Bug", Notes: "Bug-related items", Color: "dark-red"},
			CreatedAt: createdAt,
			Workspace: &asana.Workspace{ID: "W1", Name: "My Workspace"},
			Followers: []*asana.User{
				{ID: "U1", Name: "Alice"},
				{ID: "U2", Name: "Bob"},
			},
		},
	}

	if err := displayTags(tags, io, true, "My Workspace"); err != nil {
		t.Fatalf("displayTags error: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(result))
	}

	tag := result[0]
	assertJSONString(t, tag, "id", "111")
	assertJSONString(t, tag, "name", "Bug")
	assertJSONString(t, tag, "notes", "Bug-related items")
	assertJSONString(t, tag, "color", "dark-red")
	assertJSONString(t, tag, "created_at", "2026-01-15T10:00:00Z")

	// Workspace
	workspace := assertJSONObject(t, tag, "workspace")
	if workspace != nil {
		assertJSONString(t, workspace, "id", "W1")
		assertJSONString(t, workspace, "name", "My Workspace")
	}

	// Followers
	followers := assertJSONArray(t, tag, "followers")
	if len(followers) != 2 {
		t.Errorf("followers length = %d; want 2", len(followers))
	}
}

func TestDisplayTags_JSONMinimalTag(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	tags := []*asana.Tag{
		makeTag("555", "Minimal", "", "", ""),
	}

	if err := displayTags(tags, io, true, "Workspace"); err != nil {
		t.Fatalf("displayTags error: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(result))
	}

	tag := result[0]
	assertJSONString(t, tag, "id", "555")
	assertJSONString(t, tag, "name", "Minimal")
}

func TestDisplayTags_JSONEmpty(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	if err := displayTags([]*asana.Tag{}, io, true, "Workspace"); err != nil {
		t.Fatalf("displayTags error: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}

	if len(result) != 0 {
		t.Errorf("expected empty array, got %d items", len(result))
	}
}

func TestNewCmdList_JSONFlag(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	f := factory.Factory{
		IOStreams: io,
	}

	cmd := NewCmdList(f, nil)
	flag := cmd.Flags().Lookup("json")
	if flag == nil {
		t.Fatal("--json flag not registered on list command")
	}
}

// --- Test Helpers ---

func makeTag(id, name, color, notes, createdAtStr string) *asana.Tag {
	tag := &asana.Tag{
		ID:      id,
		TagBase: asana.TagBase{Name: name, Color: color, Notes: notes},
	}
	if createdAtStr != "" {
		tag.CreatedAt = parseTime(createdAtStr)
	}
	return tag
}

func parseTime(s string) *time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return &t
}

func assertJSONString(t *testing.T, m map[string]interface{}, key, want string) {
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

func assertJSONObject(t *testing.T, m map[string]interface{}, key string) map[string]interface{} {
	t.Helper()
	val, ok := m[key]
	if !ok {
		t.Errorf("JSON missing key %q", key)
		return nil
	}
	obj, ok := val.(map[string]interface{})
	if !ok {
		t.Errorf("JSON key %q is %T, not object", key, val)
		return nil
	}
	return obj
}

func assertJSONArray(t *testing.T, m map[string]interface{}, key string) []interface{} {
	t.Helper()
	val, ok := m[key]
	if !ok {
		t.Errorf("JSON missing key %q", key)
		return nil
	}
	arr, ok := val.([]interface{})
	if !ok {
		t.Errorf("JSON key %q is %T, not array", key, val)
		return nil
	}
	return arr
}
