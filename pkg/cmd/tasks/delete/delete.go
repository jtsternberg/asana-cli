package delete

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/iostreams"
)

type DeleteOptions struct {
	IO     *iostreams.IOStreams
	Client func() (*asana.Client, error)

	TaskID string
}

func NewCmdDelete(f factory.Factory, runF func(*DeleteOptions) error) *cobra.Command {
	opts := &DeleteOptions{
		IO:     f.IOStreams,
		Client: f.Client,
	}

	cmd := &cobra.Command{
		Use:   "delete <task-id>",
		Short: "Delete a task",
		Long:  "Permanently delete a task by its ID.",
		Args:  cobra.ExactArgs(1),
		Example: heredoc.Doc(`
			$ asana tasks delete 1234567890
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.TaskID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return runDelete(opts)
		},
	}

	return cmd
}

func runDelete(opts *DeleteOptions) error {
	cs := opts.IO.ColorScheme()

	client, err := opts.Client()
	if err != nil {
		return fmt.Errorf("failed to initialize Asana client: %w", err)
	}

	task := &asana.Task{ID: opts.TaskID}
	if err := task.Fetch(client); err != nil {
		return fmt.Errorf("task %q not found: %w", opts.TaskID, err)
	}

	if err := task.Delete(client); err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	opts.IO.Printf("%s Deleted task %s\n", cs.SuccessIcon, cs.Bold(task.Name))
	return nil
}
