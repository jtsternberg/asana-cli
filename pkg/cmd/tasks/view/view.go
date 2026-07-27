package view

import (
	"fmt"
	"strings"
	"time"

	"github.com/timwehrle/asana/internal/config"
	"github.com/timwehrle/asana/internal/prompter"

	"github.com/MakeNowJust/heredoc"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/format"
	"github.com/timwehrle/asana/pkg/iostreams"
	"github.com/timwehrle/asana/pkg/taskjson"

	"github.com/spf13/cobra"
	"github.com/timwehrle/asana/internal/api/asana"
)

type ViewOptions struct {
	IO       *iostreams.IOStreams
	Prompter prompter.Prompter

	Config func() (*config.Config, error)
	Client func() (*asana.Client, error)

	TaskID string
	JSON   bool
}

func NewCmdView(f factory.Factory, runF func(*ViewOptions) error) *cobra.Command {
	opts := &ViewOptions{
		IO:       f.IOStreams,
		Prompter: f.Prompter,
		Config:   f.Config,
		Client:   f.Client,
	}

	cmd := &cobra.Command{
		Use:   "view [task-id]",
		Short: "View details of a specific task",
		Example: heredoc.Doc(`
				# Interactive: select from your tasks
				$ asana tasks view

				# Non-interactive: view by task ID
				$ asana tasks view 1234567890
				$ asana ts view 1234567890`),
		Long: heredoc.Doc(`
				Display detailed information about a specific task.
				Pass a task ID to view it directly, or omit for interactive selection.`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				opts.TaskID = args[0]
			}
			if runF != nil {
				return runF(opts)
			}

			return viewRun(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output in JSON format")

	return cmd
}

func viewRun(opts *ViewOptions) error {
	client, err := opts.Client()
	if err != nil {
		return err
	}

	// Non-interactive: view by task ID
	if opts.TaskID != "" {
		task := &asana.Task{ID: opts.TaskID}
		if err := task.Fetch(client, taskjson.Options()); err != nil {
			return fmt.Errorf("task %q not found: %w", opts.TaskID, err)
		}
		return displayDetails(task, opts.IO, opts.JSON)
	}

	// Interactive: select from your tasks
	cfg, err := opts.Config()
	if err != nil {
		return err
	}

	ws, err := cfg.RequireWorkspace()
	if err != nil {
		return err
	}

	allTasks, _, err := client.QueryTasks(&asana.TaskQuery{
		Assignee:       "me",
		Workspace:      ws.ID,
		CompletedSince: "now",
	}, &asana.Options{
		Fields: []string{"due_on", "name"},
	})
	if err != nil {
		return err
	}

	selectedTask, err := prompt(allTasks, opts.Prompter)
	if err != nil {
		return err
	}

	if err := selectedTask.Fetch(client, taskjson.Options()); err != nil {
		return fmt.Errorf("failed to load task details: %w", err)
	}
	return displayDetails(selectedTask, opts.IO, opts.JSON)
}

func prompt(allTasks []*asana.Task, prompter prompter.Prompter) (*asana.Task, error) {
	taskNames := format.Tasks(allTasks)

	today := time.Now()
	selectMessage := fmt.Sprintf(
		"Your Tasks on %s (Select one for more details):",
		today.Format("Jan 02, 2006"),
	)

	index, err := prompter.Select(selectMessage, taskNames)
	if err != nil {
		return nil, err
	}

	return allTasks[index], nil
}

func displayDetails(task *asana.Task, io *iostreams.IOStreams, jsonOutput bool) error {
	if jsonOutput {
		return displayJSON(task, io)
	}
	return displayText(task, io)
}

func displayJSON(task *asana.Task, io *iostreams.IOStreams) error {
	return taskjson.Write(io.Out, task)
}

func displayText(task *asana.Task, io *iostreams.IOStreams) error {
	cs := io.ColorScheme()

	assigneeName := "Unassigned"
	if task.Assignee != nil {
		assigneeName = task.Assignee.Name
	}

	// Header line
	fmt.Fprintf(io.Out, "%s\n", cs.Bold(task.Name))

	// Status line
	status := "Incomplete"
	if task.Completed != nil && *task.Completed {
		status = "Completed"
		if task.CompletedAt != nil {
			status += " (" + task.CompletedAt.Format("Jan 02, 2006") + ")"
		}
	}
	fmt.Fprintf(io.Out, "Status: %s | Assignee: %s\n", status, assigneeName)

	// Dates line
	dateParts := []string{fmt.Sprintf("Due: %s", format.Date(task.DueOn))}
	if task.DueAt != nil {
		dateParts = append(dateParts, fmt.Sprintf("Due At: %s", task.DueAt.Format("Jan 02, 2006 3:04 PM")))
	}
	if task.StartOn != nil {
		dateParts = append(dateParts, fmt.Sprintf("Start: %s", time.Time(*task.StartOn).Format("Jan 02, 2006")))
	}
	fmt.Fprintf(io.Out, "%s\n", strings.Join(dateParts, " | "))

	// Projects
	fmt.Fprintf(io.Out, "%s\n", format.Projects(task.Projects))

	// Tags
	if len(task.Tags) > 0 {
		fmt.Fprintf(io.Out, "%s\n", format.Tags(task.Tags))
	}

	// Parent
	if task.Parent != nil {
		name := task.Parent.Name
		if name == "" {
			name = task.Parent.ID
		}
		fmt.Fprintf(io.Out, "Parent: %s\n", name)
	}

	// Subtasks
	if task.NumSubtasks > 0 {
		fmt.Fprintf(io.Out, "Subtasks: %d\n", task.NumSubtasks)
	}

	// Custom fields
	if len(task.CustomFields) > 0 {
		fmt.Fprintln(io.Out, "\nCustom Fields:")
		for _, cf := range task.CustomFields {
			val := "(empty)"
			if cf.DisplayValue != nil {
				val = *cf.DisplayValue
			}
			fmt.Fprintf(io.Out, "  %s: %s\n", cf.Name, val)
		}
	}

	// Dependencies
	if len(task.Dependencies) > 0 {
		fmt.Fprintln(io.Out, "\nDependencies:")
		for _, d := range task.Dependencies {
			name := d.Name
			if name == "" {
				name = d.ID
			}
			fmt.Fprintf(io.Out, "  - %s\n", name)
		}
	}

	// Dependents
	if len(task.Dependents) > 0 {
		fmt.Fprintln(io.Out, "\nDependents:")
		for _, d := range task.Dependents {
			name := d.Name
			if name == "" {
				name = d.ID
			}
			fmt.Fprintf(io.Out, "  - %s\n", name)
		}
	}

	// Notes
	fmt.Fprintln(io.Out, format.Notes(task.Notes))

	// URL
	if task.PermalinkURL != "" {
		fmt.Fprintf(io.Out, "\n%s %s\n", cs.Gray("URL:"), task.PermalinkURL)
	}

	return nil
}
