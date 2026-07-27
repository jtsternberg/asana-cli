package cmdutils

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jtsternberg/asana-cli/internal/api/asana"
	"github.com/jtsternberg/asana-cli/pkg/iostreams"
)

func TestPrintDryRun(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	req := &asana.CreateTaskRequest{
		TaskBase: asana.TaskBase{
			Name:      "Ship the thing",
			HTMLNotes: "<body><strong>hi</strong></body>",
		},
		Workspace: "W1",
	}

	if err := PrintDryRun(io, "POST /tasks", req); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	if !strings.Contains(got, "no request was made") {
		t.Errorf("output should say nothing was sent; got:\n%s", got)
	}
	if !strings.Contains(got, "POST /tasks") {
		t.Errorf("output should name the endpoint; got:\n%s", got)
	}

	// The markup must be readable, not <-escaped — inspecting it is the
	// whole reason to run a dry run.
	if !strings.Contains(got, "<strong>hi</strong>") {
		t.Errorf("html notes should appear unescaped; got:\n%s", got)
	}

	// The payload must be the real thing, parseable and complete, so a caller
	// can inspect generated html_notes without creating a task.
	start := strings.Index(got, "{")
	if start < 0 {
		t.Fatalf("no JSON payload in output:\n%s", got)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(got[start:]), &payload); err != nil {
		t.Fatalf("payload is not valid JSON: %v\n%s", err, got[start:])
	}
	if payload["html_notes"] != "<body><strong>hi</strong></body>" {
		t.Errorf("html_notes = %v", payload["html_notes"])
	}
	if payload["name"] != "Ship the thing" {
		t.Errorf("name = %v", payload["name"])
	}
}
