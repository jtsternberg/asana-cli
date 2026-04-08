package status

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/pkg/iostreams"
)

// makeDate creates an asana.Date from a YYYY-MM-DD string.
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

// sampleEntries returns time tracking entries with all fields populated.
func sampleEntries() []*asana.TimeTrackingEntry {
	date1 := makeDate("2026-04-07")
	date2 := makeDate("2026-04-08")

	return []*asana.TimeTrackingEntry{
		{
			ID:              "tte-1",
			DurationMinutes: 60,
			EnteredOn:       date1,
			CreatedBy:       &asana.User{ID: "u1", Name: "Alice"},
			Task:            &asana.Task{ID: "t1"},
			CreatedAt:       makeTime("2026-04-07T10:00:00Z"),
			ApprovalStatus:  "pending",
			BillableStatus:  "billable",
			Description:     "Reviewed the PR",
		},
		{
			ID:              "tte-2",
			DurationMinutes: 30,
			EnteredOn:       date2,
			CreatedBy:       &asana.User{ID: "u2", Name: "Bob"},
			Task:            &asana.Task{ID: "t1"},
			CreatedAt:       makeTime("2026-04-08T14:00:00Z"),
			ApprovalStatus:  "approved",
			BillableStatus:  "non_billable",
			Description:     "",
		},
	}
}

func init() {
	// Set the task name on sample entries (Name is on the embedded NamedResource).
	// We do this inline in tests instead.
}

func TestRunStatus_TextOutput(t *testing.T) {
	entries := sampleEntries()
	entries[0].Task.Name = "Fix the flux capacitor"
	entries[1].Task.Name = "Fix the flux capacitor"

	io, _, out, _ := iostreams.Test()
	task := &asana.Task{ID: "t1"}
	task.Name = "Fix the flux capacitor"

	opts := &StatusOptions{
		JSON: false,
	}

	err := renderOutput(opts, io, task, entries)
	if err != nil {
		t.Fatalf("renderOutput error: %v", err)
	}

	output := out.String()

	// Check header
	if !strings.Contains(output, "Fix the flux capacitor") {
		t.Errorf("missing task name in output:\n%s", output)
	}

	// Check entry fields: Duration, CreatedBy, Description when present
	mustContain := []string{
		"Alice",
		"1 hour",
		"Reviewed the PR",
		"Bob",
		"30 minutes",
		"Total:",
		"1 hour 30 minutes",
	}
	for _, want := range mustContain {
		if !strings.Contains(output, want) {
			t.Errorf("text output missing %q\nGot:\n%s", want, output)
		}
	}
}

func TestRunStatus_TextOutput_NoDescription(t *testing.T) {
	// Entry with empty description should NOT show " — " separator for description
	date := makeDate("2026-04-08")
	entries := []*asana.TimeTrackingEntry{
		{
			ID:              "tte-3",
			DurationMinutes: 15,
			EnteredOn:       date,
			CreatedBy:       &asana.User{ID: "u1", Name: "Carol"},
			Description:     "",
		},
	}

	io, _, out, _ := iostreams.Test()
	task := &asana.Task{ID: "t1"}
	task.Name = "Solo entry"

	opts := &StatusOptions{JSON: false}
	err := renderOutput(opts, io, task, entries)
	if err != nil {
		t.Fatalf("renderOutput error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Carol") {
		t.Errorf("missing creator name:\n%s", output)
	}
	if !strings.Contains(output, "15 minutes") {
		t.Errorf("missing duration:\n%s", output)
	}
}

func TestRunStatus_JSONOutput(t *testing.T) {
	entries := sampleEntries()
	entries[0].Task.Name = "Fix the flux capacitor"
	entries[1].Task.Name = "Fix the flux capacitor"

	io, _, out, _ := iostreams.Test()
	task := &asana.Task{ID: "t1"}
	task.Name = "Fix the flux capacitor"

	opts := &StatusOptions{JSON: true}
	err := renderOutput(opts, io, task, entries)
	if err != nil {
		t.Fatalf("renderOutput error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}

	// Check top-level fields
	assertJSONString(t, result, "task_name", "Fix the flux capacitor")
	assertJSONNumber(t, result, "total_minutes", 90)

	// Check entries array
	entriesArr, ok := result["entries"].([]interface{})
	if !ok {
		t.Fatalf("entries is not an array")
	}
	if len(entriesArr) != 2 {
		t.Fatalf("entries length = %d; want 2", len(entriesArr))
	}

	e0 := entriesArr[0].(map[string]interface{})
	assertJSONString(t, e0, "id", "tte-1")
	assertJSONNumber(t, e0, "duration_minutes", 60)
	assertJSONString(t, e0, "entered_on", "2026-04-07")
	assertJSONString(t, e0, "description", "Reviewed the PR")
	assertJSONString(t, e0, "approval_status", "pending")
	assertJSONString(t, e0, "billable_status", "billable")
	assertJSONString(t, e0, "created_at", "2026-04-07T10:00:00Z")

	createdBy := e0["created_by"].(map[string]interface{})
	assertJSONString(t, createdBy, "id", "u1")
	assertJSONString(t, createdBy, "name", "Alice")

	taskObj := e0["task"].(map[string]interface{})
	assertJSONString(t, taskObj, "id", "t1")
	assertJSONString(t, taskObj, "name", "Fix the flux capacitor")

	e1 := entriesArr[1].(map[string]interface{})
	assertJSONString(t, e1, "id", "tte-2")
	assertJSONNumber(t, e1, "duration_minutes", 30)
	assertJSONString(t, e1, "approval_status", "approved")
	assertJSONString(t, e1, "billable_status", "non_billable")
}

func TestRunStatus_EmptyEntries(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	task := &asana.Task{ID: "t1"}
	task.Name = "Empty"

	opts := &StatusOptions{JSON: false}
	err := renderOutput(opts, io, task, nil)
	if err != nil {
		t.Fatalf("renderOutput error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "No time entries") {
		t.Errorf("expected 'No time entries' message, got:\n%s", output)
	}
}

func TestRunStatus_JSONEmptyEntries(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	task := &asana.Task{ID: "t1"}
	task.Name = "Empty"

	opts := &StatusOptions{JSON: true}
	err := renderOutput(opts, io, task, nil)
	if err != nil {
		t.Fatalf("renderOutput error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}

	assertJSONNumber(t, result, "total_minutes", 0)
	entriesArr := result["entries"].([]interface{})
	if len(entriesArr) != 0 {
		t.Errorf("expected empty entries array, got %d", len(entriesArr))
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
