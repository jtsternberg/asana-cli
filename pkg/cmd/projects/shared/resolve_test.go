package shared

import (
	"net/http"
	"strings"
	"testing"

	"github.com/h2non/gock"
	"github.com/jtsternberg/asana-cli/internal/api/asana"
)

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
