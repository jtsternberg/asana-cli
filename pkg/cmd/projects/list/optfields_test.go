package list

import (
	"net/http"
	"strings"
	"testing"

	"github.com/h2non/gock"
	"github.com/jtsternberg/asana-cli/internal/api/asana"
	"github.com/jtsternberg/asana-cli/pkg/cmd/projects/shared"
)

// The typeahead endpoint behind `projects list -q` returns compact records, so
// the owner/team column silently vanished for query results while the plain
// listing showed it — the same data rendering two different-looking answers.
func TestSearchProjects_RequestsTheSameFieldsAsTheFullListing(t *testing.T) {
	defer gock.Off()

	var optFields string
	gock.New("https://app.asana.com").
		Get("/api/1.0/workspaces/WS1/typeahead").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			optFields = req.URL.Query().Get("opt_fields")
			return true, nil
		}).
		Reply(200).
		JSON(map[string]any{"data": []map[string]any{
			{"gid": "P1", "name": "Lindris", "owner": map[string]any{"gid": "U1", "name": "Justin"}},
		}})

	projects, err := (&asana.Workspace{ID: "WS1"}).SearchProjects(
		asana.NewClient(http.DefaultClient), "Lindris", 100,
		&asana.Options{Fields: shared.ProjectFields()},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range shared.ProjectFields() {
		if !strings.Contains(optFields, want) {
			t.Errorf("opt_fields is missing %q (got %q)", want, optFields)
		}
	}
	if len(projects) != 1 || projects[0].Owner == nil {
		t.Errorf("owner did not reach the project: %+v", projects)
	}
}
