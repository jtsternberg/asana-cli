package list

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

// fullTask returns a Task with every relevant field populated for list testing.
func fullTask() *asana.Task {
	task := &asana.Task{
		ID: "999",
		Assignee: &asana.User{
			ID:   "456",
			Name: "Tom McFarlin",
		},
		NumSubtasks:  2,
		PermalinkURL: "https://app.asana.com/0/0/999",
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
	}
	task.Name = "Ship the thing"
	task.Notes = "Make it so, Number One."
	task.Completed = boolPtr(false)
	task.DueOn = makeDate("2026-04-10")
	task.DueAt = nil
	task.StartOn = makeDate("2026-04-01")
	task.ResourceSubtype = "default_task"

	// Set names on nested objects
	task.Projects[0].Name = "Project Alpha"
	task.Projects[1].Name = "Project Beta"
	task.Tags[0].Name = "urgent"
	task.Parent.Name = "Epic parent task"

	return task
}

// minimalTask returns a Task with only ID and Name.
func minimalTask() *asana.Task {
	task := &asana.Task{ID: "1"}
	task.Name = "Bare bones"
	return task
}

// --- JSON Output Tests ---

func TestListRun_JSONAllFields(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	tasks := []*asana.Task{fullTask()}

	opts := &ListOptions{
		IO:   io,
		JSON: true,
	}

	if err := displayJSON(opts, tasks); err != nil {
		t.Fatalf("displayJSON error: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result))
	}

	jt := result[0]

	// Core fields
	assertJSONString(t, jt, "id", "999")
	assertJSONString(t, jt, "name", "Ship the thing")
	assertJSONString(t, jt, "notes", "Make it so, Number One.")
	assertJSONString(t, jt, "permalink_url", "https://app.asana.com/0/0/999")
	assertJSONString(t, jt, "resource_subtype", "default_task")

	// Dates
	assertJSONString(t, jt, "due_on", "2026-04-10")
	assertJSONString(t, jt, "start_on", "2026-04-01")

	// Completion
	assertJSONBool(t, jt, "completed", false)

	// Assignee
	assignee := assertJSONObject(t, jt, "assignee")
	if assignee != nil {
		assertJSONString(t, assignee, "id", "456")
		assertJSONString(t, assignee, "name", "Tom McFarlin")
	}

	// Parent
	parent := assertJSONObject(t, jt, "parent")
	if parent != nil {
		assertJSONString(t, parent, "id", "888")
		assertJSONString(t, parent, "name", "Epic parent task")
	}

	// Projects
	projects := assertJSONArray(t, jt, "projects")
	if len(projects) != 2 {
		t.Errorf("projects length = %d; want 2", len(projects))
	}

	// Tags
	tags := assertJSONArray(t, jt, "tags")
	if len(tags) != 1 {
		t.Errorf("tags length = %d; want 1", len(tags))
	}

	// Custom fields
	cfs := assertJSONArray(t, jt, "custom_fields")
	if len(cfs) != 1 {
		t.Errorf("custom_fields length = %d; want 1", len(cfs))
	}

	// Numeric fields
	assertJSONNumber(t, jt, "num_subtasks", 2)
}

func TestListRun_JSONMinimalTask(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	tasks := []*asana.Task{minimalTask()}

	opts := &ListOptions{
		IO:   io,
		JSON: true,
	}

	if err := displayJSON(opts, tasks); err != nil {
		t.Fatalf("displayJSON error: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result))
	}

	jt := result[0]
	assertJSONString(t, jt, "id", "1")
	assertJSONString(t, jt, "name", "Bare bones")

	// Nullable fields should be present but null
	if _, ok := jt["assignee"]; !ok {
		t.Error("missing 'assignee' key")
	}
	if _, ok := jt["completed"]; !ok {
		t.Error("missing 'completed' key")
	}
}

// --- Text Output Tests ---

func TestPrintTasks_TextAllFields(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	tasks := []*asana.Task{fullTask()}

	if err := printTasks(io, "testuser", tasks); err != nil {
		t.Fatalf("printTasks error: %v", err)
	}

	output := out.String()

	mustContain := []string{
		"Ship the thing",
		"Tom McFarlin",
		"Project Alpha, Project Beta",
		"Incomplete",
		"999",
	}

	for _, want := range mustContain {
		if !strings.Contains(output, want) {
			t.Errorf("text output missing %q\nGot:\n%s", want, output)
		}
	}
}

func TestPrintTasks_TextMinimalTask(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	tasks := []*asana.Task{minimalTask()}

	if err := printTasks(io, "testuser", tasks); err != nil {
		t.Fatalf("printTasks error: %v", err)
	}

	output := out.String()

	// Should show "Unassigned" for nil assignee
	if !strings.Contains(output, "Unassigned") {
		t.Errorf("text output should show 'Unassigned' when no assignee.\nGot: %s", output)
	}

	// Should show task name and ID
	if !strings.Contains(output, "Bare bones") {
		t.Errorf("text output missing task name.\nGot: %s", output)
	}
	if !strings.Contains(output, "1") {
		t.Errorf("text output missing task ID.\nGot: %s", output)
	}
}

func TestPrintTasks_TextCompletedTask(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	task := &asana.Task{ID: "42"}
	task.Name = "Done and dusted"
	task.Completed = boolPtr(true)
	tasks := []*asana.Task{task}

	if err := printTasks(io, "testuser", tasks); err != nil {
		t.Fatalf("printTasks error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Completed") {
		t.Errorf("text output should show 'Completed' for completed task.\nGot: %s", output)
	}
}

func TestPrintTasks_TextNoProjects(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	task := &asana.Task{ID: "42"}
	task.Name = "Projectless"
	tasks := []*asana.Task{task}

	if err := printTasks(io, "testuser", tasks); err != nil {
		t.Fatalf("printTasks error: %v", err)
	}

	output := out.String()
	// With no projects, the projects column should show "-"
	if !strings.Contains(output, "-") {
		t.Errorf("text output should show '-' when no projects.\nGot: %s", output)
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
		if val == nil {
			return nil // null is fine for optional objects
		}
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
