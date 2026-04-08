package create

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

func sampleEntry() *asana.TimeTrackingEntry {
	return &asana.TimeTrackingEntry{
		ID:              "tte-42",
		DurationMinutes: 45,
		EnteredOn:       makeDate("2026-04-08"),
		CreatedBy:       &asana.User{ID: "u1", Name: "Alice"},
		Task:            &asana.Task{ID: "t1"},
		CreatedAt:       makeTime("2026-04-08T09:00:00Z"),
		ApprovalStatus:  "pending",
		BillableStatus:  "billable",
		Description:     "Pair programming session",
	}
}

func TestRenderCreateResult_TextOutput(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	entry := sampleEntry()
	entry.Task.Name = "Build the time machine"
	taskName := "Build the time machine"

	err := renderCreateResult(io, entry, taskName, false)
	if err != nil {
		t.Fatalf("renderCreateResult error: %v", err)
	}

	output := out.String()
	mustContain := []string{
		"45 minutes",
		"Build the time machine",
	}
	for _, want := range mustContain {
		if !strings.Contains(output, want) {
			t.Errorf("text output missing %q\nGot:\n%s", want, output)
		}
	}
}

func TestRenderCreateResult_JSONOutput(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	entry := sampleEntry()
	entry.Task.Name = "Build the time machine"
	taskName := "Build the time machine"

	err := renderCreateResult(io, entry, taskName, true)
	if err != nil {
		t.Fatalf("renderCreateResult error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}

	assertJSONString(t, result, "id", "tte-42")
	assertJSONNumber(t, result, "duration_minutes", 45)
	assertJSONString(t, result, "entered_on", "2026-04-08")
	assertJSONString(t, result, "description", "Pair programming session")
	assertJSONString(t, result, "approval_status", "pending")
	assertJSONString(t, result, "billable_status", "billable")
	assertJSONString(t, result, "created_at", "2026-04-08T09:00:00Z")

	createdBy := result["created_by"].(map[string]interface{})
	assertJSONString(t, createdBy, "id", "u1")
	assertJSONString(t, createdBy, "name", "Alice")

	taskObj := result["task"].(map[string]interface{})
	assertJSONString(t, taskObj, "id", "t1")
	assertJSONString(t, taskObj, "name", "Build the time machine")
}

func TestRenderCreateResult_JSONMinimal(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	entry := &asana.TimeTrackingEntry{
		ID:              "tte-99",
		DurationMinutes: 10,
		EnteredOn:       makeDate("2026-04-08"),
	}

	err := renderCreateResult(io, entry, "Minimal task", true)
	if err != nil {
		t.Fatalf("renderCreateResult error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}

	assertJSONString(t, result, "id", "tte-99")
	assertJSONNumber(t, result, "duration_minutes", 10)

	// Nil fields should still be present
	if _, ok := result["created_by"]; !ok {
		t.Error("missing 'created_by' key")
	}
	if _, ok := result["task"]; !ok {
		t.Error("missing 'task' key")
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
