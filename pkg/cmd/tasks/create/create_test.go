package create

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/internal/config"
	"github.com/timwehrle/asana/internal/prompter"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/iostreams"
)

// explodingPrompter fails every prompt. Any test that uses it asserts the
// prompt is never reached.
type explodingPrompter struct{ t *testing.T }

func (p *explodingPrompter) Input(prompt, defaultValue string) (string, error) {
	p.t.Fatalf("unexpected Input prompt: %q", prompt)
	return "", nil
}

func (p *explodingPrompter) Confirm(prompt, defaultValue string) (bool, error) {
	p.t.Fatalf("unexpected Confirm prompt: %q", prompt)
	return false, nil
}

func (p *explodingPrompter) Token() (string, error) {
	p.t.Fatal("unexpected Token prompt")
	return "", nil
}

func (p *explodingPrompter) Select(message string, options []string) (int, error) {
	p.t.Fatalf("unexpected Select prompt: %q", message)
	return 0, nil
}

func (p *explodingPrompter) Editor(prompt, existingDescription string) (string, error) {
	p.t.Fatalf("unexpected Editor prompt: %q", prompt)
	return "", nil
}

// eofPrompter mimics a prompt running with no usable stdin: survey surfaces
// io.EOF, wrapped by the prompter package.
type eofPrompter struct{ prompter.Prompter }

func (p *eofPrompter) Input(prompt, defaultValue string) (string, error) {
	return "", fmt.Errorf("could not prompt: %w", io.EOF)
}

func (p *eofPrompter) Confirm(prompt, defaultValue string) (bool, error) {
	return false, fmt.Errorf("could not prompt: %w", io.EOF)
}

func ttyIO() *iostreams.IOStreams {
	io, _, _, _ := iostreams.Test()
	io.IsStdinTTY = true
	return io
}

func TestNewCmdCreate_RunE(t *testing.T) {
	f, _, _ := factory.NewTestFactory()

	var sawOpts *CreateOptions
	cmd := NewCmdCreate(f, func(opts *CreateOptions) error {
		sawOpts = opts
		return nil
	})

	cmd.SetArgs([]string{
		"--name", "My Task",
		"--assignee", "me",
		"--due", "2025-01-01",
		"--description", "Test description",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if sawOpts == nil {
		t.Fatal("runF was never called")
	}

	if sawOpts.Name != "My Task" {
		t.Errorf("Name = %q; want %q", sawOpts.Name, "My Task")
	}
	if sawOpts.Assignee != "me" {
		t.Errorf("Assignee = %q; want %q", sawOpts.Assignee, "me")
	}
	if sawOpts.Due != "2025-01-01" {
		t.Errorf("Due = %q; want %q", sawOpts.Due, "2025-01-01")
	}
	if sawOpts.Description != "Test description" {
		t.Errorf("Description = %q; want %q", sawOpts.Description, "Test description")
	}
}

func TestRunCreate_ConfigError(t *testing.T) {
	io, _, _, _ := iostreams.Test()

	opts := &CreateOptions{
		IO: io,
		Config: func() (*config.Config, error) {
			return nil, errors.New("no config")
		},
		Client: func() (*asana.Client, error) {
			return nil, nil
		},
	}

	err := runCreate(opts)
	if err == nil || !strings.Contains(err.Error(), "failed to load config") {
		t.Fatalf("expected config error, got %v", err)
	}
}

func TestNewCmdCreate_CCAlias(t *testing.T) {
	f, _, _ := factory.NewTestFactory()

	var sawOpts *CreateOptions
	cmd := NewCmdCreate(f, func(opts *CreateOptions) error {
		sawOpts = opts
		return nil
	})

	cmd.SetArgs([]string{
		"--name", "My Task",
		"--assignee", "me",
		"--cc", "Chris Christoff,Tom McFarlin",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if sawOpts == nil {
		t.Fatal("runF was never called")
	}

	if len(sawOpts.Followers) != 2 {
		t.Fatalf("Followers = %v; want 2 entries", sawOpts.Followers)
	}
	if sawOpts.Followers[0] != "Chris Christoff" {
		t.Errorf("Followers[0] = %q; want %q", sawOpts.Followers[0], "Chris Christoff")
	}
}

func TestDueDateKeyword(t *testing.T) {
	if got := dueDateKeyword("today"); got != "today" {
		t.Errorf("dueDateKeyword(\"today\") = %q; want \"today\"", got)
	}
	if got := dueDateKeyword("Tomorrow"); got != "tomorrow" {
		t.Errorf("dueDateKeyword(\"Tomorrow\") = %q; want \"tomorrow\"", got)
	}
	if got := dueDateKeyword("2026-04-01"); got != "" {
		t.Errorf("dueDateKeyword(\"2026-04-01\") = %q; want empty", got)
	}
}

func TestGetOrPromptDueDate(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		input   string
		wantDay string
	}{
		{
			name:    "today",
			input:   "today",
			wantDay: now.Format(time.DateOnly),
		},
		{
			name:    "tomorrow",
			input:   "tomorrow",
			wantDay: now.AddDate(0, 0, 1).Format(time.DateOnly),
		},
		{
			name:    "explicit date",
			input:   "2025-01-10",
			wantDay: "2025-01-10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &CreateOptions{Due: tt.input}

			got, err := getOrPromptDueDate(opts, false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got == nil {
				t.Fatal("got nil date")
			}

			gotDay := time.Time(*got).Format(time.DateOnly)
			if gotDay != tt.wantDay {
				t.Fatalf("date = %v; want %v", gotDay, tt.wantDay)
			}
		})
	}
}

func TestGetOrPromptDueDate_Invalid(t *testing.T) {
	opts := &CreateOptions{Due: "not-a-date"}

	_, err := getOrPromptDueDate(opts, false)
	if err == nil || !strings.Contains(err.Error(), "invalid due date") {
		t.Fatalf("expected invalid-date error, got %v", err)
	}
}

func TestIsNonInteractive(t *testing.T) {
	tests := []struct {
		name string
		opts *CreateOptions
		want bool
	}{
		{
			name: "explicit flag wins",
			opts: &CreateOptions{IO: ttyIO(), NonInteractive: true},
			want: true,
		},
		{
			name: "all required flags on a tty",
			opts: &CreateOptions{IO: ttyIO(), Name: "n", Assignee: "a", Project: "p"},
			want: true,
		},
		{
			name: "partial flags on a tty stays interactive",
			opts: &CreateOptions{IO: ttyIO(), Name: "n"},
			want: false,
		},
		{
			name: "no tty means nothing to prompt on",
			opts: func() *CreateOptions {
				io, _, _, _ := iostreams.Test() // IsStdinTTY == false
				return &CreateOptions{IO: io, Name: "n"}
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

// The optional due-date prompt used to consult opts.NonInteractive directly,
// so an inferred non-interactive run still hit the prompt (and died on EOF).
func TestGetOrPromptDueDate_InferredNonInteractiveSkipsPrompt(t *testing.T) {
	opts := &CreateOptions{
		IO:       ttyIO(),
		Prompter: &explodingPrompter{t: t},
		Name:     "Task",
		Assignee: "me",
		Project:  "Outgoing Tasks",
	}

	got, err := getOrPromptDueDate(opts, opts.isNonInteractive())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("due date = %v; want nil", got)
	}
}

// "leave blank for none" must mean none when stdin hands us EOF, not a fatal error.
func TestGetOrPromptDueDate_EOFMeansNoDueDate(t *testing.T) {
	opts := &CreateOptions{
		IO:       ttyIO(),
		Prompter: &eofPrompter{},
	}

	got, err := getOrPromptDueDate(opts, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("due date = %v; want nil", got)
	}
}

// Same for the optional "Add description?" confirmation.
func TestPromptDescription_EOFMeansNoDescription(t *testing.T) {
	opts := &CreateOptions{
		IO:       ttyIO(),
		Prompter: &eofPrompter{},
	}

	got, err := getOrPromptDescription(opts, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("description = %q; want empty", got)
	}
}
