package create

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/internal/config"
	"github.com/timwehrle/asana/internal/prompter"
	"github.com/timwehrle/asana/pkg/convert"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/format"
	"github.com/timwehrle/asana/pkg/iostreams"
)

type CreateOptions struct {
	IO       *iostreams.IOStreams
	Prompter prompter.Prompter
	Config   func() (*config.Config, error)
	Client   func() (*asana.Client, error)

	Name           string
	Assignee       string
	Due            string
	Description    string
	Project        string
	Section        string
	Followers      []string
	NonInteractive bool
}

// isNonInteractive returns true when prompts should be suppressed.
// Explicit --non-interactive flag takes priority, but we also infer it
// when the required flags (name, assignee, project) are all provided.
func (o *CreateOptions) isNonInteractive() bool {
	if o.NonInteractive {
		return true
	}
	return o.Name != "" && o.Assignee != "" && o.Project != ""
}

func NewCmdCreate(f factory.Factory, runF func(*CreateOptions) error) *cobra.Command {
	opts := &CreateOptions{
		IO:       f.IOStreams,
		Prompter: f.Prompter,
		Config:   f.Config,
		Client:   f.Client,
	}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new task",
		Long:  "Create a new task in Asana.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF != nil {
				return runF(opts)
			}
			return runCreate(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Name, "name", "n", "", "Task name")
	cmd.Flags().StringVarP(&opts.Assignee, "assignee", "a", "", "Assignee name or 'me'")
	cmd.Flags().StringVarP(&opts.Due, "due", "d", "", "Due date (YYYY-MM-DD, 'today', 'tomorrow')")
	cmd.Flags().StringVarP(&opts.Description, "description", "m", "", "Task description")
	cmd.Flags().StringVarP(&opts.Project, "project", "p", "", "Project name or ID")
	cmd.Flags().StringVarP(&opts.Section, "section", "s", "", "Section name or ID")
	cmd.Flags().StringSliceVarP(&opts.Followers, "followers", "f", nil, "Comma-separated follower names or IDs")
	cmd.Flags().BoolVar(&opts.NonInteractive, "non-interactive", false, "Never prompt; error if required flags are missing")

	// --cc is a natural alias for --followers (agents and humans reach for "CC" intuitively)
	cmd.Flags().StringSliceVar(&opts.Followers, "cc", nil, "Alias for --followers")
	cmd.Flags().Lookup("cc").Hidden = true

	return cmd
}

func runCreate(opts *CreateOptions) error {
	cs := opts.IO.ColorScheme()
	ni := opts.isNonInteractive()

	cfg, err := opts.Config()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	client, err := opts.Client()
	if err != nil {
		return fmt.Errorf("failed to initialize Asana client: %w", err)
	}

	// --- Name ---
	name := opts.Name
	if name == "" {
		if ni {
			return fmt.Errorf("--name is required in non-interactive mode")
		}
		name, err = opts.Prompter.Input("Enter task name: ", "")
		if err != nil {
			return fmt.Errorf("failed to read task name: %w", err)
		}
	}
	if name == "" {
		return fmt.Errorf("task name cannot be empty")
	}

	// --- Assignee ---
	assignee, err := getOrSelectAssignee(opts, ni, cfg, client)
	if err != nil {
		return err
	}

	// --- Due date (optional) ---
	dueDate, err := getOrPromptDueDate(opts)
	if err != nil {
		return err
	}

	// --- Description (optional) ---
	description := opts.Description
	if description == "" && !ni {
		shouldPromptForDescription, err := opts.Prompter.Confirm("Add description?", "No")
		if err == nil && shouldPromptForDescription {
			description, err = addDescription(opts)
		}
		if err != nil {
			return err
		}
	}

	// --- Project ---
	project, err := getProject(opts, ni, cfg.Workspace.ID, client)
	if err != nil {
		return err
	}

	// --- Section (defaults to first section when not specified in non-interactive mode) ---
	section, err := getSection(opts, ni, project.ID, client)
	if err != nil {
		return err
	}

	// --- Followers (optional) ---
	followerIDs, followerNames, err := resolveFollowers(opts, cfg, client)
	if err != nil {
		return err
	}

	req := &asana.CreateTaskRequest{
		TaskBase: asana.TaskBase{
			Name:  name,
			DueOn: dueDate,
			Notes: description,
		},
		Workspace: cfg.Workspace.ID,
		Assignee:  assignee.ID,
		Followers: followerIDs,
		Projects:  []string{project.ID},
		Memberships: []*asana.CreateMembership{
			{
				Project: project.ID,
				Section: section.ID,
			},
		},
	}
	if err := req.Validate(); err != nil {
		return fmt.Errorf("task validation failed: %w", err)
	}

	task, err := client.CreateTask(req)
	if err != nil {
		return fmt.Errorf("error creating task: %w", err)
	}

	opts.IO.Printf("%s Created task %s\n", cs.SuccessIcon, cs.Bold(task.Name))
	opts.IO.Printf("  %s %s\n", cs.Gray("Assignee:"), assignee.Name)
	if len(followerNames) > 0 {
		opts.IO.Printf("  %s %s\n", cs.Gray("Followers:"), strings.Join(followerNames, ", "))
	}
	if task.DueOn != nil {
		dueStr := format.Date(task.DueOn)
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

func getOrSelectAssignee(opts *CreateOptions, ni bool, cfg *config.Config, client *asana.Client) (*asana.User, error) {
	ws := &asana.Workspace{ID: cfg.Workspace.ID}
	users, _, err := ws.Users(client)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch users: %w", err)
	}

	if opts.Assignee != "" {
		if strings.ToLower(opts.Assignee) == "me" {
			if cfg.UserID == "" {
				currentUser, err := client.CurrentUser()
				if err != nil {
					return nil, fmt.Errorf("failed to fetch current user: %w", err)
				}
				for _, user := range users {
					if user.ID == currentUser.ID {
						return user, nil
					}
				}
				return nil, fmt.Errorf("could not find current user in workspace")
			} else {
				for _, user := range users {
					if user.ID == cfg.UserID {
						return user, nil
					}
				}
				return nil, fmt.Errorf("could not find current user in workspace")
			}
		}

		// Try exact name match
		assigneeLower := strings.ToLower(opts.Assignee)
		for _, user := range users {
			if strings.ToLower(user.Name) == assigneeLower {
				return user, nil
			}
		}

		// Try partial/contains match
		for _, user := range users {
			if strings.Contains(strings.ToLower(user.Name), assigneeLower) {
				return user, nil
			}
		}

		// Try ID match
		for _, user := range users {
			if user.ID == opts.Assignee {
				return user, nil
			}
		}

		return nil, fmt.Errorf("assignee %q not found in workspace", opts.Assignee)
	}

	if ni {
		return nil, fmt.Errorf("--assignee is required in non-interactive mode")
	}

	names := format.MapToStrings(users, func(u *asana.User) string {
		return u.Name
	})

	selected, err := opts.Prompter.Select("Select assignee: ", names)
	if err != nil {
		return nil, fmt.Errorf("assignee selection failed: %w", err)
	}
	return users[selected], nil
}

func getOrPromptDueDate(opts *CreateOptions) (*asana.Date, error) {
	if opts.Due != "" {
		return parseDueDate(opts.Due)
	}
	if opts.NonInteractive {
		return nil, nil
	}
	return promptDueDate(opts)
}

// dueDateKeyword returns the keyword if the input was a relative date keyword, empty otherwise.
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

func promptDueDate(opts *CreateOptions) (*asana.Date, error) {
	input, err := opts.Prompter.Input("Enter due date (YYYY-MM-DD), leave blank for none: ", "")
	if err != nil {
		return nil, fmt.Errorf("failed to read due date: %w", err)
	}
	if input == "" {
		return nil, nil
	}
	return parseDueDate(input)
}

func addDescription(opts *CreateOptions) (string, error) {
	description, err := opts.Prompter.Editor("Enter task description: ", "")
	if err != nil {
		return "", fmt.Errorf("failed to read task description: %w", err)
	}
	return strings.TrimSpace(description), nil
}

func getProject(opts *CreateOptions, ni bool, workspaceID string, client *asana.Client) (*asana.Project, error) {
	ws := &asana.Workspace{ID: workspaceID}
	projects, err := ws.AllProjects(client)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch projects: %w", err)
	}

	if opts.Project != "" {
		projectLower := strings.ToLower(opts.Project)
		// Exact match first
		for _, p := range projects {
			if strings.ToLower(p.Name) == projectLower || p.ID == opts.Project {
				return p, nil
			}
		}
		// Partial/contains match
		for _, p := range projects {
			if strings.Contains(strings.ToLower(p.Name), projectLower) {
				return p, nil
			}
		}
		return nil, fmt.Errorf("project %q not found in workspace", opts.Project)
	}

	if ni {
		return nil, fmt.Errorf("--project is required in non-interactive mode")
	}

	names := format.MapToStrings(projects, func(p *asana.Project) string {
		return p.Name
	})

	selected, err := opts.Prompter.Select("Select project: ", names)
	if err != nil {
		return nil, fmt.Errorf("project selection failed: %w", err)
	}
	return projects[selected], nil
}

func getSection(opts *CreateOptions, ni bool, projectID string, client *asana.Client) (*asana.Section, error) {
	project := &asana.Project{ID: projectID}
	sections, _, err := project.Sections(client)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch sections: %w", err)
	}

	if opts.Section != "" {
		sectionLower := strings.ToLower(opts.Section)
		// Exact match
		for _, s := range sections {
			if strings.ToLower(s.Name) == sectionLower || s.ID == opts.Section {
				return s, nil
			}
		}
		// Partial/contains match
		for _, s := range sections {
			if strings.Contains(strings.ToLower(s.Name), sectionLower) {
				return s, nil
			}
		}
		return nil, fmt.Errorf("section %q not found in project", opts.Section)
	}

	// In non-interactive mode, default to the first section
	if ni {
		if len(sections) == 0 {
			return nil, fmt.Errorf("project has no sections")
		}
		return sections[0], nil
	}

	names := format.MapToStrings(sections, func(p *asana.Section) string {
		return p.Name
	})

	selected, err := opts.Prompter.Select("Select section: ", names)
	if err != nil {
		return nil, fmt.Errorf("section selection failed: %w", err)
	}
	return sections[selected], nil
}

// resolveFollowers resolves follower names/IDs to user IDs.
// Returns (followerIDs, followerNames, error).
func resolveFollowers(opts *CreateOptions, cfg *config.Config, client *asana.Client) ([]string, []string, error) {
	if len(opts.Followers) == 0 {
		return nil, nil, nil
	}

	ws := &asana.Workspace{ID: cfg.Workspace.ID}
	users, _, err := ws.Users(client)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot fetch users for follower resolution: %w", err)
	}

	var ids []string
	var names []string

	for _, f := range opts.Followers {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}

		found := false
		fLower := strings.ToLower(f)

		// Exact name match
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

		// Partial/contains match
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

		// ID match
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
