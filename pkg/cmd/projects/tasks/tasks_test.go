package tasks

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/h2non/gock"
	"github.com/jtsternberg/asana-cli/internal/api/asana"
	"github.com/jtsternberg/asana-cli/pkg/iostreams"
)

type obj map[string]any

// TestSelectProject_ByNameUsesTypeahead verifies that resolving a project by
// name uses the workspace typeahead endpoint (constant-cost) rather than
// enumerating every project in the workspace. Only the typeahead endpoint is
// mocked; if selectProject fell back to /workspaces/{id}/projects it would hit
// an unmocked endpoint and error.
func TestSelectProject_ByNameUsesTypeahead(t *testing.T) {
	defer gock.Off()

	gock.New("https://app.asana.com").
		Get("/api/1.0/workspaces/WS1/typeahead").
		MatchParam("resource_type", "project").
		Reply(200).
		JSON(obj{"data": []obj{
			{"gid": "1204651307630741", "name": "Outgoing Tasks"},
		}})

	opts := &TasksOptions{ProjectName: "Outgoing Tasks"}
	client := asana.NewClient(http.DefaultClient)

	project, err := selectProject(opts, client, "WS1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project.ID != "1204651307630741" {
		t.Fatalf("expected project 1204651307630741, got %q", project.ID)
	}
	if !gock.IsDone() {
		t.Errorf("expected typeahead request; pending mocks: %d", len(gock.Pending()))
	}
}

// TestSelectProject_ByIDFetchesDirectly verifies that a numeric project ID is
// fetched directly by ID rather than searched by name (typeahead matches on
// name, not gid).
func TestSelectProject_ByIDFetchesDirectly(t *testing.T) {
	defer gock.Off()

	gock.New("https://app.asana.com").
		Get("/api/1.0/projects/1204651307630741").
		Reply(200).
		JSON(obj{"data": obj{"gid": "1204651307630741", "name": "Outgoing Tasks"}})

	opts := &TasksOptions{ProjectName: "1204651307630741"}
	client := asana.NewClient(http.DefaultClient)

	project, err := selectProject(opts, client, "WS1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project.ID != "1204651307630741" {
		t.Fatalf("expected project 1204651307630741, got %q", project.ID)
	}
	if !gock.IsDone() {
		t.Errorf("expected direct fetch by ID; pending mocks: %d", len(gock.Pending()))
	}
}

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

// --- opt_fields regression tests ---
//
// /projects/{gid}/tasks and /sections/{gid}/tasks return *compact* task records
// by default: gid, name, resource_type and nothing else. Both listings render
// assignee, due date and completion status, so without an explicit opt_fields
// list every task came back with a nil assignee, a nil due date and a nil
// Completed — which taskStatus renders as "Incomplete" for completed tasks too.
// A caller reading this listing to decide what to archive was being told
// something false, not merely something incomplete.

// requestedFields returns the opt_fields values sent on the last matched request.
func requestedFields(t *testing.T, captured *string) []string {
	t.Helper()
	if captured == nil || *captured == "" {
		return nil
	}
	return strings.Split(*captured, ",")
}

func hasField(fields []string, want string) bool {
	return slices.Contains(fields, want)
}

func TestListAllTasks_RequestsDisplayedFields(t *testing.T) {
	defer gock.Off()

	var optFields string
	gock.New("https://app.asana.com").
		Get("/api/1.0/projects/P1/tasks").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			optFields = req.URL.Query().Get("opt_fields")
			return true, nil
		}).
		Reply(200).
		JSON(obj{"data": []obj{
			{"gid": "111", "name": "Ship it", "completed": true,
				"assignee": obj{"gid": "U1", "name": "Ada Lovelace"}},
		}})

	io, _, out, _ := iostreams.Test()
	opts := &TasksOptions{IO: io}
	project := &asana.Project{ID: "P1"}
	project.Name = "Project Alpha"

	if err := listAllTasks(opts, asana.NewClient(http.DefaultClient), project); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fields := requestedFields(t, &optFields)
	if len(fields) == 0 {
		t.Fatalf("no opt_fields requested; compact records omit assignee, due_on and completed")
	}
	for _, want := range []string{"assignee", "assignee.name", "completed", "due_on"} {
		if !hasField(fields, want) {
			t.Errorf("opt_fields missing %q (got %v)", want, fields)
		}
	}

	// End-to-end: the values we asked for have to reach the rendered output.
	output := out.String()
	if !strings.Contains(output, "Ada Lovelace") {
		t.Errorf("output missing assignee name\nGot:\n%s", output)
	}
	if !strings.Contains(output, "Completed") {
		t.Errorf("a completed task rendered as something other than Completed\nGot:\n%s", output)
	}
}

func TestListTasksWithSections_RequestsDisplayedFields(t *testing.T) {
	defer gock.Off()

	gock.New("https://app.asana.com").
		Get("/api/1.0/projects/P1/sections").
		Reply(200).
		JSON(obj{"data": []obj{{"gid": "S1", "name": "Done"}}})

	var optFields string
	gock.New("https://app.asana.com").
		Get("/api/1.0/sections/S1/tasks").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			optFields = req.URL.Query().Get("opt_fields")
			return true, nil
		}).
		Reply(200).
		JSON(obj{"data": []obj{
			{"gid": "111", "name": "Ship it", "completed": true,
				"assignee": obj{"gid": "U1", "name": "Ada Lovelace"}},
		}})

	io, _, out, _ := iostreams.Test()
	opts := &TasksOptions{IO: io}
	project := &asana.Project{ID: "P1"}
	project.Name = "Project Alpha"

	if err := listTasksWithSections(opts, asana.NewClient(http.DefaultClient), project); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fields := requestedFields(t, &optFields)
	if len(fields) == 0 {
		t.Fatalf("no opt_fields requested on the per-section task fetch")
	}
	for _, want := range []string{"assignee", "assignee.name", "completed", "due_on"} {
		if !hasField(fields, want) {
			t.Errorf("opt_fields missing %q (got %v)", want, fields)
		}
	}

	output := out.String()
	if !strings.Contains(output, "Ada Lovelace") {
		t.Errorf("output missing assignee name\nGot:\n%s", output)
	}
	if !strings.Contains(output, "Completed") {
		t.Errorf("a completed task rendered as something other than Completed\nGot:\n%s", output)
	}
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
