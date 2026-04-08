package search

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/pkg/iostreams"
)

// --- Test Helpers ---

func boolPtr(b bool) *bool { return &b }

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

// richTask returns a Task with many fields populated for testing search output.
func richTask() *asana.Task {
	task := &asana.Task{
		ID: "12345",
		Assignee: &asana.User{
			ID:   "456",
			Name: "Tom McFarlin",
		},
		Projects: []*asana.Project{
			{ID: "P1"},
			{ID: "P2"},
		},
		Tags: []*asana.Tag{
			{ID: "T1"},
		},
		Parent: &asana.Task{
			ID: "888",
		},
		CustomFields: []*asana.CustomFieldValue{
			{
				CustomField: asana.CustomField{
					ID:              "CF1",
					CustomFieldBase: asana.CustomFieldBase{Name: "Priority"},
				},
				DisplayValue: strPtr("High"),
			},
		},
		NumSubtasks:  3,
		PermalinkURL: "https://app.asana.com/0/0/12345",
	}
	task.Name = "Deploy the flux capacitor"
	task.Completed = boolPtr(false)
	task.DueOn = makeDate("2026-04-15")
	task.DueAt = makeTime("2026-04-15T17:00:00Z")
	task.StartOn = makeDate("2026-04-01")
	task.Notes = "Great Scott!"
	task.ResourceSubtype = "default_task"

	// Set names on nested objects
	task.Projects[0].Name = "Project Alpha"
	task.Projects[1].Name = "Project Beta"
	task.Tags[0].Name = "urgent"
	task.Parent.Name = "Epic parent"

	return task
}

// minimalTask returns a Task with only ID, Name, and DueOn.
func minimalTask() *asana.Task {
	task := &asana.Task{ID: "99999"}
	task.Name = "Bare minimum task"
	task.DueOn = makeDate("2026-05-01")
	return task
}

// --- JSON Output Tests ---

func TestSearchJSON_AllFields(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	tasks := []*asana.Task{richTask()}

	opts := &SearchOptions{
		IO:   io,
		JSON: true,
	}

	if err := renderResults(opts, tasks); err != nil {
		t.Fatalf("renderResults error: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result))
	}

	task := result[0]

	// Core fields
	assertJSONString(t, task, "id", "12345")
	assertJSONString(t, task, "name", "Deploy the flux capacitor")
	assertJSONString(t, task, "resource_subtype", "default_task")
	assertJSONString(t, task, "notes", "Great Scott!")
	assertJSONString(t, task, "permalink_url", "https://app.asana.com/0/0/12345")

	// Dates
	assertJSONString(t, task, "due_on", "2026-04-15")
	assertJSONString(t, task, "due_at", "2026-04-15T17:00:00Z")
	assertJSONString(t, task, "start_on", "2026-04-01")

	// Completed
	assertJSONBool(t, task, "completed", false)

	// Assignee
	assignee := assertJSONObject(t, task, "assignee")
	if assignee != nil {
		assertJSONString(t, assignee, "id", "456")
		assertJSONString(t, assignee, "name", "Tom McFarlin")
	}

	// Parent
	parent := assertJSONObject(t, task, "parent")
	if parent != nil {
		assertJSONString(t, parent, "id", "888")
		assertJSONString(t, parent, "name", "Epic parent")
	}

	// Projects
	projects := assertJSONArray(t, task, "projects")
	if len(projects) != 2 {
		t.Errorf("projects length = %d; want 2", len(projects))
	}

	// Tags
	tags := assertJSONArray(t, task, "tags")
	if len(tags) != 1 {
		t.Errorf("tags length = %d; want 1", len(tags))
	}

	// Custom fields
	cfs := assertJSONArray(t, task, "custom_fields")
	if len(cfs) != 1 {
		t.Errorf("custom_fields length = %d; want 1", len(cfs))
	}

	// Num subtasks
	assertJSONNumber(t, task, "num_subtasks", 3)
}

func TestSearchJSON_MinimalTask(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	tasks := []*asana.Task{minimalTask()}

	opts := &SearchOptions{
		IO:   io,
		JSON: true,
	}

	if err := renderResults(opts, tasks); err != nil {
		t.Fatalf("renderResults error: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}

	task := result[0]
	assertJSONString(t, task, "id", "99999")
	assertJSONString(t, task, "name", "Bare minimum task")
	assertJSONString(t, task, "due_on", "2026-05-01")

	// Nullable fields should be absent or null
	if val, ok := task["assignee"]; ok && val != nil {
		t.Errorf("expected assignee to be null or absent, got %v", val)
	}
}

// --- Text Output Tests ---

func TestSearchText_RichTask(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	tasks := []*asana.Task{richTask()}

	opts := &SearchOptions{
		IO:   io,
		JSON: false,
	}

	if err := renderResults(opts, tasks); err != nil {
		t.Fatalf("renderResults error: %v", err)
	}

	output := out.String()

	mustContain := []string{
		"Deploy the flux capacitor",
		"Tom McFarlin",
		"Project Alpha, Project Beta",
		"12345",
		"Incomplete",
	}

	for _, want := range mustContain {
		if !strings.Contains(output, want) {
			t.Errorf("text output missing %q\nGot:\n%s", want, output)
		}
	}
}

func TestSearchText_UnassignedTask(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	tasks := []*asana.Task{minimalTask()}

	opts := &SearchOptions{
		IO:   io,
		JSON: false,
	}

	if err := renderResults(opts, tasks); err != nil {
		t.Fatalf("renderResults error: %v", err)
	}

	output := out.String()

	// Unassigned tasks should show a dash for the assignee
	if !strings.Contains(output, "\u2014") && !strings.Contains(output, "Unassigned") {
		t.Errorf("text output should show unassigned indicator\nGot:\n%s", output)
	}
}

func TestSearchText_CompletedTask(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	task := minimalTask()
	task.Completed = boolPtr(true)
	tasks := []*asana.Task{task}

	opts := &SearchOptions{
		IO:   io,
		JSON: false,
	}

	if err := renderResults(opts, tasks); err != nil {
		t.Fatalf("renderResults error: %v", err)
	}

	output := out.String()

	if !strings.Contains(output, "Done") && !strings.Contains(output, "Complete") {
		t.Errorf("text output should indicate completed status\nGot:\n%s", output)
	}
}

func TestSearchText_WithAssigneeHeader(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	tasks := []*asana.Task{richTask()}

	opts := &SearchOptions{
		IO:       io,
		JSON:     false,
		Assignee: []string{"456"},
	}

	if err := renderResults(opts, tasks); err != nil {
		t.Fatalf("renderResults error: %v", err)
	}

	output := out.String()

	if !strings.Contains(output, "456") {
		t.Errorf("text output should mention assignee in header\nGot:\n%s", output)
	}
}

// --- Test Assertion Helpers ---

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
