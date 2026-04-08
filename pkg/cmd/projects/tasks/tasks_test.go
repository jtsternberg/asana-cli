package tasks

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

// makeDate creates an asana.Date from a time string (YYYY-MM-DD).
func makeDate(s string) *asana.Date {
	t, _ := time.Parse("2006-01-02", s)
	d := asana.Date(t)
	return &d
}

func makeTasks() []*asana.Task {
	t1 := &asana.Task{
		ID: "111",
		Assignee: &asana.User{
			ID:   "U1",
			Name: "Alice Wonderland",
		},
		Tags: []*asana.Tag{
			{ID: "T1"},
			{ID: "T2"},
		},
		PermalinkURL: "https://app.asana.com/0/0/111",
	}
	t1.Name = "Fix the flux capacitor"
	t1.Completed = boolPtr(false)
	t1.DueOn = makeDate("2026-04-15")
	t1.Tags[0].Name = "urgent"
	t1.Tags[1].Name = "backend"

	t2 := &asana.Task{
		ID:           "222",
		PermalinkURL: "https://app.asana.com/0/0/222",
	}
	t2.Name = "Write TPS reports"
	t2.Completed = boolPtr(true)

	return []*asana.Task{t1, t2}
}

// --- JSON output tests for displayTasks (flat list) ---

func TestDisplayTasks_JSONRichFields(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	project := &asana.Project{ID: "P1"}
	project.Name = "Project Alpha"

	opts := &TasksOptions{IO: io, JSON: true}
	tasks := makeTasks()

	if err := displayTasks(opts, project, tasks); err != nil {
		t.Fatalf("displayTasks error: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(result))
	}

	// First task: all fields populated
	r0 := result[0]
	assertStr(t, r0, "id", "111")
	assertStr(t, r0, "name", "Fix the flux capacitor")
	assertStr(t, r0, "due_on", "2026-04-15")
	assertStr(t, r0, "permalink_url", "https://app.asana.com/0/0/111")
	assertBool(t, r0, "completed", false)

	assignee := assertObj(t, r0, "assignee")
	if assignee != nil {
		assertStr(t, assignee, "id", "U1")
		assertStr(t, assignee, "name", "Alice Wonderland")
	}

	tags := assertArr(t, r0, "tags")
	if len(tags) != 2 {
		t.Errorf("tags length = %d; want 2", len(tags))
	}

	// Second task: minimal fields
	r1 := result[1]
	assertStr(t, r1, "id", "222")
	assertStr(t, r1, "name", "Write TPS reports")
	assertBool(t, r1, "completed", true)
	if r1["assignee"] != nil {
		t.Error("expected nil assignee for task 2")
	}
}

// --- JSON output tests for displayTasksBySection ---

func TestDisplayTasksBySection_JSONRichFields(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	project := &asana.Project{ID: "P1"}
	project.Name = "Project Alpha"

	section := &asana.Section{ID: "S1"}
	section.Name = "In Progress"

	tasks := makeTasks()
	sections := []sectionTasks{{section: section, tasks: tasks}}

	opts := &TasksOptions{IO: io, JSON: true, WithSections: true}
	if err := displayTasksBySection(opts, project, sections); err != nil {
		t.Fatalf("displayTasksBySection error: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 section, got %d", len(result))
	}

	assertStr(t, result[0], "section", "In Progress")

	tasksArr, ok := result[0]["tasks"].([]interface{})
	if !ok {
		t.Fatal("tasks is not an array")
	}
	if len(tasksArr) != 2 {
		t.Fatalf("expected 2 tasks in section, got %d", len(tasksArr))
	}

	task0 := tasksArr[0].(map[string]interface{})
	assertStr(t, task0, "id", "111")
	assertStr(t, task0, "name", "Fix the flux capacitor")
	assertStr(t, task0, "permalink_url", "https://app.asana.com/0/0/111")

	assignee := assertObj(t, task0, "assignee")
	if assignee != nil {
		assertStr(t, assignee, "id", "U1")
	}
}

// --- Text output tests for displayTasks (flat list) ---

func TestDisplayTasks_TextRichFields(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	project := &asana.Project{ID: "P1"}
	project.Name = "Project Alpha"

	opts := &TasksOptions{IO: io, JSON: false}
	tasks := makeTasks()

	if err := displayTasks(opts, project, tasks); err != nil {
		t.Fatalf("displayTasks error: %v", err)
	}

	output := out.String()

	mustContain := []string{
		"Fix the flux capacitor",
		"Alice Wonderland",
		"Apr 15, 2026",
		"Incomplete",
		"111",
		"Write TPS reports",
		"Completed",
		"222",
	}

	for _, want := range mustContain {
		if !strings.Contains(output, want) {
			t.Errorf("text output missing %q\nGot:\n%s", want, output)
		}
	}
}

func TestDisplayTasks_TextNoAssignee(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	project := &asana.Project{ID: "P1"}
	project.Name = "Test"

	task := &asana.Task{ID: "333"}
	task.Name = "Lonely task"

	opts := &TasksOptions{IO: io, JSON: false}
	if err := displayTasks(opts, project, []*asana.Task{task}); err != nil {
		t.Fatalf("displayTasks error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Lonely task") {
		t.Errorf("text output missing task name\nGot:\n%s", output)
	}
}

// --- Text output tests for displayTasksBySection ---

func TestDisplayTasksBySection_TextRichFields(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	project := &asana.Project{ID: "P1"}
	project.Name = "Project Alpha"

	section := &asana.Section{ID: "S1"}
	section.Name = "In Progress"

	tasks := makeTasks()
	sections := []sectionTasks{{section: section, tasks: tasks}}

	opts := &TasksOptions{IO: io, JSON: false, WithSections: true}
	if err := displayTasksBySection(opts, project, sections); err != nil {
		t.Fatalf("displayTasksBySection error: %v", err)
	}

	output := out.String()
	mustContain := []string{
		"In Progress",
		"Fix the flux capacitor",
		"Alice Wonderland",
		"111",
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

func assertBool(t *testing.T, m map[string]interface{}, key string, want bool) {
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

func assertObj(t *testing.T, m map[string]interface{}, key string) map[string]interface{} {
	t.Helper()
	val, ok := m[key]
	if !ok {
		t.Errorf("JSON missing key %q", key)
		return nil
	}
	if val == nil {
		return nil
	}
	obj, ok := val.(map[string]interface{})
	if !ok {
		t.Errorf("JSON key %q is %T, not object", key, val)
		return nil
	}
	return obj
}

func assertArr(t *testing.T, m map[string]interface{}, key string) []interface{} {
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
