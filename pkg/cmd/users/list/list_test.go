package list

import (
	"testing"

	"github.com/timwehrle/asana/pkg/factory"
)

func TestNewCmdList_JSONFlag(t *testing.T) {
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

func TestNewCmdList_WithIDFlag(t *testing.T) {
	f, _, _ := factory.NewTestFactory()

	var sawOpts *ListOptions
	cmd := NewCmdList(f, func(opts *ListOptions) error {
		sawOpts = opts
		return nil
	})

	cmd.SetArgs([]string{"--with-id", "--limit", "10"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if sawOpts == nil {
		t.Fatal("runF was never called")
	}

	if !sawOpts.WithID {
		t.Error("WithID = false; want true")
	}
	if sawOpts.Limit != 10 {
		t.Errorf("Limit = %d; want 10", sawOpts.Limit)
	}
}
