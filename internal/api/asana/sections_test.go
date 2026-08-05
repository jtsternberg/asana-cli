package asana

import (
	"encoding/json"
	stdio "io"
	"net/http"
	"testing"

	"github.com/h2non/gock"
)

// Asana's docs list `project` among the insert endpoint's body fields, but the
// project is already a path parameter — sending both makes the API reject the
// request with "400: Duplicate field: project". Verified live against
// /projects/{gid}/sections/insert.
func TestInsertSection_OmitsProjectFromBody(t *testing.T) {
	defer gock.Off()

	var body map[string]any
	gock.New("https://app.asana.com").
		Post("/api/1.0/projects/P1/sections/insert").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			raw, err := stdio.ReadAll(req.Body)
			if err != nil {
				return false, err
			}
			var envelope struct {
				Data map[string]any `json:"data"`
			}
			if err := json.Unmarshal(raw, &envelope); err != nil {
				return false, err
			}
			body = envelope.Data
			return true, nil
		}).
		Reply(200).
		JSON(map[string]any{"data": map[string]any{}})

	project := &Project{ID: "P1"}
	err := project.InsertSection(NewClient(http.DefaultClient), &SectionInsertRequest{
		Section:       "S2",
		BeforeSection: "S1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, present := body["project"]; present {
		t.Errorf("request body carries a project field; Asana answers 400 Duplicate field: project\nbody: %v", body)
	}
	if body["section"] != "S2" {
		t.Errorf("section = %v; want S2", body["section"])
	}
	if body["before_section"] != "S1" {
		t.Errorf("before_section = %v; want S1", body["before_section"])
	}
}

// The path needs its leading slash: getURL panics without one, which is how
// InsertSection managed to be unreachable rather than merely wrong.
func TestInsertSection_PathHasLeadingSlash(t *testing.T) {
	defer gock.Off()

	gock.New("https://app.asana.com").
		Post("/api/1.0/projects/P1/sections/insert").
		Reply(200).
		JSON(map[string]any{"data": map[string]any{}})

	project := &Project{ID: "P1"}
	if err := project.InsertSection(NewClient(http.DefaultClient),
		&SectionInsertRequest{Section: "S2", AfterSection: "S1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
