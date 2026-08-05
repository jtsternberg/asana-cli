package list

import (
	"net/http"
	"strings"
	"testing"

	"github.com/h2non/gock"
	"github.com/jtsternberg/asana-cli/internal/api/asana"
)

// /workspaces/{gid}/tags returns compact records, so the color column printed "-"
// for every tag until this list was requested explicitly.
func TestFetchTags_RequestsColor(t *testing.T) {
	defer gock.Off()

	var optFields string
	gock.New("https://app.asana.com").
		Get("/api/1.0/workspaces/WS1$").
		Reply(200).
		JSON(map[string]any{"data": map[string]any{"gid": "WS1", "name": "acme"}})

	gock.New("https://app.asana.com").
		Get("/api/1.0/workspaces/WS1/tags").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			optFields = req.URL.Query().Get("opt_fields")
			return true, nil
		}).
		Reply(200).
		JSON(map[string]any{"data": []map[string]any{
			{"gid": "T1", "name": "urgent", "color": "dark-red"},
		}})

	tags, err := fetchTags(asana.NewClient(http.DefaultClient), &asana.Workspace{ID: "WS1"}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(optFields, "color") {
		t.Errorf("opt_fields = %q; want it to include color", optFields)
	}
	if len(tags) != 1 || tags[0].Color != "dark-red" {
		t.Errorf("color did not reach the tag: %+v", tags)
	}
}

// The paging loop advances options.Offset, so the request has to actually carry
// options — a call that ignores them re-reads page one.
func TestFetchTags_SendsPagingOptions(t *testing.T) {
	defer gock.Off()

	gock.New("https://app.asana.com").
		Get("/api/1.0/workspaces/WS1$").
		Reply(200).
		JSON(map[string]any{"data": map[string]any{"gid": "WS1", "name": "acme"}})

	gock.New("https://app.asana.com").
		Get("/api/1.0/workspaces/WS1/tags").
		MatchParam("limit", "100").
		Reply(200).
		JSON(map[string]any{
			"data":      []map[string]any{{"gid": "T1", "name": "one"}},
			"next_page": map[string]any{"offset": "abc", "path": "/tags?offset=abc"},
		})

	gock.New("https://app.asana.com").
		Get("/api/1.0/workspaces/WS1/tags").
		MatchParam("offset", "abc").
		Reply(200).
		JSON(map[string]any{"data": []map[string]any{{"gid": "T2", "name": "two"}}})

	tags, err := fetchTags(asana.NewClient(http.DefaultClient), &asana.Workspace{ID: "WS1"}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("got %d tags; want 2 — the offset never reached the API", len(tags))
	}
	if !gock.IsDone() {
		t.Errorf("pending mocks: %d", len(gock.Pending()))
	}
}
