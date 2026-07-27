package update

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/h2non/gock"
	"github.com/jtsternberg/asana-cli/internal/api/asana"
	"github.com/jtsternberg/asana-cli/internal/config"
	"github.com/jtsternberg/asana-cli/pkg/factory"
	"github.com/jtsternberg/asana-cli/pkg/iostreams"
)

func TestNewCmdUpdate_RichNotesFlags(t *testing.T) {
	f, _, _ := factory.NewTestFactory()

	var sawOpts *UpdateOptions
	cmd := NewCmdUpdate(f, func(opts *UpdateOptions) error {
		sawOpts = opts
		return nil
	})
	cmd.SetArgs([]string{"123", "--markdown-notes", "@notes.md"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if sawOpts.TaskID != "123" {
		t.Errorf("TaskID = %q", sawOpts.TaskID)
	}
	if sawOpts.MarkdownNotes != "@notes.md" {
		t.Errorf("MarkdownNotes = %q", sawOpts.MarkdownNotes)
	}
}

func TestNewCmdUpdate_NotesFlagsAreMutuallyExclusive(t *testing.T) {
	pairs := [][]string{
		{"--description", "plain", "--html-notes", "<body>x</body>"},
		{"--description", "plain", "--markdown-notes", "x"},
		{"--html-notes", "<body>x</body>", "--markdown-notes", "x"},
	}

	for _, pair := range pairs {
		f, _, _ := factory.NewTestFactory()
		cmd := NewCmdUpdate(f, func(opts *UpdateOptions) error { return nil })
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(append([]string{"123"}, pair...))

		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "none of the others can be") {
			t.Fatalf("%v: expected a mutual-exclusion error, got %v", pair, err)
		}
	}
}

func TestRunNonInteractiveUpdate_InvalidHTMLNotesFailsBeforeAnyRequest(t *testing.T) {
	io, _, _, _ := iostreams.Test()

	opts := &UpdateOptions{
		IO:        io,
		TaskID:    "123",
		HTMLNotes: "<body><p>nope</p></body>",
		Config: func() (*config.Config, error) {
			t.Fatal("config should not be loaded when the notes are invalid")
			return nil, nil
		},
		Client: func() (*asana.Client, error) {
			t.Fatal("client should not be built when the notes are invalid")
			return nil, nil
		},
	}

	err := runUpdate(opts)
	if err == nil || !strings.Contains(err.Error(), "<p> is not allowed") {
		t.Fatalf("expected an html-notes validation error, got %v", err)
	}
}

func TestWantsClear(t *testing.T) {
	tests := []struct {
		name string
		opts *UpdateOptions
		want []asana.ClearableField
	}{
		{
			name: "nothing cleared",
			opts: &UpdateOptions{Name: "n"},
			want: nil,
		},
		{
			name: "--unassigned",
			opts: &UpdateOptions{Unassigned: true},
			want: []asana.ClearableField{asana.ClearAssignee},
		},
		{
			// -a "" is the shorthand an agent reaches for first.
			name: "empty assignee passed explicitly",
			opts: &UpdateOptions{assigneeFlagSet: true},
			want: []asana.ClearableField{asana.ClearAssignee},
		},
		{
			// Absent is not empty: omitting -a must stay a no-op, or every
			// update without it would silently unassign the task.
			name: "assignee flag never passed",
			opts: &UpdateOptions{},
			want: nil,
		},
		{
			name: "a real assignee is not a clear",
			opts: &UpdateOptions{Assignee: "me", assigneeFlagSet: true},
			want: nil,
		},
		{
			name: "--no-due",
			opts: &UpdateOptions{NoDue: true},
			want: []asana.ClearableField{asana.ClearDueDate},
		},
		{
			name: "empty due passed explicitly",
			opts: &UpdateOptions{dueFlagSet: true},
			want: []asana.ClearableField{asana.ClearDueDate},
		},
		{
			name: "--no-description",
			opts: &UpdateOptions{NoDescription: true},
			want: []asana.ClearableField{asana.ClearNotes},
		},
		{
			name: "empty description passed explicitly",
			opts: &UpdateOptions{descriptionFlagSet: true},
			want: []asana.ClearableField{asana.ClearNotes},
		},
		{
			name: "several at once, in a stable order",
			opts: &UpdateOptions{Unassigned: true, NoDue: true, NoDescription: true},
			want: []asana.ClearableField{asana.ClearAssignee, asana.ClearDueDate, asana.ClearNotes},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.opts.wantsClear()
			if len(got) != len(tt.want) {
				t.Fatalf("wantsClear() = %v; want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("wantsClear() = %v; want %v", got, tt.want)
				}
			}
		})
	}
}

// End to end through runUpdate: the payload must carry an explicit null, and a
// clear on its own must count as a change rather than "no updates specified".
func TestRunNonInteractiveUpdate_ClearsSendExplicitNull(t *testing.T) {
	tests := []struct {
		name       string
		opts       *UpdateOptions
		wantKey    string
		wantJSON   string
		wantChange string
	}{
		{
			name:       "--unassigned",
			opts:       &UpdateOptions{Unassigned: true},
			wantKey:    "assignee",
			wantJSON:   `"assignee":null`,
			wantChange: "assignee cleared",
		},
		{
			name:       "--no-due",
			opts:       &UpdateOptions{NoDue: true},
			wantKey:    "due_on",
			wantJSON:   `"due_on":null`,
			wantChange: "due date cleared",
		},
		{
			name:       "--no-description",
			opts:       &UpdateOptions{NoDescription: true},
			wantKey:    "notes",
			wantJSON:   `"notes":""`,
			wantChange: "description cleared",
		},
		{
			name:       "--incomplete reopens the task",
			opts:       &UpdateOptions{Incomplete: true},
			wantKey:    "completed",
			wantJSON:   `"completed":false`,
			wantChange: "reopened",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer gock.Off()
			// --dry-run still fetches the task, on purpose: a rehearsal that
			// does not notice a bad ID is not much of a rehearsal.
			gock.New("https://app.asana.com").
				Get("/api/1.0/tasks/123").
				Reply(200).
				JSON(map[string]any{"data": map[string]any{"gid": "123", "name": "A task"}})

			io, _, out, _ := iostreams.Test()
			opts := tt.opts
			opts.IO = io
			opts.TaskID = "123"
			opts.DryRun = true
			opts.Config = func() (*config.Config, error) {
				return &config.Config{Workspace: &asana.Workspace{ID: "W1", Name: "WS"}}, nil
			}
			opts.Client = func() (*asana.Client, error) { return asana.NewClient(http.DefaultClient), nil }

			if err := runUpdate(opts); err != nil {
				t.Fatalf("runUpdate() error = %v", err)
			}

			got := out.String()
			if !strings.Contains(got, tt.wantChange) {
				t.Errorf("output should report %q; got:\n%s", tt.wantChange, got)
			}
			if !strings.Contains(strings.ReplaceAll(got, " ", ""), tt.wantJSON) {
				t.Errorf("payload should contain %s; got:\n%s", tt.wantJSON, got)
			}

			// The payload is bracketed by prose on both sides now, so slice it
			// out rather than reading to end of output.
			start, end := strings.Index(got, "{"), strings.LastIndex(got, "}")
			if start < 0 || end < start {
				t.Fatalf("no JSON payload in output:\n%s", got)
			}
			var payload map[string]any
			if err := json.Unmarshal([]byte(got[start:end+1]), &payload); err != nil {
				t.Fatalf("payload is not valid JSON: %v", err)
			}
			if _, present := payload[tt.wantKey]; !present {
				t.Errorf("payload should carry key %q; got %v", tt.wantKey, payload)
			}
		})
	}
}

func TestNewCmdUpdate_ClearFlagConflicts(t *testing.T) {
	conflicts := [][]string{
		{"--assignee", "me", "--unassigned"},
		{"--due", "today", "--no-due"},
		{"--description", "x", "--no-description"},
		{"--markdown-notes", "x", "--no-description"},
		{"--html-notes", "<body>x</body>", "--no-description"},
		{"--complete", "--incomplete"},
	}

	for _, args := range conflicts {
		f, _, _ := factory.NewTestFactory()
		cmd := NewCmdUpdate(f, func(opts *UpdateOptions) error { return nil })
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(append([]string{"123"}, args...))

		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "none of the others can be") {
			t.Fatalf("%v: expected a mutual-exclusion error, got %v", args, err)
		}
	}
}

func TestNewCmdUpdate_RecordsExplicitlyEmptyFlags(t *testing.T) {
	f, _, _ := factory.NewTestFactory()

	var sawOpts *UpdateOptions
	cmd := NewCmdUpdate(f, func(opts *UpdateOptions) error {
		sawOpts = opts
		return nil
	})
	cmd.SetArgs([]string{"123", "-a", "", "-d", "", "-m", ""})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !sawOpts.assigneeFlagSet || !sawOpts.dueFlagSet || !sawOpts.descriptionFlagSet {
		t.Fatalf("flag-set tracking = %v/%v/%v; want all true",
			sawOpts.assigneeFlagSet, sawOpts.dueFlagSet, sawOpts.descriptionFlagSet)
	}

	want := []asana.ClearableField{asana.ClearAssignee, asana.ClearDueDate, asana.ClearNotes}
	got := sawOpts.wantsClear()
	if len(got) != len(want) {
		t.Fatalf("wantsClear() = %v; want %v", got, want)
	}
}

func TestApplyFieldFlags(t *testing.T) {
	tests := []struct {
		name        string
		opts        *UpdateOptions
		htmlNotes   string
		wantChanges []string
		wantErr     string
	}{
		{
			name:        "nothing set",
			opts:        &UpdateOptions{},
			wantChanges: nil,
		},
		{
			name:        "plain description",
			opts:        &UpdateOptions{Description: "text"},
			wantChanges: []string{"description"},
		},
		{
			// Rich notes on their own are a real change; the command must not
			// report "no updates specified".
			name:        "rich description",
			htmlNotes:   "<body><strong>hi</strong></body>",
			opts:        &UpdateOptions{MarkdownNotes: "**hi**"},
			wantChanges: []string{"description"},
		},
		{
			name:        "several fields",
			opts:        &UpdateOptions{Name: "n", Due: "today", Complete: true},
			wantChanges: []string{"name", "due date", "completed"},
		},
		{
			name:    "invalid due date",
			opts:    &UpdateOptions{Due: "not-a-date"},
			wantErr: "invalid due date",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &asana.UpdateTaskRequest{}
			changes, err := applyFieldFlags(tt.opts, tt.htmlNotes, req)

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("applyFieldFlags() error = %v; want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("applyFieldFlags() error = %v", err)
			}
			if strings.Join(changes, ",") != strings.Join(tt.wantChanges, ",") {
				t.Fatalf("changes = %v; want %v", changes, tt.wantChanges)
			}
			if tt.htmlNotes != "" && req.TaskBase.HTMLNotes != tt.htmlNotes {
				t.Fatalf("HTMLNotes = %q; want %q", req.TaskBase.HTMLNotes, tt.htmlNotes)
			}
		})
	}
}

func ttyIO() *iostreams.IOStreams {
	io, _, _, _ := iostreams.Test()
	io.IsStdinTTY = true
	return io
}

func TestIsNonInteractive(t *testing.T) {
	tests := []struct {
		name string
		opts *UpdateOptions
		want bool
	}{
		{
			name: "explicit flag",
			opts: &UpdateOptions{IO: ttyIO(), NonInteractive: true},
			want: true,
		},
		{
			name: "task id implies flag-driven update",
			opts: &UpdateOptions{IO: ttyIO(), TaskID: "123"},
			want: true,
		},
		{
			name: "tty with no task id stays interactive",
			opts: &UpdateOptions{IO: ttyIO()},
			want: false,
		},
		{
			name: "no tty means nothing to prompt on",
			opts: func() *UpdateOptions {
				io, _, _, _ := iostreams.Test() // IsStdinTTY == false
				return &UpdateOptions{IO: io}
			}(),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.opts.isNonInteractive(); got != tt.want {
				t.Fatalf("isNonInteractive() = %v; want %v", got, tt.want)
			}
		})
	}
}

// Without a tty and without a task ID there is no way to pick a task, so say
// so instead of issuing a GET /tasks/ with an empty ID.
func TestRunNonInteractiveUpdate_RequiresTaskID(t *testing.T) {
	io, _, _, _ := iostreams.Test()

	opts := &UpdateOptions{
		IO:   io,
		Name: "New name",
		Config: func() (*config.Config, error) {
			return &config.Config{Workspace: &asana.Workspace{ID: "W1", Name: "WS"}}, nil
		},
		Client: func() (*asana.Client, error) {
			t.Fatal("client should not be built without a task ID")
			return nil, nil
		},
	}

	err := runUpdate(opts)
	if err == nil || !strings.Contains(err.Error(), "task ID is required") {
		t.Fatalf("expected task-ID error, got %v", err)
	}
}
