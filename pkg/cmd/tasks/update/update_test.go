package update

import (
	"io"
	"strings"
	"testing"

	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/internal/config"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/iostreams"
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
