package create

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/pkg/iostreams"
)

func TestDisplaySection_JSON(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	project := &asana.Project{ID: "P1"}
	project.Name = "Outgoing Tasks"

	createdAt := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	section := &asana.Section{ID: "S99", CreatedAt: &createdAt}
	section.Name = "Ben"

	opts := &CreateOptions{IO: io, JSON: true}
	if err := displaySection(opts, project, section); err != nil {
		t.Fatalf("displaySection error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}

	if result["id"] != "S99" {
		t.Errorf("id = %v; want S99", result["id"])
	}
	if result["name"] != "Ben" {
		t.Errorf("name = %v; want Ben", result["name"])
	}
	if result["project_id"] != "P1" {
		t.Errorf("project_id = %v; want P1", result["project_id"])
	}
	if result["created_at"] != "2026-05-01T10:00:00Z" {
		t.Errorf("created_at = %v; want 2026-05-01T10:00:00Z", result["created_at"])
	}
}

func TestDisplaySection_Text(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	project := &asana.Project{ID: "P1"}
	project.Name = "Outgoing Tasks"

	section := &asana.Section{ID: "S99"}
	section.Name = "Ben"

	opts := &CreateOptions{IO: io}
	if err := displaySection(opts, project, section); err != nil {
		t.Fatalf("displaySection error: %v", err)
	}

	output := out.String()
	for _, want := range []string{"Ben", "Outgoing Tasks", "S99"} {
		if !strings.Contains(output, want) {
			t.Errorf("text output missing %q\nGot:\n%s", want, output)
		}
	}
}

func TestRunCreate_EmptyName(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	opts := &CreateOptions{
		IO:          io,
		ProjectName: "Outgoing Tasks",
		SectionName: "   ",
	}
	err := runCreate(opts)
	if err == nil || !strings.Contains(err.Error(), "section name cannot be empty") {
		t.Errorf("expected 'section name cannot be empty' error, got: %v", err)
	}
}
