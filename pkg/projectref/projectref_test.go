package projectref

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/h2non/gock"
	"github.com/jtsternberg/asana-cli/internal/api/asana"
)

type obj map[string]any

func mockSections(sections ...obj) {
	gock.New("https://app.asana.com").
		Get("/api/1.0/projects/P1/sections").
		Reply(200).
		JSON(obj{"data": sections})
}

func testProject() *asana.Project {
	p := &asana.Project{ID: "P1"}
	p.Name = "Lindris"
	return p
}

func TestIsGID(t *testing.T) {
	cases := map[string]bool{
		"1211512149485000": true,
		"0":                true,
		"":                 false,
		"Q3 2026":          false,
		"12a":              false,
		"-1":               false,
	}
	for in, want := range cases {
		if got := IsGID(in); got != want {
			t.Errorf("IsGID(%q) = %v; want %v", in, got, want)
		}
	}
}

func TestResolveSection_NonNumericTokenIsNotAGID(t *testing.T) {
	defer gock.Off()
	mockSections(
		obj{"gid": "S1", "name": "Untitled section"},
		obj{"gid": "S2", "name": "Q3 2026 Rocks - Ben"},
	)

	got, err := ResolveSection(asana.NewClient(http.DefaultClient), testProject(), "S2")
	// "S2" is not all-digits, so it resolves by name — and no section is named
	// "S2", so this must fail rather than silently matching by ID.
	if err == nil {
		t.Fatalf("expected a not-found error for a non-gid, non-name token, got %v", got)
	}
}

func TestResolveSection_ByNumericGID(t *testing.T) {
	defer gock.Off()
	mockSections(
		obj{"gid": "1001", "name": "Untitled section"},
		obj{"gid": "1002", "name": "Q3 2026 Rocks - Ben"},
	)

	got, err := ResolveSection(asana.NewClient(http.DefaultClient), testProject(), "1002")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "1002" {
		t.Errorf("resolved %q; want 1002", got.ID)
	}
}

func TestResolveSection_ExactNameBeatsPartial(t *testing.T) {
	defer gock.Off()
	mockSections(
		obj{"gid": "S1", "name": "Done - Q3"},
		obj{"gid": "S2", "name": "Done"},
	)

	got, err := ResolveSection(asana.NewClient(http.DefaultClient), testProject(), "done")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "S2" {
		t.Errorf("resolved %q (%q); want S2 (the exact match)", got.ID, got.Name)
	}
}

// The reason ResolveSection refuses to guess: a real project's section names are
// prefixed variants of one another, and the callers delete and reorder things.
func TestResolveSection_AmbiguousPartialIsAnError(t *testing.T) {
	defer gock.Off()
	mockSections(
		obj{"gid": "S1", "name": "Q3 2026 Rocks - Ben"},
		obj{"gid": "S2", "name": "Q3 2026 Rocks - Alyssa"},
		obj{"gid": "S3", "name": "Untitled section"},
	)

	_, err := ResolveSection(asana.NewClient(http.DefaultClient), testProject(), "Q3 2026 Rocks")
	if err == nil {
		t.Fatal("expected an ambiguity error, got nil — a delete would have hit an arbitrary section")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("error should say the query is ambiguous, got: %v", err)
	}
	// Both candidates and their gids must be named, or the caller cannot recover.
	for _, want := range []string{"Q3 2026 Rocks - Ben", "S1", "Q3 2026 Rocks - Alyssa", "S2"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("ambiguity error missing %q\nGot: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "Untitled section") {
		t.Errorf("ambiguity error listed a non-matching section\nGot: %v", err)
	}
}

func TestResolveSection_NotFound(t *testing.T) {
	defer gock.Off()
	mockSections(obj{"gid": "S1", "name": "Untitled section"})

	_, err := ResolveSection(asana.NewClient(http.DefaultClient), testProject(), "Nope")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected a not-found error, got: %v", err)
	}
}

func TestResolveSection_EmptyName(t *testing.T) {
	_, err := ResolveSection(asana.NewClient(http.DefaultClient), testProject(), "   ")
	if err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("expected an empty-name error, got: %v", err)
	}
}

// ResolveProject must not enumerate the workspace: only typeahead is mocked, so
// a fallback to /workspaces/{id}/projects would hit an unmocked endpoint.
func TestResolveProject_ByNameUsesTypeahead(t *testing.T) {
	defer gock.Off()

	gock.New("https://app.asana.com").
		Get("/api/1.0/workspaces/WS1/typeahead").
		MatchParam("resource_type", "project").
		Reply(200).
		JSON(obj{"data": []obj{{"gid": "P1", "name": "Lindris"}}})

	got, err := ResolveProject(asana.NewClient(http.DefaultClient), &asana.Workspace{ID: "WS1"}, "Lindris")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "P1" {
		t.Errorf("resolved %q; want P1", got.ID)
	}
}

func TestResolveProject_ByGIDFetchesDirectly(t *testing.T) {
	defer gock.Off()

	gock.New("https://app.asana.com").
		Get("/api/1.0/projects/1211512149484999").
		Reply(200).
		JSON(obj{"data": obj{"gid": "1211512149484999", "name": "Lindris"}})

	got, err := ResolveProject(asana.NewClient(http.DefaultClient),
		&asana.Workspace{ID: "WS1"}, "1211512149484999")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "Lindris" {
		t.Errorf("resolved %q; want Lindris", got.Name)
	}
}

func TestFetchAllSections_Paginates(t *testing.T) {
	defer gock.Off()

	gock.New("https://app.asana.com").
		Get("/api/1.0/projects/P1/sections").
		MatchParam("limit", "100").
		Reply(200).
		JSON(obj{
			"data":      []obj{{"gid": "S1", "name": "One"}},
			"next_page": obj{"offset": "abc", "path": "/projects/P1/sections?offset=abc"},
		})

	gock.New("https://app.asana.com").
		Get("/api/1.0/projects/P1/sections").
		MatchParam("offset", "abc").
		Reply(200).
		JSON(obj{"data": []obj{{"gid": "S2", "name": "Two"}}})

	sections, err := FetchAllSections(asana.NewClient(http.DefaultClient), testProject())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) != 2 {
		t.Fatalf("got %d sections; want 2 (pagination dropped a page)", len(sections))
	}
	if !gock.IsDone() {
		t.Errorf("pending mocks: %d", len(gock.Pending()))
	}
}

// --- project ambiguity ---

func project(id, name string) *asana.Project {
	p := &asana.Project{ID: id}
	p.Name = name
	return p
}

// The live shape: 1203 projects, and "rocks" appears in 211 of their names.
// First-match-wins wrote tasks into whichever one sorted first.
func TestFindProject_AmbiguousPartialIsAnError(t *testing.T) {
	projects := []*asana.Project{
		project("P1", "Q3 2026 Rocks - Marketing"),
		project("P2", "Q3 2026 Rocks - Support"),
		project("P3", "Lindris"),
	}

	_, err := FindProject(projects, "Rocks")
	if err == nil {
		t.Fatal("expected an ambiguity error; a silent pick writes to the wrong project")
	}

	var ambiguous AmbiguousError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("want an AmbiguousError so callers can annotate it, got %T", err)
	}
	if ambiguous.Kind != "project" {
		t.Errorf("Kind = %q; want project", ambiguous.Kind)
	}
	for _, want := range []string{"Q3 2026 Rocks - Marketing", "P1", "Q3 2026 Rocks - Support", "P2"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("ambiguity error missing %q\nGot: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "Lindris") {
		t.Errorf("ambiguity error listed a non-matching project\nGot: %v", err)
	}
}

// An exact match still wins, or "Lindris" would be unreachable while
// "Lindris Previous Rocks" exists — both real projects in this workspace.
func TestFindProject_ExactNameBeatsPartial(t *testing.T) {
	projects := []*asana.Project{
		project("P1", "Lindris Previous Rocks"),
		project("P2", "Lindris"),
		project("P3", "Lindris Archive"),
	}

	got, err := FindProject(projects, "lindris")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "P2" {
		t.Errorf("resolved %q (%q); want P2 (the exact match)", got.ID, got.Name)
	}
}

func TestFindProject_UniquePartial(t *testing.T) {
	projects := []*asana.Project{project("P1", "Lindris Previous Rocks"), project("P2", "Something Else")}

	got, err := FindProject(projects, "previous")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "P1" {
		t.Errorf("resolved %q; want P1", got.ID)
	}
}

func TestFindProject_ByGID(t *testing.T) {
	projects := []*asana.Project{project("1001", "Alpha"), project("1002", "Beta")}

	got, err := FindProject(projects, "1002")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "1002" {
		t.Errorf("resolved %q; want 1002", got.ID)
	}
}

func TestFindProject_NotFound(t *testing.T) {
	_, err := FindProject([]*asana.Project{project("P1", "Alpha")}, "Nope")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found, got: %v", err)
	}
}

// A query matching hundreds of projects must not print hundreds of lines — an
// error nobody reads is barely better than a silent wrong guess.
func TestAmbiguousError_ElidesLongCandidateLists(t *testing.T) {
	projects := make([]*asana.Project, 0, 211)
	for i := range 211 {
		projects = append(projects, project(fmt.Sprintf("P%d", i), fmt.Sprintf("Q3 Rocks %d", i)))
	}

	_, err := FindProject(projects, "rocks")
	if err == nil {
		t.Fatal("expected an ambiguity error")
	}

	lines := strings.Count(err.Error(), "\n")
	if lines > maxListedCandidates+3 {
		t.Errorf("error spans %d lines; want it capped near %d", lines, maxListedCandidates)
	}
	if !strings.Contains(err.Error(), "211 projects match") {
		t.Errorf("error should report the true match count\nGot: %v", err)
	}
	if !strings.Contains(err.Error(), "and 201 more") {
		t.Errorf("error should say how many candidates it elided\nGot: %v", err)
	}
}

func TestFindProject_EmptyRef(t *testing.T) {
	_, err := FindProject([]*asana.Project{project("P1", "Alpha")}, "  ")
	if err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("expected an empty-name error, got: %v", err)
	}
}
