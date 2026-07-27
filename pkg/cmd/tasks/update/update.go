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
	"github.com/timwehrle/asana/pkg/cmdutils"
	"github.com/timwehrle/asana/pkg/convert"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/format"
	"github.com/timwehrle/asana/pkg/htmlnotes"
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
	TaskID          string
	Name            string
	Description     string
	HTMLNotes       string
	MarkdownNotes   string
	Due             string
	Assignee        string
	Followers       []string
	RemoveFollowers []string
	Complete        bool
	Incomplete      bool
	Unassigned      bool
	NoDue           bool
	NoDescription   bool
	NonInteractive  bool
	DryRun          bool

	// Whether each value flag appeared on the command line at all. Passing one
	// explicitly empty (-a "") means "clear this"; omitting it means "leave it
	// alone", and the value alone cannot tell the two apart.
	assigneeFlagSet    bool
	dueFlagSet         bool
	descriptionFlagSet bool
}

// wantsClear returns the fields this invocation asks to blank out, in a stable
// order. Each has two spellings: an explicit --no-* flag, and passing the value
// flag empty.
func (o *UpdateOptions) wantsClear() []asana.ClearableField {
	var clear []asana.ClearableField

	if o.Unassigned || (o.assigneeFlagSet && o.Assignee == "") {
		clear = append(clear, asana.ClearAssignee)
	}
	if o.NoDue || (o.dueFlagSet && o.Due == "") {
		clear = append(clear, asana.ClearDueDate)
	}
	if o.NoDescription || (o.descriptionFlagSet && o.Description == "") {
		clear = append(clear, asana.ClearNotes)
	}

	return clear
}

// isNonInteractive returns true when prompts should be suppressed: an explicit
// --non-interactive, a task ID (which means the caller is driving by flags), or
// stdin not being a terminal — with no tty there is nothing to prompt on, so
// prompting could only ever fail with EOF.
func (o *UpdateOptions) isNonInteractive() bool {
	if o.NonInteractive || o.TaskID != "" {
		return true
	}
	return o.IO != nil && !o.IO.IsStdinTTY
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
		Long: heredoc.Doc(`
			Update a task interactively or via flags with a task ID.

			A task ID is required whenever prompts are unavailable -- that is,
			when stdin is not a terminal or --non-interactive is passed.

			Fields can be emptied as well as set. --unassigned, --no-due and
			--no-description remove the assignee, the due date and the
			description; --incomplete reopens a completed task. Passing the
			corresponding value flag explicitly empty does the same thing, so
			--assignee "" and --unassigned are equivalent. Omitting a flag
			entirely still means "leave this alone".

			Followers are the exception to that shape: Asana changes them
			through their own endpoints rather than the task body, so use
			--followers to add and --remove-followers to unfollow.

			A new description can be given in one of three mutually exclusive
			forms: --description for plain text, --markdown-notes for Markdown,
			or --html-notes for Asana-flavored HTML. Either of the latter two
			replaces the description with rich text, giving you working links,
			lists and emphasis.

			Asana accepts only these HTML elements, and only <a> and <img> may
			carry attributes: body strong em u s code ol ul li a blockquote pre
			h1 h2 hr img. --html-notes is checked against those rules locally,
			before any request is made. <a data-asana-gid="123"/> becomes an
			@-mention.
		`),
		Args: cobra.MaximumNArgs(1),
		Example: heredoc.Doc(`
			# Interactive mode
			$ asana tasks update

			# Non-interactive: update by task ID
			$ asana tasks update 1234567890 --name "New name" --due 2026-04-01
			$ asana tasks update 1234567890 --complete
			$ asana tasks update 1234567890 --assignee "Chris Christoff" --followers "Tom McFarlin"

			# Empty a field rather than set it
			$ asana tasks update 1234567890 --unassigned
			$ asana tasks update 1234567890 --no-due --no-description
			$ asana tasks update 1234567890 --incomplete

			# Rehearse: print the request, change nothing
			$ asana tasks update 1234567890 --dry-run --markdown-notes @notes.md

			# Replace the description with rich text
			$ asana tasks update 1234567890 --markdown-notes "Now with a [link](https://example.com)"
			$ asana tasks update 1234567890 --markdown-notes @notes.md
			$ generate-notes | asana tasks update 1234567890 --markdown-notes -
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				opts.TaskID = args[0]
			}
			opts.assigneeFlagSet = cmd.Flags().Changed("assignee")
			opts.dueFlagSet = cmd.Flags().Changed("due")
			opts.descriptionFlagSet = cmd.Flags().Changed("description")
			if runF != nil {
				return runF(opts)
			}
			return runUpdate(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Name, "name", "n", "", "New task name")
	cmd.Flags().StringVarP(&opts.Description, "description", "m", "", "New task description (plain text)")
	cmd.Flags().StringVar(&opts.HTMLNotes, "html-notes", "", "New task description as Asana-flavored HTML; pass @file to read a file or - for stdin")
	cmd.Flags().StringVar(&opts.MarkdownNotes, "markdown-notes", "", "New task description as Markdown, converted to rich text; pass @file to read a file or - for stdin")
	cmd.Flags().StringVarP(&opts.Due, "due", "d", "", "New due date (YYYY-MM-DD, 'today', 'tomorrow')")
	cmd.Flags().StringVarP(&opts.Assignee, "assignee", "a", "", "New assignee name or 'me'")
	cmd.Flags().StringSliceVarP(&opts.Followers, "followers", "f", nil, "Comma-separated follower names or IDs to add")
	cmd.Flags().StringSliceVar(&opts.RemoveFollowers, "remove-followers", nil, "Comma-separated follower names or IDs to unfollow")
	cmd.Flags().BoolVar(&opts.Complete, "complete", false, "Mark task as completed")
	cmd.Flags().BoolVar(&opts.Incomplete, "incomplete", false, "Reopen the task, marking it not completed")
	cmd.Flags().BoolVar(&opts.Unassigned, "unassigned", false, `Remove the assignee, leaving the task unassigned (same as --assignee "")`)
	cmd.Flags().BoolVar(&opts.NoDue, "no-due", false, `Remove the due date (same as --due "")`)
	cmd.Flags().BoolVar(&opts.NoDescription, "no-description", false, `Empty the description (same as --description "")`)
	cmd.Flags().BoolVar(&opts.NonInteractive, "non-interactive", false, "Never prompt; error if required flags are missing")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Resolve everything and print the request without updating the task")

	// --cc is a natural alias for --followers (agents and humans reach for "CC" intuitively)
	cmd.Flags().StringSliceVar(&opts.Followers, "cc", nil, "Alias for --followers")
	cmd.Flags().Lookup("cc").Hidden = true

	// A task has one description; pick one representation of it.
	cmd.MarkFlagsMutuallyExclusive("description", "html-notes", "markdown-notes")

	// Setting a value and clearing it are contradictory asks.
	cmd.MarkFlagsMutuallyExclusive("assignee", "unassigned")
	cmd.MarkFlagsMutuallyExclusive("due", "no-due")
	cmd.MarkFlagsMutuallyExclusive("description", "no-description")
	cmd.MarkFlagsMutuallyExclusive("html-notes", "no-description")
	cmd.MarkFlagsMutuallyExclusive("markdown-notes", "no-description")
	cmd.MarkFlagsMutuallyExclusive("complete", "incomplete")

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

	if opts.TaskID == "" {
		return fmt.Errorf("a task ID is required when not running interactively: asana tasks update <task-id> [flags]")
	}

	// Resolve and validate rich-text notes first: it needs no network access,
	// and a rejected value should not cost a round trip to Asana.
	htmlNotes, err := htmlnotes.Rich(opts.HTMLNotes, opts.MarkdownNotes, opts.IO.In)
	if err != nil {
		return err
	}

	cfg, err := opts.Config()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	ws, err := cfg.RequireWorkspace()
	if err != nil {
		return err
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
	fieldChanges, err := applyFieldFlags(opts, htmlNotes, req)
	if err != nil {
		return err
	}
	changes := append([]string{}, fieldChanges...)

	if opts.Assignee != "" {
		userID, err := resolveUserID(opts.Assignee, cfg, ws.ID, client)
		if err != nil {
			return err
		}
		req.Assignee = userID
		changes = append(changes, "assignee")
	}

	var followerIDs []string
	if len(opts.Followers) > 0 {
		var err error
		followerIDs, _, err = resolveFollowerIDs(opts.Followers, cfg, ws.ID, client)
		if err != nil {
			return err
		}
		changes = append(changes, "followers")
	}

	var removeFollowerIDs []string
	if len(opts.RemoveFollowers) > 0 {
		var err error
		removeFollowerIDs, _, err = resolveFollowerIDs(opts.RemoveFollowers, cfg, ws.ID, client)
		if err != nil {
			return err
		}
		changes = append(changes, "followers removed")
	}

	if len(changes) == 0 {
		return fmt.Errorf("no updates specified; use flags like --name, --due, --complete, --assignee, --followers, --markdown-notes, or a clearing flag such as --unassigned, --no-due, --no-description, --incomplete")
	}

	if opts.DryRun {
		if len(followerIDs) > 0 {
			req.Followers = followerIDs // shown for inspection; sent separately for real
		}
		if err := cmdutils.PrintDryRun(opts.IO, fmt.Sprintf("PUT /tasks/%s", opts.TaskID), req); err != nil {
			return err
		}
		// Say what the payload means. "assignee": null only reads as "unassign"
		// if you already know the convention.
		opts.IO.Printf("  %s %s\n", cs.Gray("Would change:"), strings.Join(changes, ", "))
		return nil
	}

	// Update task fields (everything except followers)
	if len(fieldChanges) > 0 || opts.Assignee != "" {
		if err := task.Update(client, req); err != nil {
			return fmt.Errorf("failed to update task: %w", err)
		}
	}

	// Followers are their own endpoints, one per direction — they cannot ride
	// along in the update body.
	if len(followerIDs) > 0 {
		if err := task.AddFollowers(client, followerIDs); err != nil {
			return fmt.Errorf("failed to add followers: %w", err)
		}
	}
	if len(removeFollowerIDs) > 0 {
		if err := task.RemoveFollowers(client, removeFollowerIDs); err != nil {
			return fmt.Errorf("failed to remove followers: %w", err)
		}
	}

	opts.IO.Printf("%s Updated task %s (%s)\n", cs.SuccessIcon, cs.Bold(task.Name), strings.Join(changes, ", "))
	if htmlNotes != "" {
		kind := "html"
		if opts.MarkdownNotes != "" {
			kind = "markdown"
		}
		opts.IO.Printf("  %s rich text (%s, %d chars)\n", cs.Gray("Description:"), kind, len(htmlNotes))
	}
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

// applyFieldFlags copies the flag-driven task fields onto req and returns a
// human-readable list of what changed. It touches nothing that needs the API,
// which is what makes it testable.
func applyFieldFlags(opts *UpdateOptions, htmlNotes string, req *asana.UpdateTaskRequest) ([]string, error) {
	var changes []string

	if opts.Name != "" {
		req.TaskBase.Name = opts.Name
		changes = append(changes, "name")
	}

	switch {
	case htmlNotes != "":
		req.TaskBase.HTMLNotes = htmlNotes
		changes = append(changes, "description")
	case opts.Description != "":
		req.TaskBase.Notes = opts.Description
		changes = append(changes, "description")
	}

	if opts.Due != "" {
		dueDate, err := parseDueDate(opts.Due)
		if err != nil {
			return nil, err
		}
		req.TaskBase.DueOn = dueDate
		changes = append(changes, "due date")
	}

	switch {
	case opts.Complete:
		completed := true
		req.TaskBase.Completed = &completed
		changes = append(changes, "completed")
	case opts.Incomplete:
		// A *bool, so false marshals rather than being dropped by omitempty.
		completed := false
		req.TaskBase.Completed = &completed
		changes = append(changes, "reopened")
	}

	// Clears are changes too, and must be reported as such or a caller sees
	// "no updates specified" for a perfectly valid request.
	for _, field := range opts.wantsClear() {
		req.Clear = append(req.Clear, field)
		changes = append(changes, clearedLabels[field])
	}

	return changes, nil
}

// clearedLabels names each clear in the command's own vocabulary rather than
// the API's, so the output reads as what was asked for.
var clearedLabels = map[asana.ClearableField]string{
	asana.ClearAssignee: "assignee cleared",
	asana.ClearDueDate:  "due date cleared",
	asana.ClearNotes:    "description cleared",
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

	ws, err := cfg.RequireWorkspace()
	if err != nil {
		return nil, err
	}

	client, err := opts.Client()
	if err != nil {
		return nil, fmt.Errorf("failed to create Asana client: %w", err)
	}

	tasks, _, err := client.QueryTasks(&asana.TaskQuery{
		Assignee:       "me",
		Workspace:      ws.ID,
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

func resolveUserID(name string, cfg *config.Config, workspaceID string, client *asana.Client) (string, error) {
	ws := &asana.Workspace{ID: workspaceID}
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

func resolveFollowerIDs(followers []string, cfg *config.Config, workspaceID string, client *asana.Client) ([]string, []string, error) {
	if len(followers) == 0 {
		return nil, nil, nil
	}

	ws := &asana.Workspace{ID: workspaceID}
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
