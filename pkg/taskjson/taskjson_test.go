package taskjson

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/timwehrle/asana/internal/api/asana"
)

func boolPtr(b bool) *bool    { return &b }
func strPtr(s string) *string { return &s }
func makeDate(s string) *asana.Date {
	t, _ := time.Parse("2006-01-02", s)
	d := asana.Date(t)
	return &d
}
func makeTime(s string) *time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return &t
}

// fullTask returns a task with every field the canonical shape can carry.
func fullTask() *asana.Task {
	task := &asana.Task{
		ID:          "999",
		Assignee:    &asana.User{ID: "456", Name: "Tom McFarlin"},
		CreatedAt:   makeTime("2026-03-01T10:00:00Z"),
		ModifiedAt:  makeTime("2026-04-07T15:30:00Z"),
		CompletedAt: makeTime("2026-04-08T12:00:00Z"),
		Parent:      &asana.Task{ID: "888"},
		Workspace:   &asana.Workspace{ID: "W1", Name: "My Workspace"},
		Liked:       true,
		NumLikes:    3,
		NumSubtasks: 2,
		Followers: []*asana.User{
			{ID: "F1", Name: "Alice"},
			{ID: "F2", Name: "Bob"},
		},
		CustomFields: []*asana.CustomFieldValue{
			{
				CustomField:  asana.CustomField{ID: "CF1", CustomFieldBase: asana.CustomFieldBase{Name: "Priority"}},
				DisplayValue: strPtr("High"),
			},
			{
				CustomField:  asana.CustomField{ID: "CF2", CustomFieldBase: asana.CustomFieldBase{Name: "Sprint"}},
				DisplayValue: nil,
			},
		},
		Projects: []*asana.Project{{ID: "P1"}},
		Tags:     []*asana.Tag{{ID: "T1"}},
		Memberships: []*asana.Membership{
			{Project: &asana.Project{ID: "P1"}, Section: &asana.Section{ID: "S1"}},
		},
		Dependencies: []*asana.Task{{ID: "dep-1"}, {ID: "dep-2"}},
		Dependents:   []*asana.Task{{ID: "dpt-1"}},
		PermalinkURL: "https://app.asana.com/0/0/999",
	}
	task.Name = "Ship the thing"
	task.Notes = "Make it so, Number One."
	task.HTMLNotes = `<body>See <a href="https://example.com">the docs</a></body>`
	task.Completed = boolPtr(true)
	task.DueOn = makeDate("2026-04-10")
	task.DueAt = makeTime("2026-04-10T17:00:00Z")
	task.StartOn = makeDate("2026-04-01")
	task.ResourceSubtype = "default_task"

	task.Projects[0].Name = "Project Alpha"
	task.Tags[0].Name = "urgent"
	task.Memberships[0].Project.Name = "Project Alpha"
	task.Memberships[0].Section.Name = "In Progress"
	task.Parent.Name = "Epic parent task"

	return task
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, b)
	}
	return m
}

func TestNew_MapsEveryField(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, fullTask()); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got := decode(t, buf.Bytes())

	// The key is "id", not "gid": the wire name is renamed on the way out.
	for key, want := range map[string]any{
		"id":               "999",
		"name":             "Ship the thing",
		"resource_subtype": "default_task",
		"completed":        true,
		"completed_at":     "2026-04-08T12:00:00Z",
		"created_at":       "2026-03-01T10:00:00Z",
		"modified_at":      "2026-04-07T15:30:00Z",
		"due_on":           "2026-04-10",
		"due_at":           "2026-04-10T17:00:00Z",
		"start_on":         "2026-04-01",
		"notes":            "Make it so, Number One.",
		"html_notes":       `<body>See <a href="https://example.com">the docs</a></body>`,
		"num_subtasks":     float64(2),
		"liked":            true,
		"num_likes":        float64(3),
		"permalink_url":    "https://app.asana.com/0/0/999",
	} {
		if got[key] != want {
			t.Errorf("%s = %#v, want %#v", key, got[key], want)
		}
	}

	assignee, ok := got["assignee"].(map[string]any)
	if !ok {
		t.Fatalf("assignee = %#v, want an object", got["assignee"])
	}
	if assignee["id"] != "456" || assignee["name"] != "Tom McFarlin" {
		t.Errorf("assignee = %#v", assignee)
	}

	if ws := got["workspace"].(map[string]any); ws["id"] != "W1" || ws["name"] != "My Workspace" {
		t.Errorf("workspace = %#v", ws)
	}
	if parent := got["parent"].(map[string]any); parent["id"] != "888" || parent["name"] != "Epic parent task" {
		t.Errorf("parent = %#v", parent)
	}

	if n := len(got["projects"].([]any)); n != 1 {
		t.Errorf("projects length = %d, want 1", n)
	}
	if n := len(got["tags"].([]any)); n != 1 {
		t.Errorf("tags length = %d, want 1", n)
	}
	if n := len(got["followers"].([]any)); n != 2 {
		t.Errorf("followers length = %d, want 2", n)
	}
	if n := len(got["dependencies"].([]any)); n != 2 {
		t.Errorf("dependencies length = %d, want 2", n)
	}
	if n := len(got["dependents"].([]any)); n != 1 {
		t.Errorf("dependents length = %d, want 1", n)
	}

	membership := got["memberships"].([]any)[0].(map[string]any)
	if p := membership["project"].(map[string]any); p["id"] != "P1" {
		t.Errorf("membership project = %#v", p)
	}
	if s := membership["section"].(map[string]any); s["id"] != "S1" || s["name"] != "In Progress" {
		t.Errorf("membership section = %#v", s)
	}

	fields := got["custom_fields"].([]any)
	if len(fields) != 2 {
		t.Fatalf("custom_fields length = %d, want 2", len(fields))
	}
	if cf := fields[0].(map[string]any); cf["id"] != "CF1" || cf["name"] != "Priority" || cf["display_value"] != "High" {
		t.Errorf("custom_fields[0] = %#v", cf)
	}
	// display_value is explicitly null rather than absent when unset, so a
	// consumer can tell "no value" from "field not returned".
	cf := fields[1].(map[string]any)
	if v, present := cf["display_value"]; !present || v != nil {
		t.Errorf("custom_fields[1].display_value = %#v (present=%v), want explicit null", v, present)
	}
}

// A ledger has to be able to distinguish "unassigned" and "not complete" from
// "field missing", so these two keys are always emitted.
func TestNew_EmptyTaskStillEmitsAssigneeAndCompleted(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, &asana.Task{ID: "1"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got := decode(t, buf.Bytes())

	for _, key := range []string{"id", "name", "assignee", "completed", "num_subtasks", "liked", "num_likes"} {
		if _, present := got[key]; !present {
			t.Errorf("key %q should always be present", key)
		}
	}
	if got["assignee"] != nil {
		t.Errorf("assignee = %#v, want null", got["assignee"])
	}
	if got["completed"] != nil {
		t.Errorf("completed = %#v, want null", got["completed"])
	}
	// Optional keys stay out of the way when unset.
	for _, key := range []string{"html_notes", "notes", "due_on", "parent", "projects"} {
		if _, present := got[key]; present {
			t.Errorf("key %q should be omitted when empty", key)
		}
	}
}

// Escaping < and > would turn every link in html_notes into <a href=...,
// which is semantically identical and unreadable.
func TestWrite_DoesNotEscapeHTML(t *testing.T) {
	task := &asana.Task{ID: "1"}
	task.HTMLNotes = `<body>a <a href="https://example.com?a=1&b=2">link</a></body>`

	var buf bytes.Buffer
	if err := Write(&buf, task); err != nil {
		t.Fatalf("Write: %v", err)
	}

	raw := buf.String()
	for _, escaped := range []string{`\u003c`, `\u003e`, `\u0026`} {
		if strings.Contains(raw, escaped) {
			t.Errorf("output should not contain the escape %s:\n%s", escaped, raw)
		}
	}
	if !strings.Contains(raw, `<a href=\"https://example.com?a=1&b=2\">`) {
		t.Errorf("expected readable markup, got:\n%s", raw)
	}
}

func TestWriteAll_EmitsAnArray(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteAll(&buf, []*asana.Task{{ID: "1"}, {ID: "2"}}); err != nil {
		t.Fatalf("WriteAll: %v", err)
	}

	var got []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, buf.String())
	}
	if len(got) != 2 || got[0]["id"] != "1" || got[1]["id"] != "2" {
		t.Errorf("got %#v", got)
	}
}

func TestWriteAll_EmptySliceIsAnEmptyArray(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteAll(&buf, nil); err != nil {
		t.Fatalf("WriteAll: %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got != "[]" {
		t.Errorf("got %q, want []", got)
	}
}

// The reason this package exists is that four commands had drifted apart. Tie
// the opt_fields list to the shape so a new key cannot be added without also
// being requested from the API — otherwise it silently serializes as empty.
func TestFields_CoversEveryKeyInTheShape(t *testing.T) {
	requested := map[string]bool{}
	for _, f := range Fields() {
		requested[f] = true
		// "assignee.name" also implies "assignee".
		if root, _, found := strings.Cut(f, "."); found {
			requested[root] = true
		}
	}

	typ := reflect.TypeFor[Task]()
	for i := 0; i < typ.NumField(); i++ {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		if name == "" || name == "id" {
			continue // gid is always returned, regardless of opt_fields
		}
		if !requested[name] {
			t.Errorf("shape has %q but Fields() does not request it", name)
		}
	}
}

func TestFields_HasNoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range Fields() {
		if seen[f] {
			t.Errorf("duplicate field %q", f)
		}
		seen[f] = true
	}
}

func TestOptions_CarriesFields(t *testing.T) {
	opts := Options()
	if opts == nil {
		t.Fatal("Options() = nil")
	}
	if !reflect.DeepEqual(opts.Fields, Fields()) {
		t.Errorf("Options().Fields = %v, want Fields()", opts.Fields)
	}
	// Callers must not be able to mutate the package-level list through it.
	opts.Fields[0] = "mutated"
	if Fields()[0] == "mutated" {
		t.Error("Options() handed out a reference to the canonical slice")
	}
}
