package view

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/pkg/iostreams"
)

// boolPtr is a test helper for creating *bool values.
func boolPtr(b bool) *bool { return &b }

// strPtr is a test helper for creating *string values.
func strPtr(s string) *string { return &s }

// makeDate creates an asana.Date from a time string (YYYY-MM-DD).
func makeDate(s string) *asana.Date {
	t, _ := time.Parse("2006-01-02", s)
	d := asana.Date(t)
	return &d
}

// makeTime creates a *time.Time from an RFC3339 string.
func makeTime(s string) *time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return &t
}

// fullTask returns a Task with every field populated for testing.
func fullTask() *asana.Task {
	task := &asana.Task{
		ID: "999",
		Assignee: &asana.User{
			ID:   "456",
			Name: "Tom McFarlin",
		},
		CreatedAt:  makeTime("2026-03-01T10:00:00Z"),
		ModifiedAt: makeTime("2026-04-07T15:30:00Z"),
		CompletedAt: makeTime("2026-04-08T12:00:00Z"),
		Parent: &asana.Task{
			ID: "888",
		},
		Workspace: &asana.Workspace{
			ID:   "W1",
			Name: "My Workspace",
		},
		Liked:       true,
		NumLikes:    3,
		NumSubtasks: 2,
		Followers: []*asana.User{
			{ID: "F1", Name: "Alice"},
			{ID: "F2", Name: "Bob"},
		},
		CustomFields: []*asana.CustomFieldValue{
			{
				CustomField: asana.CustomField{
					ID:              "CF1",
					CustomFieldBase: asana.CustomFieldBase{Name: "Priority"},
				},
				DisplayValue: strPtr("High"),
			},
			{
				CustomField: asana.CustomField{
					ID:              "CF2",
					CustomFieldBase: asana.CustomFieldBase{Name: "Sprint"},
				},
				DisplayValue: strPtr("Sprint 42"),
			},
		},
		Projects: []*asana.Project{
			{ID: "P1"},
		},
		Tags: []*asana.Tag{
			{ID: "T1"},
		},
		Memberships: []*asana.Membership{
			{
				Project: &asana.Project{ID: "P1"},
				Section: &asana.Section{ID: "S1"},
			},
		},
		Dependencies: []*asana.Task{{ID: "dep-1"}, {ID: "dep-2"}},
		Dependents:   []*asana.Task{{ID: "dpt-1"}},
		PermalinkURL: "https://app.asana.com/0/0/999",
	}
	task.Name = "Ship the thing"
	task.Notes = "Make it so, Number One."
	task.Completed = boolPtr(true)
	task.DueOn = makeDate("2026-04-10")
	task.DueAt = makeTime("2026-04-10T17:00:00Z")
	task.StartOn = makeDate("2026-04-01")
	task.ResourceSubtype = "default_task"

	// Set names on projects/tags/sections (embedded in bases)
	task.Projects[0].Name = "Project Alpha"
	task.Tags[0].Name = "urgent"
	task.Memberships[0].Section.Name = "In Progress"
	task.Parent.Name = "Epic parent task"

	return task
}

// --- JSON Output Tests ---

func TestDisplayDetails_JSONAllFields(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	task := fullTask()

	if err := displayDetails(task, io, true); err != nil {
		t.Fatalf("displayDetails error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}

	// Core fields
	assertJSONString(t, result, "id", "999")
	assertJSONString(t, result, "name", "Ship the thing")
	assertJSONString(t, result, "notes", "Make it so, Number One.")
	assertJSONString(t, result, "permalink_url", "https://app.asana.com/0/0/999")
	assertJSONString(t, result, "resource_subtype", "default_task")

	// Dates
	assertJSONString(t, result, "due_on", "2026-04-10")
	assertJSONString(t, result, "due_at", "2026-04-10T17:00:00Z")
	assertJSONString(t, result, "start_on", "2026-04-01")
	assertJSONString(t, result, "created_at", "2026-03-01T10:00:00Z")
	assertJSONString(t, result, "modified_at", "2026-04-07T15:30:00Z")
	assertJSONString(t, result, "completed_at", "2026-04-08T12:00:00Z")

	// Completion
	assertJSONBool(t, result, "completed", true)

	// Assignee
	assignee := assertJSONObject(t, result, "assignee")
	if assignee != nil {
		assertJSONString(t, assignee, "id", "456")
		assertJSONString(t, assignee, "name", "Tom McFarlin")
	}

	// Parent
	parent := assertJSONObject(t, result, "parent")
	if parent != nil {
		assertJSONString(t, parent, "id", "888")
		assertJSONString(t, parent, "name", "Epic parent task")
	}

	// Workspace
	workspace := assertJSONObject(t, result, "workspace")
	if workspace != nil {
		assertJSONString(t, workspace, "id", "W1")
		assertJSONString(t, workspace, "name", "My Workspace")
	}

	// Projects (array of objects with id+name)
	projects := assertJSONArray(t, result, "projects")
	if len(projects) != 1 {
		t.Errorf("projects length = %d; want 1", len(projects))
	}

	// Tags (array of objects with id+name)
	tags := assertJSONArray(t, result, "tags")
	if len(tags) != 1 {
		t.Errorf("tags length = %d; want 1", len(tags))
	}

	// Custom fields
	cfs := assertJSONArray(t, result, "custom_fields")
	if len(cfs) != 2 {
		t.Errorf("custom_fields length = %d; want 2", len(cfs))
	}

	// Dependencies & dependents
	deps := assertJSONArray(t, result, "dependencies")
	if len(deps) != 2 {
		t.Errorf("dependencies length = %d; want 2", len(deps))
	}
	dpts := assertJSONArray(t, result, "dependents")
	if len(dpts) != 1 {
		t.Errorf("dependents length = %d; want 1", len(dpts))
	}

	// Followers
	followers := assertJSONArray(t, result, "followers")
	if len(followers) != 2 {
		t.Errorf("followers length = %d; want 2", len(followers))
	}

	// Memberships
	memberships := assertJSONArray(t, result, "memberships")
	if len(memberships) != 1 {
		t.Errorf("memberships length = %d; want 1", len(memberships))
	}

	// Numeric fields
	assertJSONNumber(t, result, "num_subtasks", 2)
	assertJSONNumber(t, result, "num_likes", 3)
	assertJSONBool(t, result, "liked", true)
}

func TestDisplayDetails_JSONMinimalTask(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	task := &asana.Task{ID: "1"}
	task.Name = "Bare bones"

	if err := displayDetails(task, io, true); err != nil {
		t.Fatalf("displayDetails error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	assertJSONString(t, result, "id", "1")
	assertJSONString(t, result, "name", "Bare bones")

	// Nullable fields should be present but null
	if _, ok := result["assignee"]; !ok {
		t.Error("missing 'assignee' key")
	}
	if _, ok := result["completed"]; !ok {
		t.Error("missing 'completed' key")
	}
}

func TestDisplayDetails_JSONCustomFieldDisplayValue(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	task := &asana.Task{
		ID: "42",
		CustomFields: []*asana.CustomFieldValue{
			{
				CustomField: asana.CustomField{
					ID:              "CF1",
					CustomFieldBase: asana.CustomFieldBase{Name: "Status"},
				},
				DisplayValue: strPtr("On Track"),
			},
			{
				CustomField: asana.CustomField{
					ID:              "CF2",
					CustomFieldBase: asana.CustomFieldBase{Name: "Empty field"},
				},
				DisplayValue: nil,
			},
		},
	}
	task.Name = "Custom fields test"

	if err := displayDetails(task, io, true); err != nil {
		t.Fatalf("displayDetails error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	cfs := assertJSONArray(t, result, "custom_fields")
	if len(cfs) != 2 {
		t.Fatalf("custom_fields length = %d; want 2", len(cfs))
	}

	cf0 := cfs[0].(map[string]interface{})
	if cf0["name"] != "Status" {
		t.Errorf("custom_fields[0].name = %v; want Status", cf0["name"])
	}
	if cf0["display_value"] != "On Track" {
		t.Errorf("custom_fields[0].display_value = %v; want On Track", cf0["display_value"])
	}
}

// --- Text Output Tests ---

func TestDisplayDetails_TextAllFields(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	task := fullTask()

	if err := displayDetails(task, io, false); err != nil {
		t.Fatalf("displayDetails error: %v", err)
	}

	output := out.String()

	mustContain := []string{
		"Ship the thing",
		"Tom McFarlin",
		"Project Alpha",
		"urgent",
		"Make it so, Number One.",
		"Completed",
		"Parent:",
		"Epic parent task",
		"Custom Fields:",
		"Priority: High",
		"Sprint: Sprint 42",
		"Dependencies:",
		"Dependents:",
		"Subtasks: 2",
	}

	for _, want := range mustContain {
		if !strings.Contains(output, want) {
			t.Errorf("text output missing %q\nGot:\n%s", want, output)
		}
	}
}

func TestDisplayDetails_TextNoAssignee(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	task := &asana.Task{ID: "123"}
	task.Name = "Unassigned task"

	if err := displayDetails(task, io, false); err != nil {
		t.Fatalf("displayDetails error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Unassigned") {
		t.Errorf("text output should show 'Unassigned' when no assignee.\nGot: %s", output)
	}
}

func TestDisplayDetails_TextIncompleteTask(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	task := &asana.Task{ID: "123"}
	task.Name = "In progress task"
	task.Completed = boolPtr(false)

	if err := displayDetails(task, io, false); err != nil {
		t.Fatalf("displayDetails error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Incomplete") {
		t.Errorf("text output should show 'Incomplete' for uncompleted task.\nGot: %s", output)
	}
}

// --- Test Helpers ---

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

func assertJSONBool(t *testing.T, m map[string]interface{}, key string, want bool) {
	t.Helper()
	val, ok := m[key]
	if !ok {
		t.Errorf("JSON missing key %q", key)
		return
	}
	got, ok := val.(bool)
	if !ok {
		t.Errorf("JSON key %q is %T, not bool", key, val)
		return
	}
	if got != want {
		t.Errorf("JSON %q = %v; want %v", key, got, want)
	}
}

func assertJSONNumber(t *testing.T, m map[string]interface{}, key string, want float64) {
	t.Helper()
	val, ok := m[key]
	if !ok {
		t.Errorf("JSON missing key %q", key)
		return
	}
	got, ok := val.(float64)
	if !ok {
		t.Errorf("JSON key %q is %T, not number", key, val)
		return
	}
	if got != want {
		t.Errorf("JSON %q = %v; want %v", key, got, want)
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
