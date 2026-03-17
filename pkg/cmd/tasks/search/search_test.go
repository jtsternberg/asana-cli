package search

import (
	"testing"

	"github.com/timwehrle/asana/pkg/factory"
)

func TestNewCmdSearch_ProjectFlag(t *testing.T) {
	f, _, _ := factory.NewTestFactory()

	var sawOpts *SearchOptions
	cmd := NewCmdSearch(f, func(opts *SearchOptions) error {
		sawOpts = opts
		return nil
	})

	cmd.SetArgs([]string{"--project", "1234567890"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if sawOpts == nil {
		t.Fatal("runF was never called")
	}

	if len(sawOpts.ProjectsAny) != 1 || sawOpts.ProjectsAny[0] != "1234567890" {
		t.Errorf("ProjectsAny = %v; want [1234567890]", sawOpts.ProjectsAny)
	}
}

func TestNewCmdSearch_MultipleProjects(t *testing.T) {
	f, _, _ := factory.NewTestFactory()

	var sawOpts *SearchOptions
	cmd := NewCmdSearch(f, func(opts *SearchOptions) error {
		sawOpts = opts
		return nil
	})

	cmd.SetArgs([]string{"--project", "111,222"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if sawOpts == nil {
		t.Fatal("runF was never called")
	}

	if len(sawOpts.ProjectsAny) != 2 {
		t.Errorf("ProjectsAny length = %d; want 2", len(sawOpts.ProjectsAny))
	}

	joined := sawOpts.join(sawOpts.ProjectsAny)
	if joined != "111,222" {
		t.Errorf("joined ProjectsAny = %q; want %q", joined, "111,222")
	}
}

func TestNewCmdSearch_ProjectWithOtherFlags(t *testing.T) {
	f, _, _ := factory.NewTestFactory()

	var sawOpts *SearchOptions
	cmd := NewCmdSearch(f, func(opts *SearchOptions) error {
		sawOpts = opts
		return nil
	})

	cmd.SetArgs([]string{
		"--project", "9999",
		"--query", "deploy",
		"--assignee", "me",
		"--limit", "5",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if sawOpts == nil {
		t.Fatal("runF was never called")
	}

	if len(sawOpts.ProjectsAny) != 1 || sawOpts.ProjectsAny[0] != "9999" {
		t.Errorf("ProjectsAny = %v; want [9999]", sawOpts.ProjectsAny)
	}
	if sawOpts.Query != "deploy" {
		t.Errorf("Query = %q; want %q", sawOpts.Query, "deploy")
	}
	if sawOpts.Limit != 5 {
		t.Errorf("Limit = %d; want 5", sawOpts.Limit)
	}
}
