package list

import (
	"net/http"
	"strings"
	"testing"

	"github.com/h2non/gock"
	"github.com/jtsternberg/asana-cli/internal/api/asana"
)

// is_organization is not in the compact workspace record, so the type column
// printed "Workspace" for every workspace — including organizations. This
// workspace really is an organization.
func TestAllWorkspaces_RequestsIsOrganization(t *testing.T) {
	defer gock.Off()

	var optFields string
	gock.New("https://app.asana.com").
		Get("/api/1.0/workspaces").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			if optFields == "" {
				optFields = req.URL.Query().Get("opt_fields")
			}
			return true, nil
		}).
		Reply(200).
		JSON(map[string]any{"data": []map[string]any{
			{"gid": "WS1", "name": "acme.com", "is_organization": true},
		}})

	workspaces, err := asana.NewClient(http.DefaultClient).AllWorkspaces(
		&asana.Options{Fields: []string{"name", "is_organization"}},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(optFields, "is_organization") {
		t.Errorf("opt_fields = %q; want it to include is_organization", optFields)
	}
	if len(workspaces) != 1 || !workspaces[0].IsOrganization {
		t.Errorf("is_organization did not reach the workspace: %+v", workspaces)
	}
}
