package shared

import (
	"net/http"
	"testing"

	"github.com/h2non/gock"
	"github.com/timwehrle/asana/internal/api/asana"
)

type obj map[string]any

// TestFetchAllProjects_NeverSendsUnboundedRequest guards the large-workspace
// bug: FetchAllProjects must always request a bounded page (limit=100) so the
// Asana API never returns "400: The result is too large". Passing limit=0
// ("no total cap") must NOT translate into an unbounded first request.
func TestFetchAllProjects_NeverSendsUnboundedRequest(t *testing.T) {
	defer gock.Off()

	gock.New("https://app.asana.com").
		Get("/api/1.0/workspaces/WS1/projects").
		MatchParam("limit", "100").
		Reply(200).
		JSON(obj{"data": []obj{
			{"gid": "p1", "name": "Alpha"},
			{"gid": "p2", "name": "Beta"},
		}})

	client := asana.NewClient(http.DefaultClient)
	projects, err := FetchAllProjects(client, &asana.Workspace{ID: "WS1"}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}
	if !gock.IsDone() {
		t.Errorf("expected the limit=100 request to be made; pending mocks: %d", len(gock.Pending()))
	}
}

// TestFetchAllProjects_LimitCapsTotal verifies that a positive limit caps the
// total number of results returned, independent of page size.
func TestFetchAllProjects_LimitCapsTotal(t *testing.T) {
	defer gock.Off()

	page := make([]obj, 100)
	for i := range page {
		page[i] = obj{"gid": "p", "name": "P"}
	}

	// A cap below the page maximum is itself a valid bounded page size.
	gock.New("https://app.asana.com").
		Get("/api/1.0/workspaces/WS1/projects").
		MatchParam("limit", "25").
		Reply(200).
		JSON(obj{"data": page})

	client := asana.NewClient(http.DefaultClient)
	projects, err := FetchAllProjects(client, &asana.Workspace{ID: "WS1"}, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 25 {
		t.Fatalf("expected results capped at 25, got %d", len(projects))
	}
}
