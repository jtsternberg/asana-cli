package create

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/internal/config"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/iostreams"
)

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

// makeUsers returns a workspace user slice including a name with "me" as a substring,
// to exercise the regression where -f "me" would substring-match into "Angie Meeker".
func makeUsers() []*asana.User {
	me := &asana.User{ID: "U_ME"}
	me.Name = "Justin Sternberg"
	angie := &asana.User{ID: "U_ANGIE"}
	angie.Name = "Angie Meeker"
	tom := &asana.User{ID: "U_TOM"}
	tom.Name = "Tom Mendez"
	return []*asana.User{angie, tom, me}
}

func TestResolveMeUser_FromCachedUserID(t *testing.T) {
	cfg := &config.Config{UserID: "U_ME"}
	users := makeUsers()

	got, err := resolveMeUser(cfg, nil, users)
	if err != nil {
		t.Fatalf("resolveMeUser error: %v", err)
	}
	if got.ID != "U_ME" {
		t.Errorf("ID = %q; want U_ME (got user %q) — 'me' should never substring-match into other names", got.ID, got.Name)
	}
}

func TestResolveMeUser_NotInWorkspace(t *testing.T) {
	cfg := &config.Config{UserID: "U_OTHER"}
	users := makeUsers()

	_, err := resolveMeUser(cfg, nil, users)
	if err == nil || !strings.Contains(err.Error(), "could not find current user") {
		t.Errorf("expected not-found error, got: %v", err)
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

			got, err := getOrPromptDueDate(opts)
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

	_, err := getOrPromptDueDate(opts)
	if err == nil || !strings.Contains(err.Error(), "invalid due date") {
		t.Fatalf("expected invalid-date error, got %v", err)
	}
}
