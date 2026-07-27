package update

import (
	"strings"
	"testing"

	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/internal/config"
	"github.com/timwehrle/asana/pkg/iostreams"
)

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
