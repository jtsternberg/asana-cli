package list

import (
	"errors"
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
