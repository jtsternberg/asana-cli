package update

import (
	"fmt"
	"strings"
	"time"

	"github.com/timwehrle/asana/internal/config"
	"github.com/timwehrle/asana/internal/prompter"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/pkg/convert"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/format"
	"github.com/timwehrle/asana/pkg/iostreams"
)

type UpdateAction int

const (
	ActionComplete UpdateAction = iota
	ActionEditName
	ActionEditDescription
	ActionSetDueDate
	ActionCancel
)

type taskAction struct {
	name   string
	action UpdateAction
}

var availableActions = []taskAction{
	{name: "Mark as Completed", action: ActionComplete},
	{name: "Edit Task Name", action: ActionEditName},
	{name: "Edit Description", action: ActionEditDescription},
	{name: "Set Due Date", action: ActionSetDueDate},
	{name: "Cancel", action: ActionCancel},
}

type UpdateOptions struct {
	IO       *iostreams.IOStreams
	Prompter prompter.Prompter

	Config func() (*config.Config, error)
	Client func() (*asana.Client, error)

	// Non-interactive flags
	TaskID         string
	Name           string
	Description    string
	Due            string
	Assignee       string
	Followers      []string
	Complete       bool
	NonInteractive bool
}

func (o *UpdateOptions) isNonInteractive() bool {
	return o.NonInteractive || o.TaskID != ""
}

func NewCmdUpdate(f factory.Factory, runF func(*UpdateOptions) error) *cobra.Command {
	opts := &UpdateOptions{
		IO:       f.IOStreams,
		Prompter: f.Prompter,
		Config:   f.Config,
		Client:   f.Client,
	}

	cmd := &cobra.Command{
		Use:   "update [task-id]",
		Short: "Update details of a specific task",
		Long:  "Update a task interactively or via flags with a task ID.",
		Args:  cobra.MaximumNArgs(1),
		Example: heredoc.Doc(`
			# Interactive mode
			$ asana tasks update

			# Non-interactive: update by task ID
			$ asana tasks update 1234567890 --name "New name" --due 2026-04-01
			$ asana tasks update 1234567890 --complete
			$ asana tasks update 1234567890 --assignee "Chris Christoff" --followers "Tom McFarlin"
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				opts.TaskID = args[0]
			}
			if runF != nil {
				return runF(opts)
			}
			return runUpdate(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Name, "name", "n", "", "New task name")
	cmd.Flags().StringVarP(&opts.Description, "description", "m", "", "New task description")
	cmd.Flags().StringVarP(&opts.Due, "due", "d", "", "New due date (YYYY-MM-DD, 'today', 'tomorrow')")
	cmd.Flags().StringVarP(&opts.Assignee, "assignee", "a", "", "New assignee name or 'me'")
	cmd.Flags().StringSliceVarP(&opts.Followers, "followers", "f", nil, "Comma-separated follower names or IDs to add")
	cmd.Flags().BoolVar(&opts.Complete, "complete", false, "Mark task as completed")
	cmd.Flags().BoolVar(&opts.NonInteractive, "non-interactive", false, "Never prompt; error if required flags are missing")

	// --cc is a natural alias for --followers (agents and humans reach for "CC" intuitively)
	cmd.Flags().StringSliceVar(&opts.Followers, "cc", nil, "Alias for --followers")
	cmd.Flags().Lookup("cc").Hidden = true

	return cmd
}

func runUpdate(opts *UpdateOptions) error {
	if opts.isNonInteractive() {
		return runNonInteractiveUpdate(opts)
	}
	return runInteractiveUpdate(opts)
}

func runNonInteractiveUpdate(opts *UpdateOptions) error {
	cs := opts.IO.ColorScheme()

	cfg, err := opts.Config()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	client, err := opts.Client()
	if err != nil {
		return fmt.Errorf("failed to create Asana client: %w", err)
	}

	task := &asana.Task{ID: opts.TaskID}
	if err := task.Fetch(client); err != nil {
		return fmt.Errorf("task %q not found: %w", opts.TaskID, err)
	}

	req := &asana.UpdateTaskRequest{}
	changes := []string{}

	if opts.Name != "" {
		req.TaskBase.Name = opts.Name
		changes = append(changes, "name")
	}

	if opts.Description != "" {
		req.TaskBase.Notes = opts.Description
		changes = append(changes, "description")
	}

	if opts.Due != "" {
		dueDate, err := parseDueDate(opts.Due)
		if err != nil {
			return err
		}
		req.TaskBase.DueOn = dueDate
		changes = append(changes, "due date")
	}

	if opts.Complete {
		completed := true
		req.TaskBase.Completed = &completed
		changes = append(changes, "completed")
	}

	if opts.Assignee != "" {
		userID, err := resolveUserID(opts.Assignee, cfg, client)
		if err != nil {
			return err
		}
		req.Assignee = userID
		changes = append(changes, "assignee")
	}

	var followerIDs []string
	if len(opts.Followers) > 0 {
		var err error
		followerIDs, _, err = resolveFollowerIDs(opts.Followers, cfg, client)
		if err != nil {
			return err
		}
		changes = append(changes, "followers")
	}

	if len(changes) == 0 {
		return fmt.Errorf("no updates specified; use flags like --name, --due, --complete, --assignee, --followers")
	}

	// Update task fields (everything except followers)
	hasFieldUpdates := opts.Name != "" || opts.Description != "" || opts.Due != "" || opts.Complete || opts.Assignee != ""
	if hasFieldUpdates {
		if err := task.Update(client, req); err != nil {
			return fmt.Errorf("failed to update task: %w", err)
		}
	}

	// Add followers via separate endpoint
	if len(followerIDs) > 0 {
		if err := task.AddFollowers(client, followerIDs); err != nil {
			return fmt.Errorf("failed to add followers: %w", err)
		}
	}

	opts.IO.Printf("%s Updated task %s (%s)\n", cs.SuccessIcon, cs.Bold(task.Name), strings.Join(changes, ", "))
	if opts.Due != "" && req.TaskBase.DueOn != nil {
		dueStr := format.Date(req.TaskBase.DueOn)
		if keyword := dueDateKeyword(opts.Due); keyword != "" {
			dueStr = fmt.Sprintf("%s (%s)", dueStr, keyword)
		}
		opts.IO.Printf("  %s %s\n", cs.Gray("Due:"), dueStr)
	}
	if task.PermalinkURL != "" {
		opts.IO.Printf("  %s %s\n", cs.Gray("URL:"), task.PermalinkURL)
	}

	return nil
}

func runInteractiveUpdate(opts *UpdateOptions) error {
	task, err := selectTask(opts)
	if err != nil {
		return err
	}

	action, err := selectAction(opts)
	if err != nil {
		return err
	}

	if err := performAction(opts, task, action); err != nil {
		return fmt.Errorf("failed to perform action: %w", err)
	}

	return nil
}

func selectTask(opts *UpdateOptions) (*asana.Task, error) {
	cfg, err := opts.Config()
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	client, err := opts.Client()
	if err != nil {
		return nil, fmt.Errorf("failed to create Asana client: %w", err)
	}

	tasks, _, err := client.QueryTasks(&asana.TaskQuery{
		Assignee:       "me",
		Workspace:      cfg.Workspace.ID,
		CompletedSince: "now",
	}, &asana.Options{
		Fields: []string{"name", "due_on"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks: %w", err)
	}

	if len(tasks) == 0 {
		fmt.Fprintln(opts.IO.Out, "No tasks found.")
		return nil, nil
	}

	taskNames := format.Tasks(tasks)
	index, err := opts.Prompter.Select("Select the task to update:", taskNames)
	if err != nil {
		return nil, fmt.Errorf("failed to select task: %w", err)
	}

	selectedTask := tasks[index]
	if err := selectedTask.Fetch(client); err != nil {
		return nil, fmt.Errorf("failed to fetch task details: %w", err)
	}

	return selectedTask, nil
}

func selectAction(opts *UpdateOptions) (UpdateAction, error) {
	actions := make([]string, len(availableActions))
	for i, action := range availableActions {
		actions[i] = action.name
	}

	index, err := opts.Prompter.Select("What do you want to do with this task:", actions)
	if err != nil {
		return 0, fmt.Errorf("failed to select action: %w", err)
	}

	return availableActions[index].action, nil
}

func performAction(opts *UpdateOptions, task *asana.Task, action UpdateAction) error {
	client, err := opts.Client()
	if err != nil {
		return fmt.Errorf("failed to create Asana client: %w", err)
	}

	cs := opts.IO.ColorScheme()

	switch action {
	case ActionComplete:
		return completeTask(client, task, cs)
	case ActionEditName:
		return editTaskName(opts, client, task, cs)
	case ActionEditDescription:
		return editTaskDescription(opts, client, task, cs)
	case ActionSetDueDate:
		return setTaskDueDate(opts, client, task, cs)
	case ActionCancel:
		fmt.Fprintf(
			opts.IO.Out,
			"%s Operation canceled. You can rerun the command to try again.\n",
			cs.SuccessIcon,
		)
		return nil
	default:
		return fmt.Errorf("unknown action: %d", action)
	}
}

func completeTask(client *asana.Client, task *asana.Task, cs *iostreams.ColorScheme) error {
	completed := true
	updateRequest := &asana.UpdateTaskRequest{
		TaskBase: asana.TaskBase{
			Completed: &completed,
		},
	}

	if err := task.Update(client, updateRequest); err != nil {
		return fmt.Errorf("failed to complete task: %w", err)
	}

	fmt.Printf("%s Task completed\n", cs.SuccessIcon)

	return nil
}

func editTaskName(
	opts *UpdateOptions,
	client *asana.Client,
	task *asana.Task,
	cs *iostreams.ColorScheme,
) error {
	newName, err := opts.Prompter.Input("Enter the new task name:", task.Name)
	if err != nil {
		return fmt.Errorf("failed to get input: %w", err)
	}

	newName = strings.TrimSpace(newName)
	if newName == task.Name {
		fmt.Fprintf(opts.IO.Out, "%s No changes made to task name\n", cs.WarningIcon)
	}

	updateRequest := &asana.UpdateTaskRequest{
		TaskBase: asana.TaskBase{
			Name: newName,
		},
	}

	if err := task.Update(client, updateRequest); err != nil {
		return fmt.Errorf("failed to update task name: %w", err)
	}

	fmt.Fprintf(opts.IO.Out, "%s Task name updated\n", cs.SuccessIcon)
	return nil
}

func editTaskDescription(
	opts *UpdateOptions,
	client *asana.Client,
	task *asana.Task,
	cs *iostreams.ColorScheme,
) error {
	existingDescription := strings.TrimSpace(task.Notes)
	newDescription, err := opts.Prompter.Editor("Edit the description:", existingDescription)
	if err != nil {
		return fmt.Errorf("failed to get input: %w", err)
	}

	newDescription = strings.TrimSpace(newDescription)
	if newDescription == existingDescription {
		fmt.Fprintf(opts.IO.Out, "%s No changes made to description\n", cs.WarningIcon)
		return nil
	}

	updateRequest := &asana.UpdateTaskRequest{
		TaskBase: asana.TaskBase{
			Notes: newDescription,
		},
	}

	if err = task.Update(client, updateRequest); err != nil {
		return fmt.Errorf("failed to update task description: %w", err)
	}

	fmt.Fprintf(opts.IO.Out, "%s Description updated\n", cs.SuccessIcon)
	return nil
}

func setTaskDueDate(
	opts *UpdateOptions,
	client *asana.Client,
	task *asana.Task,
	cs *iostreams.ColorScheme,
) error {
	input, err := opts.Prompter.Input(
		"Enter the new due date (YYYY-MM-DD):",
		format.Date(task.DueOn),
	)
	if err != nil {
		return fmt.Errorf("failed to get input: %w", err)
	}

	dueDate, err := convert.ToDate(input, time.DateOnly)
	if err != nil {
		return fmt.Errorf("invalid date format: %w", err)
	}

	updateRequest := &asana.UpdateTaskRequest{
		TaskBase: asana.TaskBase{
			DueOn: dueDate,
		},
	}

	if err := task.Update(client, updateRequest); err != nil {
		return fmt.Errorf("failed to update task due date: %w", err)
	}

	fmt.Fprintf(opts.IO.Out, "%s Due date updated\n", cs.SuccessIcon)
	return nil
}

func dueDateKeyword(input string) string {
	switch strings.ToLower(input) {
	case "today", "tomorrow":
		return strings.ToLower(input)
	}
	return ""
}

func parseDueDate(input string) (*asana.Date, error) {
	now := time.Now()
	switch strings.ToLower(input) {
	case "today":
		return convert.ToDate(now.Format(time.DateOnly), time.DateOnly)
	case "tomorrow":
		return convert.ToDate(now.AddDate(0, 0, 1).Format(time.DateOnly), time.DateOnly)
	}
	due, err := convert.ToDate(input, time.DateOnly)
	if err != nil {
		return nil, fmt.Errorf("invalid due date %q: %w", input, err)
	}
	return due, nil
}

func resolveUserID(name string, cfg *config.Config, client *asana.Client) (string, error) {
	ws := &asana.Workspace{ID: cfg.Workspace.ID}
	users, _, err := ws.Users(client)
	if err != nil {
		return "", fmt.Errorf("cannot fetch users: %w", err)
	}

	if strings.ToLower(name) == "me" {
		if cfg.UserID != "" {
			return cfg.UserID, nil
		}
		currentUser, err := client.CurrentUser()
		if err != nil {
			return "", fmt.Errorf("failed to fetch current user: %w", err)
		}
		return currentUser.ID, nil
	}

	nameLower := strings.ToLower(name)
	for _, u := range users {
		if strings.ToLower(u.Name) == nameLower {
			return u.ID, nil
		}
	}
	for _, u := range users {
		if strings.Contains(strings.ToLower(u.Name), nameLower) {
			return u.ID, nil
		}
	}
	for _, u := range users {
		if u.ID == name {
			return u.ID, nil
		}
	}

	return "", fmt.Errorf("user %q not found in workspace", name)
}

func resolveFollowerIDs(followers []string, cfg *config.Config, client *asana.Client) ([]string, []string, error) {
	if len(followers) == 0 {
		return nil, nil, nil
	}

	ws := &asana.Workspace{ID: cfg.Workspace.ID}
	users, _, err := ws.Users(client)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot fetch users: %w", err)
	}

	var ids, names []string
	for _, f := range followers {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		fLower := strings.ToLower(f)
		found := false

		for _, u := range users {
			if strings.ToLower(u.Name) == fLower {
				ids = append(ids, u.ID)
				names = append(names, u.Name)
				found = true
				break
			}
		}
		if found {
			continue
		}
		for _, u := range users {
			if strings.Contains(strings.ToLower(u.Name), fLower) {
				ids = append(ids, u.ID)
				names = append(names, u.Name)
				found = true
				break
			}
		}
		if found {
			continue
		}
		for _, u := range users {
			if u.ID == f {
				ids = append(ids, u.ID)
				names = append(names, u.Name)
				found = true
				break
			}
		}
		if !found {
			return nil, nil, fmt.Errorf("follower %q not found in workspace", f)
		}
	}

	return ids, names, nil
}
