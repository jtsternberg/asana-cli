package view

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/pkg/iostreams"
)

func TestDisplayDetails_JSONIncludesAssignee(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	task := &asana.Task{
		ID: "123",
		Assignee: &asana.User{
			ID:   "456",
			Name: "Tom McFarlin",
		},
	}
	task.Name = "Test task"

	if err := displayDetails(task, io, true); err != nil {
		t.Fatalf("displayDetails returned error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	assignee, ok := result["assignee"]
	if !ok {
		t.Fatal("JSON output missing 'assignee' field")
	}

	assigneeMap, ok := assignee.(map[string]interface{})
	if !ok {
		t.Fatalf("assignee is not an object: %T", assignee)
	}

	if assigneeMap["name"] != "Tom McFarlin" {
		t.Errorf("assignee.name = %q; want %q", assigneeMap["name"], "Tom McFarlin")
	}
	if assigneeMap["id"] != "456" {
		t.Errorf("assignee.id = %q; want %q", assigneeMap["id"], "456")
	}
}

func TestDisplayDetails_JSONNilAssignee(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	task := &asana.Task{
		ID: "123",
	}
	task.Name = "Unassigned task"

	if err := displayDetails(task, io, true); err != nil {
		t.Fatalf("displayDetails returned error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	// assignee should be present but null
	val, ok := result["assignee"]
	if !ok {
		t.Fatal("JSON output missing 'assignee' field")
	}
	if val != nil {
		t.Errorf("assignee = %v; want nil", val)
	}
}

func TestDisplayDetails_TextIncludesAssignee(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	task := &asana.Task{
		ID: "123",
		Assignee: &asana.User{
			Name: "Tom McFarlin",
		},
	}
	task.Name = "Test task"

	if err := displayDetails(task, io, false); err != nil {
		t.Fatalf("displayDetails returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Tom McFarlin") {
		t.Errorf("text output missing assignee name.\nGot: %s", output)
	}
}

func TestDisplayDetails_TextNoAssignee(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	task := &asana.Task{
		ID: "123",
	}
	task.Name = "Unassigned task"

	if err := displayDetails(task, io, false); err != nil {
		t.Fatalf("displayDetails returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Unassigned") {
		t.Errorf("text output should show 'Unassigned' when no assignee.\nGot: %s", output)
	}
}
