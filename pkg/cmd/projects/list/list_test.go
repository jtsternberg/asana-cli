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

// fullProject returns a Project with every field populated for testing.
func fullProject() *asana.Project {
	p := &asana.Project{
		ID:         "proj-1",
		CreatedAt:  makeTime("2026-01-15T10:00:00Z"),
		ModifiedAt: makeTime("2026-04-01T14:30:00Z"),
		Owner: &asana.User{
			ID:   "user-1",
			Name: "Captain Picard",
		},
		Team: &asana.Team{
			ID:   "team-1",
			Name: "Bridge Crew",
		},
		Public: boolPtr(true),
	}
	// Fields on embedded ProjectBase
	p.Name = "Project Enterprise"
	p.Archived = boolPtr(false)
	p.Color = "dark-blue"
	p.DefaultView = asana.ViewBoard
	p.DueOn = makeDate("2026-06-01")
	p.StartOn = makeDate("2026-01-01")
	p.Notes = "Make it so."

	return p
}

// minimalProject returns a project with just ID and Name.
func minimalProject() *asana.Project {
	p := &asana.Project{ID: "proj-min"}
	p.Name = "Bare Minimum"
	return p
}

// --- JSON Output Tests ---

func TestRunList_JSONAllFields(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	projects := []*asana.Project{fullProject()}
	err := renderOutput(projects, io, true, "Test Workspace")
	if err != nil {
		t.Fatalf("renderOutput error: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 project, got %d", len(result))
	}

	p := result[0]

	// Core fields
	assertJSONString(t, p, "id", "proj-1")
	assertJSONString(t, p, "name", "Project Enterprise")
	assertJSONBool(t, p, "archived", false)
	assertJSONString(t, p, "color", "dark-blue")
	assertJSONString(t, p, "default_view", "board")
	assertJSONString(t, p, "due_on", "2026-06-01")
	assertJSONString(t, p, "start_on", "2026-01-01")
	assertJSONString(t, p, "notes", "Make it so.")
	assertJSONBool(t, p, "public", true)
	assertJSONString(t, p, "created_at", "2026-01-15T10:00:00Z")
	assertJSONString(t, p, "modified_at", "2026-04-01T14:30:00Z")

	// Owner (nested id+name)
	owner := assertJSONObject(t, p, "owner")
	if owner != nil {
		assertJSONString(t, owner, "id", "user-1")
		assertJSONString(t, owner, "name", "Captain Picard")
	}

	// Team (nested id+name)
	team := assertJSONObject(t, p, "team")
	if team != nil {
		assertJSONString(t, team, "id", "team-1")
		assertJSONString(t, team, "name", "Bridge Crew")
	}
}

func TestRunList_JSONMinimalProject(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	projects := []*asana.Project{minimalProject()}
	err := renderOutput(projects, io, true, "Test Workspace")
	if err != nil {
		t.Fatalf("renderOutput error: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 project, got %d", len(result))
	}

	p := result[0]
	assertJSONString(t, p, "id", "proj-min")
	assertJSONString(t, p, "name", "Bare Minimum")

	// Nullable fields should be null
	if val, ok := p["owner"]; !ok {
		t.Error("missing 'owner' key")
	} else if val != nil {
		t.Errorf("owner should be null for minimal project, got %v", val)
	}

	if val, ok := p["team"]; !ok {
		t.Error("missing 'team' key")
	} else if val != nil {
		t.Errorf("team should be null for minimal project, got %v", val)
	}
}

// --- Text Output Tests ---

func TestRunList_TextAllFields(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	projects := []*asana.Project{fullProject()}
	err := renderOutput(projects, io, false, "Test Workspace")
	if err != nil {
		t.Fatalf("renderOutput error: %v", err)
	}

	output := out.String()

	mustContain := []string{
		"Test Workspace",
		"Project Enterprise",
		"Captain Picard",
		"Bridge Crew",
		"2026-06-01",
		"proj-1",
	}

	for _, want := range mustContain {
		if !strings.Contains(output, want) {
			t.Errorf("text output missing %q\nGot:\n%s", want, output)
		}
	}
}

func TestRunList_TextNoProjects(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	err := renderOutput([]*asana.Project{}, io, false, "Test Workspace")
	if err != nil {
		t.Fatalf("renderOutput error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "No projects found") {
		t.Errorf("expected 'No projects found' message.\nGot:\n%s", output)
	}
}

func TestRunList_TextMinimalProject(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	projects := []*asana.Project{minimalProject()}
	err := renderOutput(projects, io, false, "Test Workspace")
	if err != nil {
		t.Fatalf("renderOutput error: %v", err)
	}

	output := out.String()

	if !strings.Contains(output, "Bare Minimum") {
		t.Errorf("text output missing project name\nGot:\n%s", output)
	}
	if !strings.Contains(output, "proj-min") {
		t.Errorf("text output missing project ID\nGot:\n%s", output)
	}
}

func TestRunList_TextMultipleProjects(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	projects := []*asana.Project{fullProject(), minimalProject()}
	err := renderOutput(projects, io, false, "Test Workspace")
	if err != nil {
		t.Fatalf("renderOutput error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Project Enterprise") {
		t.Errorf("missing first project name\nGot:\n%s", output)
	}
	if !strings.Contains(output, "Bare Minimum") {
		t.Errorf("missing second project name\nGot:\n%s", output)
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

func assertJSONObject(t *testing.T, m map[string]interface{}, key string) map[string]interface{} {
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
