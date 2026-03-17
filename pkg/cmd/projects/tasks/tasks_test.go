package tasks

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/iostreams"
)

func TestNewCmdTasks_LimitFlag(t *testing.T) {
	f, _, _ := factory.NewTestFactory()

	var sawOpts *TasksOptions
	cmd := NewCmdTasks(f, func(opts *TasksOptions) error {
		sawOpts = opts
		return nil
	})

	cmd.SetArgs([]string{"--limit", "50"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if sawOpts == nil {
		t.Fatal("runF was never called")
	}

	if sawOpts.Limit != 50 {
		t.Errorf("Limit = %d; want 50", sawOpts.Limit)
	}
}

func TestNewCmdTasks_SectionsAndLimitFlags(t *testing.T) {
	f, _, _ := factory.NewTestFactory()

	var sawOpts *TasksOptions
	cmd := NewCmdTasks(f, func(opts *TasksOptions) error {
		sawOpts = opts
		return nil
	})

	cmd.SetArgs([]string{"--sections", "--limit", "100"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if sawOpts == nil {
		t.Fatal("runF was never called")
	}

	if !sawOpts.WithSections {
		t.Errorf("WithSections = false; want true")
	}

	if sawOpts.Limit != 100 {
		t.Errorf("Limit = %d; want 100", sawOpts.Limit)
	}
}

func TestNewCmdTasks_DefaultLimit(t *testing.T) {
	f, _, _ := factory.NewTestFactory()

	var sawOpts *TasksOptions
	cmd := NewCmdTasks(f, func(opts *TasksOptions) error {
		sawOpts = opts
		return nil
	})

	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if sawOpts == nil {
		t.Fatal("runF was never called")
	}

	if sawOpts.Limit != 0 {
		t.Errorf("Limit = %d; want 0 (no limit)", sawOpts.Limit)
	}
}

func TestNewCmdTasks_NegativeLimit(t *testing.T) {
	f, _, _ := factory.NewTestFactory()
	cmd := NewCmdTasks(f, func(opts *TasksOptions) error { return nil })
	cmd.SetArgs([]string{"--limit", "-1"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "invalid limit") {
		t.Fatalf("expected invalid-limit error, got %v", err)
	}
}

func TestNewCmdTasks_ProjectNameArg(t *testing.T) {
	f, _, _ := factory.NewTestFactory()

	var sawOpts *TasksOptions
	cmd := NewCmdTasks(f, func(opts *TasksOptions) error {
		sawOpts = opts
		return nil
	})

	cmd.SetArgs([]string{"My Project"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if sawOpts == nil {
		t.Fatal("runF was never called")
	}

	if sawOpts.ProjectName != "My Project" {
		t.Errorf("ProjectName = %q; want %q", sawOpts.ProjectName, "My Project")
	}
}

func TestNewCmdTasks_JSONFlag(t *testing.T) {
	f, _, _ := factory.NewTestFactory()

	var sawOpts *TasksOptions
	cmd := NewCmdTasks(f, func(opts *TasksOptions) error {
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
		t.Errorf("JSON = false; want true")
	}
}

// --- Pagination behavior tests ---

type transportFunc func(*http.Request) (*http.Response, error)

func (fn transportFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

// jsonResponse builds a raw JSON API response with optional next_page
func jsonResponse(data any, nextPage *asana.NextPage) []byte {
	resp := map[string]any{"data": data}
	if nextPage != nil {
		resp["next_page"] = nextPage
	}
	b, _ := json.Marshal(resp)
	return b
}

func newMockClient(doFunc func(*http.Request) (*http.Response, error)) *asana.Client {
	return asana.NewClient(&http.Client{
		Transport: transportFunc(doFunc),
	})
}

func TestListAllTasks_Pagination(t *testing.T) {
	callCount := 0
	client := newMockClient(func(req *http.Request) (*http.Response, error) {
		callCount++
		var body []byte
		switch callCount {
		case 1:
			body = jsonResponse(
				[]map[string]string{{"gid": "1", "name": "Task A"}, {"gid": "2", "name": "Task B"}},
				&asana.NextPage{Offset: "page2"},
			)
		default:
			body = jsonResponse(
				[]map[string]string{{"gid": "3", "name": "Task C"}},
				nil,
			)
		}
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBuffer(body)),
			Header:     make(http.Header),
		}, nil
	})

	io, _, outBuf, _ := iostreams.Test()
	opts := &TasksOptions{IO: io, Limit: 0}
	project := &asana.Project{ID: "P1", ProjectBase: asana.ProjectBase{Name: "Test"}}

	if err := listAllTasks(opts, client, project); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 2 {
		t.Errorf("expected 2 API calls, got %d", callCount)
	}

	out := outBuf.String()
	if !strings.Contains(out, "Task A") || !strings.Contains(out, "Task C") {
		t.Errorf("expected all tasks in output, got: %s", out)
	}
}

func TestListAllTasks_LimitEnforced(t *testing.T) {
	callCount := 0
	client := newMockClient(func(req *http.Request) (*http.Response, error) {
		callCount++
		// Return 3 tasks on first page, pagination available
		body := jsonResponse(
			[]map[string]string{{"gid": "1", "name": "A"}, {"gid": "2", "name": "B"}, {"gid": "3", "name": "C"}},
			&asana.NextPage{Offset: "page2"},
		)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBuffer(body)),
			Header:     make(http.Header),
		}, nil
	})

	io, _, outBuf, _ := iostreams.Test()
	opts := &TasksOptions{IO: io, Limit: 2}
	project := &asana.Project{ID: "P1", ProjectBase: asana.ProjectBase{Name: "Test"}}

	if err := listAllTasks(opts, client, project); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should stop after first page because limit was reached
	if callCount != 1 {
		t.Errorf("expected 1 API call (limit hit), got %d", callCount)
	}

	// Should only show 2 tasks
	out := outBuf.String()
	if !strings.Contains(out, "1.") || !strings.Contains(out, "2.") {
		t.Errorf("expected 2 numbered tasks, got: %s", out)
	}
	if strings.Contains(out, "3.") {
		t.Errorf("task 3 should have been truncated, got: %s", out)
	}
}

func TestListAllTasks_JSONOutput(t *testing.T) {
	client := newMockClient(func(req *http.Request) (*http.Response, error) {
		body := jsonResponse(
			[]map[string]string{{"gid": "42", "name": "JSON Task"}},
			nil,
		)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBuffer(body)),
			Header:     make(http.Header),
		}, nil
	})

	io, _, outBuf, _ := iostreams.Test()
	opts := &TasksOptions{IO: io, Limit: 0, JSON: true}
	project := &asana.Project{ID: "P1", ProjectBase: asana.ProjectBase{Name: "Test"}}

	if err := listAllTasks(opts, client, project); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result []jsonTask
	if err := json.Unmarshal(outBuf.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v\noutput: %s", err, outBuf.String())
	}

	if len(result) != 1 || result[0].ID != "42" || result[0].Name != "JSON Task" {
		t.Errorf("unexpected JSON result: %+v", result)
	}
}

func TestFindProject_ExactMatch(t *testing.T) {
	projects := []*asana.Project{
		{ID: "1", ProjectBase: asana.ProjectBase{Name: "Alpha"}},
		{ID: "2", ProjectBase: asana.ProjectBase{Name: "Beta"}},
	}

	p, err := findProject(projects, "Beta")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "2" {
		t.Errorf("expected project ID 2, got %s", p.ID)
	}
}

func TestFindProject_IDMatch(t *testing.T) {
	projects := []*asana.Project{
		{ID: "123", ProjectBase: asana.ProjectBase{Name: "Alpha"}},
	}

	p, err := findProject(projects, "123")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Alpha" {
		t.Errorf("expected Alpha, got %s", p.Name)
	}
}

func TestFindProject_FuzzyMatch(t *testing.T) {
	projects := []*asana.Project{
		{ID: "1", ProjectBase: asana.ProjectBase{Name: "My Outgoing Tasks"}},
	}

	p, err := findProject(projects, "outgoing")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "1" {
		t.Errorf("expected project ID 1, got %s", p.ID)
	}
}

func TestFindProject_NotFound(t *testing.T) {
	projects := []*asana.Project{
		{ID: "1", ProjectBase: asana.ProjectBase{Name: "Alpha"}},
	}

	_, err := findProject(projects, "Nonexistent")
	if err == nil {
		t.Fatal("expected error for not-found project")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}
