package list

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/internal/config"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/iostreams"
)

func TestNewCmdList_RunE(t *testing.T) {
	f, _, _ := factory.NewTestFactory()

	var sawOpts *ListOptions
	cmd := NewCmdList(f, func(opts *ListOptions) error {
		sawOpts = opts
		return nil
	})

	cmd.SetArgs([]string{"--limit", "5", "--favorite"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if sawOpts == nil {
		t.Fatal("runF was never called")
	}
	if sawOpts.Limit != 5 {
		t.Errorf("Limit = %d; want 5", sawOpts.Limit)
	}
	if !sawOpts.Favorite {
		t.Error("Favorite = false; want true")
	}
}

func TestNewCmdList_RunE_JSONFlag(t *testing.T) {
	f, _, _ := factory.NewTestFactory()

	var sawOpts *ListOptions
	cmd := NewCmdList(f, func(opts *ListOptions) error {
		sawOpts = opts
		return nil
	})

	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if sawOpts == nil {
		t.Fatal("runF was never called")
	}
	if !sawOpts.JSON {
		t.Error("JSON = false; want true")
	}
}

func TestNewCmdList_RunE_InvalidLimit(t *testing.T) {
	f, _, _ := factory.NewTestFactory()
	cmd := NewCmdList(f, func(opts *ListOptions) error { return nil })
	cmd.SetArgs([]string{"--limit", "-1"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "invalid limit") {
		t.Fatalf("expected invalid-limit error, got %v", err)
	}
}

func TestRunList_ConfigError(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	opts := &ListOptions{
		IO:     io,
		Config: func() (*config.Config, error) { return nil, errors.New("no config") },
		Client: func() (*asana.Client, error) { return nil, nil },
	}
	if err := runList(opts); err == nil || !strings.Contains(err.Error(), "no config") {
		t.Fatalf("expected config error, got %v", err)
	}
}

func TestRunList_ClientError(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		Config: func() (*config.Config, error) {
			return &config.Config{Workspace: &asana.Workspace{ID: "W"}}, nil
		},
		Client: func() (*asana.Client, error) { return nil, errors.New("auth failed") },
	}
	if err := runList(opts); err == nil || !strings.Contains(err.Error(), "auth failed") {
		t.Fatalf("expected client error, got %v", err)
	}
}

type transportFunc func(*http.Request) (*http.Response, error)

func (fn transportFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func newTestClient(mock *asana.MockClient) *asana.Client {
	httpClient := &http.Client{
		Transport: transportFunc(mock.Do),
	}
	return asana.NewClient(httpClient)
}

func TestNewCmdList_SearchFlag(t *testing.T) {
	f, _, _ := factory.NewTestFactory()

	var sawOpts *ListOptions
	cmd := NewCmdList(f, func(opts *ListOptions) error {
		sawOpts = opts
		return nil
	})

	cmd.SetArgs([]string{"--search", "outgoing"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if sawOpts == nil {
		t.Fatal("runF was never called")
	}
	if sawOpts.Search != "outgoing" {
		t.Errorf("Search = %q; want %q", sawOpts.Search, "outgoing")
	}
}

func TestNewCmdList_SearchShortFlag(t *testing.T) {
	f, _, _ := factory.NewTestFactory()

	var sawOpts *ListOptions
	cmd := NewCmdList(f, func(opts *ListOptions) error {
		sawOpts = opts
		return nil
	})

	cmd.SetArgs([]string{"-q", "tasks"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if sawOpts == nil {
		t.Fatal("runF was never called")
	}
	if sawOpts.Search != "tasks" {
		t.Errorf("Search = %q; want %q", sawOpts.Search, "tasks")
	}
}

func TestRunList_JSONOutput(t *testing.T) {
	mockProjects := []*asana.Project{
		{ID: "111", ProjectBase: asana.ProjectBase{Name: "Alpha"}},
		{ID: "222", ProjectBase: asana.ProjectBase{Name: "Beta"}},
	}
	mock, err := asana.NewMockClient(200, mockProjects)
	if err != nil {
		t.Fatalf("NewMockClient: %v", err)
	}
	client := newTestClient(mock)

	io, _, out, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		Config: func() (*config.Config, error) {
			return &config.Config{Workspace: &asana.Workspace{ID: "W"}}, nil
		},
		Client: func() (*asana.Client, error) { return client, nil },
		JSON:   true,
	}

	if err := runList(opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got []map[string]string
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", err, out.String())
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(got))
	}
	if got[0]["id"] != "111" || got[0]["name"] != "Alpha" {
		t.Errorf("project[0] = %v; want id=111 name=Alpha", got[0])
	}
	if got[1]["id"] != "222" || got[1]["name"] != "Beta" {
		t.Errorf("project[1] = %v; want id=222 name=Beta", got[1])
	}
}
