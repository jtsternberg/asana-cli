package tasks

import (
	"testing"

	"github.com/timwehrle/asana/pkg/factory"
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
