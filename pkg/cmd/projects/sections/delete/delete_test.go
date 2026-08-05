package delete

import (
	"encoding/json"
	stdio "io"
	"net/http"
	"strings"
	"testing"

	"github.com/h2non/gock"
	"github.com/jtsternberg/asana-cli/internal/api/asana"
	"github.com/jtsternberg/asana-cli/internal/prompter"
	"github.com/jtsternberg/asana-cli/pkg/iostreams"
)

type obj map[string]any

func testProject() *asana.Project {
	p := &asana.Project{ID: "P1"}
	p.Name = "Lindris"
	return p
}

func mockSections(sections ...obj) {
	gock.New("https://app.asana.com").
		Get("/api/1.0/projects/P1/sections").
		Reply(200).
		JSON(obj{"data": sections})
}

func mockSectionTasks(sectionID string, tasks ...obj) {
	gock.New("https://app.asana.com").
		Get("/api/1.0/sections/" + sectionID + "/tasks").
		Reply(200).
		JSON(obj{"data": tasks})
}

// confirmer answers every confirmation prompt with the same canned reply and
// records how many times it was asked.
type confirmer struct {
	prompter.Prompter
	reply bool
	asked int
}

func (c *confirmer) Confirm(string, string) (bool, error) {
	c.asked++
	return c.reply, nil
}

func newOpts(io *iostreams.IOStreams, p prompter.Prompter) *DeleteOptions {
	return &DeleteOptions{
		IO:       io,
		Prompter: p,
		Client:   func() (*asana.Client, error) { return asana.NewClient(http.DefaultClient), nil },
	}
}

// A section that still holds tasks must be refused locally, with the task count
// named, rather than deferring to Asana's server-side error. The caller needs to
// know *why* before it decides whether to move the tasks or force through.
func TestDeleteSection_RefusesNonEmptySection(t *testing.T) {
	defer gock.Off()
	mockSections(obj{"gid": "S1", "name": "Q3 Rocks"})
	mockSectionTasks("S1", obj{"gid": "T1", "name": "Ship it"}, obj{"gid": "T2", "name": "Test it"})

	io, _, _, _ := iostreams.Test()
	p := &confirmer{reply: true}
	opts := newOpts(io, p)
	opts.SectionName = "Q3 Rocks"

	err := deleteSection(opts, asana.NewClient(http.DefaultClient), testProject())
	if err == nil {
		t.Fatal("expected a refusal for a section that still has tasks")
	}
	if !strings.Contains(err.Error(), "2 task") {
		t.Errorf("error should name the task count, got: %v", err)
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error should name the escape hatch, got: %v", err)
	}
	if p.asked != 0 {
		t.Errorf("prompted %d times; a refused delete must not ask for confirmation first", p.asked)
	}
}

func TestDeleteSection_ForceBypassesEmptinessCheck(t *testing.T) {
	defer gock.Off()
	mockSections(obj{"gid": "S1", "name": "Q3 Rocks"})
	gock.New("https://app.asana.com").
		Delete("/api/1.0/sections/S1").
		Reply(200).
		JSON(obj{"data": obj{}})

	io, _, _, _ := iostreams.Test()
	opts := newOpts(io, &confirmer{reply: true})
	opts.SectionName = "Q3 Rocks"
	opts.Force = true

	if err := deleteSection(opts, asana.NewClient(http.DefaultClient), testProject()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !gock.IsDone() {
		t.Errorf("expected the DELETE to be issued; pending mocks: %d", len(gock.Pending()))
	}
}

// --force is about the emptiness check, not about consent: with tasks present
// it must still delete, and the tasks-would-be-orphaned risk is why the prompt
// still happens unless --yes is also given.
func TestDeleteSection_ForceStillConfirms(t *testing.T) {
	defer gock.Off()
	mockSections(obj{"gid": "S1", "name": "Q3 Rocks"})

	io, _, _, _ := iostreams.Test()
	p := &confirmer{reply: false}
	opts := newOpts(io, p)
	opts.SectionName = "Q3 Rocks"
	opts.Force = true

	err := deleteSection(opts, asana.NewClient(http.DefaultClient), testProject())
	if err == nil {
		t.Fatal("expected cancellation when the user declines")
	}
	if p.asked != 1 {
		t.Errorf("asked %d times; want exactly 1", p.asked)
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("expected a cancellation error, got: %v", err)
	}
}

func TestDeleteSection_EmptySectionDeletesAfterConfirmation(t *testing.T) {
	defer gock.Off()
	mockSections(obj{"gid": "S1", "name": "Q3 Rocks"})
	mockSectionTasks("S1")
	gock.New("https://app.asana.com").
		Delete("/api/1.0/sections/S1").
		Reply(200).
		JSON(obj{"data": obj{}})

	io, _, out, _ := iostreams.Test()
	p := &confirmer{reply: true}
	opts := newOpts(io, p)
	opts.SectionName = "Q3 Rocks"

	if err := deleteSection(opts, asana.NewClient(http.DefaultClient), testProject()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.asked != 1 {
		t.Errorf("asked %d times; want exactly 1", p.asked)
	}
	if !gock.IsDone() {
		t.Errorf("expected the DELETE to be issued; pending mocks: %d", len(gock.Pending()))
	}
	if output := out.String(); !strings.Contains(output, "Q3 Rocks") {
		t.Errorf("output should name the deleted section\nGot:\n%s", output)
	}
}

// --yes exists so an agent can script this. Skipping the prompt must not skip
// the emptiness check.
func TestDeleteSection_YesSkipsPromptButNotEmptinessCheck(t *testing.T) {
	defer gock.Off()
	mockSections(obj{"gid": "S1", "name": "Q3 Rocks"})
	mockSectionTasks("S1", obj{"gid": "T1", "name": "Ship it"})

	io, _, _, _ := iostreams.Test()
	p := &confirmer{reply: true}
	opts := newOpts(io, p)
	opts.SectionName = "Q3 Rocks"
	opts.Yes = true

	err := deleteSection(opts, asana.NewClient(http.DefaultClient), testProject())
	if err == nil || !strings.Contains(err.Error(), "1 task") {
		t.Fatalf("expected the emptiness refusal even with --yes, got: %v", err)
	}
	if p.asked != 0 {
		t.Errorf("--yes must not prompt; asked %d times", p.asked)
	}
}

func TestDeleteSection_JSONOutput(t *testing.T) {
	defer gock.Off()
	mockSections(obj{"gid": "S1", "name": "Q3 Rocks"})
	mockSectionTasks("S1")
	gock.New("https://app.asana.com").
		Delete("/api/1.0/sections/S1").
		Reply(200).
		JSON(obj{"data": obj{}})

	io, _, out, _ := iostreams.Test()
	opts := newOpts(io, &confirmer{reply: true})
	opts.SectionName = "Q3 Rocks"
	opts.Yes = true
	opts.JSON = true

	if err := deleteSection(opts, asana.NewClient(http.DefaultClient), testProject()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}
	if result["id"] != "S1" {
		t.Errorf("id = %v; want S1", result["id"])
	}
	if result["deleted"] != true {
		t.Errorf("deleted = %v; want true", result["deleted"])
	}
	if result["project_id"] != "P1" {
		t.Errorf("project_id = %v; want P1", result["project_id"])
	}
}

// An ambiguous name must not resolve to an arbitrary section — deleting the
// wrong one is unrecoverable from the CLI.
func TestDeleteSection_AmbiguousNameRefuses(t *testing.T) {
	defer gock.Off()
	mockSections(
		obj{"gid": "S1", "name": "Q3 2026 Rocks - Ben"},
		obj{"gid": "S2", "name": "Q3 2026 Rocks - Alyssa"},
	)

	io, _, _, _ := iostreams.Test()
	p := &confirmer{reply: true}
	opts := newOpts(io, p)
	opts.SectionName = "Q3 2026 Rocks"
	opts.Yes = true

	err := deleteSection(opts, asana.NewClient(http.DefaultClient), testProject())
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected an ambiguity error, got: %v", err)
	}
	if p.asked != 0 {
		t.Errorf("must not prompt before the section is unambiguously resolved; asked %d times", p.asked)
	}
}

// Non-interactive callers with neither --yes nor a usable stdin must get a clear
// instruction rather than a hang or an opaque prompt error.
func TestDeleteSection_NoInputNamesTheFlag(t *testing.T) {
	defer gock.Off()
	mockSections(obj{"gid": "S1", "name": "Q3 Rocks"})
	mockSectionTasks("S1")

	io, _, _, _ := iostreams.Test()
	opts := newOpts(io, &noInputPrompter{})
	opts.SectionName = "Q3 Rocks"

	err := deleteSection(opts, asana.NewClient(http.DefaultClient), testProject())
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected an error naming --yes, got: %v", err)
	}
}

type noInputPrompter struct {
	prompter.Prompter
}

func (noInputPrompter) Confirm(string, string) (bool, error) {
	return false, stdio.EOF
}
