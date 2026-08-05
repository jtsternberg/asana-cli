package list

import (
	"net/http"
	"strings"
	"testing"

	"github.com/h2non/gock"
	"github.com/jtsternberg/asana-cli/internal/api/asana"
)

// /organizations/{gid}/teams returns compact records, so 17 of 61 real team
// descriptions printed as empty strings until this list was requested.
func TestAllTeams_RequestsDescription(t *testing.T) {
	defer gock.Off()

	var optFields string
	gock.New("https://app.asana.com").
		Get("/api/1.0/organizations/WS1/teams").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			if optFields == "" {
				optFields = req.URL.Query().Get("opt_fields")
			}
			return true, nil
		}).
		Reply(200).
		JSON(map[string]any{"data": []map[string]any{
			{"gid": "T1", "name": "Engineering", "description": "We build things"},
		}})

	teams, err := (&asana.Workspace{ID: "WS1"}).AllTeams(
		asana.NewClient(http.DefaultClient),
		&asana.Options{Fields: []string{"name", "description", "organization", "organization.name"}},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"description", "organization"} {
		if !strings.Contains(optFields, want) {
			t.Errorf("opt_fields = %q; want it to include %q", optFields, want)
		}
	}
	if len(teams) != 1 || teams[0].Description == "" {
		t.Errorf("description did not reach the team: %+v", teams)
	}
}
