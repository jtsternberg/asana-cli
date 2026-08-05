package move

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/h2non/gock"
	"github.com/jtsternberg/asana-cli/internal/api/asana"
	"github.com/jtsternberg/asana-cli/pkg/iostreams"
)

type obj map[string]any

func testProject() *asana.Project {
	p := &asana.Project{ID: "P1"}
	p.Name = "Lindris"
	return p
}

// The real shape that motivated this command: new quarterly sections appended
// below a stale "Untitled section".
func mockLindrisSections() {
	gock.New("https://app.asana.com").
		Get("/api/1.0/projects/P1/sections").
		Reply(200).
		JSON(obj{"data": []obj{
			{"gid": "S1", "name": "Untitled section"},
			{"gid": "S2", "name": "Q3 2026 Rocks - Shared"},
			{"gid": "S3", "name": "Q3 2026 Rocks - Ben"},
		}})
}

// captureInsert mocks the insert endpoint and returns a pointer to the decoded
// request body, so a test can assert on before_section/after_section.
func captureInsert() *map[string]any {
	captured := map[string]any{}
	gock.New("https://app.asana.com").
		Post("/api/1.0/projects/P1/sections/insert").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return false, err
			}
			var envelope struct {
				Data map[string]any `json:"data"`
			}
			if err := json.Unmarshal(body, &envelope); err != nil {
				return false, err
			}
			for k, v := range envelope.Data {
				captured[k] = v
			}
			return true, nil
		}).
		Reply(200).
		JSON(obj{"data": obj{}})
	return &captured
}

func newOpts(io *iostreams.IOStreams) *MoveOptions {
	return &MoveOptions{
		IO:     io,
		Client: func() (*asana.Client, error) { return asana.NewClient(http.DefaultClient), nil },
	}
}

func run(t *testing.T, opts *MoveOptions) error {
	t.Helper()
	return moveSection(opts, asana.NewClient(http.DefaultClient), testProject())
}

func TestMoveSection_FirstInsertsBeforeCurrentFirst(t *testing.T) {
	defer gock.Off()
	mockLindrisSections()
	body := captureInsert()

	io, _, out, _ := iostreams.Test()
	opts := newOpts(io)
	opts.SectionName = "Q3 2026 Rocks - Shared"
	opts.First = true

	if err := run(t, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := (*body)["before_section"]; got != "S1" {
		t.Errorf("before_section = %v; want S1 (the section currently at the top)", got)
	}
	if _, ok := (*body)["after_section"]; ok {
		t.Errorf("after_section must be omitted when moving to the top; body: %v", *body)
	}
	if got := (*body)["section"]; got != "S2" {
		t.Errorf("section = %v; want S2", got)
	}
	if output := out.String(); !strings.Contains(output, "top") {
		t.Errorf("output should say where it went\nGot:\n%s", output)
	}
}

func TestMoveSection_LastInsertsAfterCurrentLast(t *testing.T) {
	defer gock.Off()
	mockLindrisSections()
	body := captureInsert()

	io, _, _, _ := iostreams.Test()
	opts := newOpts(io)
	opts.SectionName = "Untitled section"
	opts.Last = true

	if err := run(t, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := (*body)["after_section"]; got != "S3" {
		t.Errorf("after_section = %v; want S3 (the section currently at the bottom)", got)
	}
	if _, ok := (*body)["before_section"]; ok {
		t.Errorf("before_section must be omitted when moving to the bottom; body: %v", *body)
	}
}

func TestMoveSection_BeforeAndAfterResolveAnchorsByName(t *testing.T) {
	defer gock.Off()
	mockLindrisSections()
	body := captureInsert()

	io, _, _, _ := iostreams.Test()
	opts := newOpts(io)
	opts.SectionName = "Q3 2026 Rocks - Ben"
	opts.Before = "Untitled section"

	if err := run(t, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := (*body)["before_section"]; got != "S1" {
		t.Errorf("before_section = %v; want S1", got)
	}
}

func TestMoveSection_AfterByGID(t *testing.T) {
	defer gock.Off()
	gock.New("https://app.asana.com").
		Get("/api/1.0/projects/P1/sections").
		Reply(200).
		JSON(obj{"data": []obj{
			{"gid": "1001", "name": "Untitled section"},
			{"gid": "1002", "name": "Q3 2026 Rocks - Shared"},
		}})
	body := captureInsert()

	io, _, _, _ := iostreams.Test()
	opts := newOpts(io)
	opts.SectionName = "1002"
	opts.After = "1001"

	if err := run(t, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := (*body)["after_section"]; got != "1001" {
		t.Errorf("after_section = %v; want 1001", got)
	}
}

// No destination is a usage error, not a silent no-op — otherwise a caller that
// forgot the flag would think the move happened.
func TestMoveSection_RequiresADestination(t *testing.T) {
	defer gock.Off()

	io, _, _, _ := iostreams.Test()
	opts := newOpts(io)
	opts.SectionName = "Q3 2026 Rocks - Shared"

	err := run(t, opts)
	if err == nil {
		t.Fatal("expected an error when no destination flag is given")
	}
	for _, want := range []string{"--first", "--last", "--before", "--after"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should name %s\nGot: %v", want, err)
		}
	}
}

// Already-in-position must not issue a request: Asana rejects an insert that
// anchors a section against itself.
func TestMoveSection_AlreadyFirstIssuesNoRequest(t *testing.T) {
	defer gock.Off()
	mockLindrisSections()

	io, _, out, _ := iostreams.Test()
	opts := newOpts(io)
	opts.SectionName = "Untitled section"
	opts.First = true

	if err := run(t, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output := out.String(); !strings.Contains(output, "already in position") {
		t.Errorf("output should say it was already in position\nGot:\n%s", output)
	}
	// gock has no insert mock registered; had a request been made it would have
	// failed the call above.
}

func TestMoveSection_AnchorCannotBeTheSectionItself(t *testing.T) {
	defer gock.Off()
	mockLindrisSections()

	io, _, _, _ := iostreams.Test()
	opts := newOpts(io)
	opts.SectionName = "Q3 2026 Rocks - Ben"
	opts.After = "Q3 2026 Rocks - Ben"

	err := run(t, opts)
	if err == nil || !strings.Contains(err.Error(), "being moved") {
		t.Fatalf("expected a self-anchor error, got: %v", err)
	}
}

func TestMoveSection_AmbiguousAnchorRefuses(t *testing.T) {
	defer gock.Off()
	mockLindrisSections()

	io, _, _, _ := iostreams.Test()
	opts := newOpts(io)
	opts.SectionName = "Untitled section"
	opts.After = "Q3 2026 Rocks"

	err := run(t, opts)
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected an ambiguity error for the anchor, got: %v", err)
	}
	if !strings.Contains(err.Error(), "--after") {
		t.Errorf("error should say which flag was ambiguous\nGot: %v", err)
	}
}

func TestMoveSection_JSONOutput(t *testing.T) {
	defer gock.Off()
	mockLindrisSections()
	captureInsert()

	io, _, out, _ := iostreams.Test()
	opts := newOpts(io)
	opts.SectionName = "Q3 2026 Rocks - Shared"
	opts.First = true
	opts.JSON = true

	if err := run(t, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}
	if result["id"] != "S2" {
		t.Errorf("id = %v; want S2", result["id"])
	}
	if result["project_id"] != "P1" {
		t.Errorf("project_id = %v; want P1", result["project_id"])
	}
	if moved, _ := result["moved"].(string); !strings.Contains(moved, "top") {
		t.Errorf("moved = %q; want it to describe the destination", moved)
	}
}
