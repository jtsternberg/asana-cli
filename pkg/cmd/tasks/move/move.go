package move

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/internal/config"
	"github.com/timwehrle/asana/internal/prompter"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/format"
	"github.com/timwehrle/asana/pkg/iostreams"
)

type MoveOptions struct {
	IO       *iostreams.IOStreams
	Prompter prompter.Prompter
	Config   func() (*config.Config, error)
	Client   func() (*asana.Client, error)

	TaskID  string
	Project string
	Section string
	Keep    bool
}

func (o *MoveOptions) isNonInteractive() bool {
	return o.TaskID != "" && o.Project != ""
}

func NewCmdMove(f factory.Factory, runF func(*MoveOptions) error) *cobra.Command {
	opts := &MoveOptions{
		IO:       f.IOStreams,
		Prompter: f.Prompter,
		Config:   f.Config,
		Client:   f.Client,
	}

	cmd := &cobra.Command{
		Use:   "move [task-id]",
		Short: "Move a task to a different project or section",
		Long:  "Move a task to a different project and/or section. Preserves task history, comments, and attachments.",
		Args:  cobra.MaximumNArgs(1),
		Example: heredoc.Doc(`
			# Move a task to a different project
			$ asana tasks move 1234567890 -p "Outgoing Tasks"

			# Move to a specific section within a project
			$ asana tasks move 1234567890 -p "Outgoing Tasks" -s "Tom"

			# Add to a project without removing from current project(s)
			$ asana tasks move 1234567890 -p "Outgoing Tasks" -s "Tom" --keep

			# Interactive mode
			$ asana tasks move
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				opts.TaskID = args[0]
			}
			if runF != nil {
				return runF(opts)
			}
			return runMove(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Project, "project", "p", "", "Target project name or ID")
	cmd.Flags().StringVarP(&opts.Section, "section", "s", "", "Target section name or ID")
	cmd.Flags().BoolVar(&opts.Keep, "keep", false, "Keep task in current project(s) too (add instead of move)")

	return cmd
}

func runMove(opts *MoveOptions) error {
	cs := opts.IO.ColorScheme()

	cfg, err := opts.Config()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	client, err := opts.Client()
	if err != nil {
		return fmt.Errorf("failed to initialize Asana client: %w", err)
	}

	// --- Task selection ---
	task, err := getOrSelectTask(opts, cfg, client)
	if err != nil {
		return err
	}

	// Fetch full task details including memberships
	if err := task.Fetch(client, &asana.Options{
		Fields: []string{"name", "memberships.project.name", "memberships.section.name", "projects.name", "permalink_url"},
	}); err != nil {
		return fmt.Errorf("failed to fetch task details: %w", err)
	}

	ni := opts.isNonInteractive()

	// --- Target project ---
	targetProject, err := getProject(opts, ni, cfg.Workspace.ID, client)
	if err != nil {
		return err
	}

	// --- Target section (optional) ---
	targetSection, err := getSection(opts, ni, targetProject.ID, client)
	if err != nil {
		return err
	}

	// --- Add to new project ---
	addReq := &asana.AddProjectRequest{
		Project: targetProject.ID,
	}
	if targetSection != nil {
		addReq.Section = targetSection.ID
	}

	if err := task.AddProject(client, addReq); err != nil {
		return fmt.Errorf("failed to add task to project %q: %w", targetProject.Name, err)
	}

	// --- Remove from old project(s) unless --keep ---
	removed := []string{}
	if !opts.Keep {
		for _, m := range task.Memberships {
			if m.Project != nil && m.Project.ID != targetProject.ID {
				if err := task.RemoveProject(client, m.Project.ID); err != nil {
					opts.IO.Printf("%s Failed to remove from project %q: %v\n", cs.WarningIcon, m.Project.Name, err)
				} else {
					removed = append(removed, m.Project.Name)
				}
			}
		}
	}

	// --- Output ---
	action := "Moved"
	if opts.Keep {
		action = "Added"
	}

	dest := targetProject.Name
	if targetSection != nil {
		dest += " → " + targetSection.Name
	}

	opts.IO.Printf("%s %s task %s to %s\n", cs.SuccessIcon, action, cs.Bold(task.Name), dest)
	if len(removed) > 0 {
		opts.IO.Printf("  %s Removed from: %s\n", cs.Gray("↳"), strings.Join(removed, ", "))
	}
	if task.PermalinkURL != "" {
		opts.IO.Printf("  %s %s\n", cs.Gray("URL:"), task.PermalinkURL)
	}

	return nil
}

func getOrSelectTask(opts *MoveOptions, cfg *config.Config, client *asana.Client) (*asana.Task, error) {
	if opts.TaskID != "" {
		return &asana.Task{ID: opts.TaskID}, nil
	}

	// Interactive: select from user's tasks
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
		return nil, fmt.Errorf("no tasks found")
	}

	taskNames := format.Tasks(tasks)
	index, err := opts.Prompter.Select("Select the task to move:", taskNames)
	if err != nil {
		return nil, fmt.Errorf("failed to select task: %w", err)
	}

	return tasks[index], nil
}

func getProject(opts *MoveOptions, ni bool, workspaceID string, client *asana.Client) (*asana.Project, error) {
	ws := &asana.Workspace{ID: workspaceID}
	projects, err := ws.AllProjects(client)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch projects: %w", err)
	}

	if opts.Project != "" {
		projectLower := strings.ToLower(opts.Project)
		for _, p := range projects {
			if strings.ToLower(p.Name) == projectLower || p.ID == opts.Project {
				return p, nil
			}
		}
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

	selected, err := opts.Prompter.Select("Select target project:", names)
	if err != nil {
		return nil, fmt.Errorf("project selection failed: %w", err)
	}
	return projects[selected], nil
}

func getSection(opts *MoveOptions, ni bool, projectID string, client *asana.Client) (*asana.Section, error) {
	project := &asana.Project{ID: projectID}
	sections, _, err := project.Sections(client)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch sections: %w", err)
	}

	if len(sections) == 0 {
		return nil, nil
	}

	if opts.Section != "" {
		sectionLower := strings.ToLower(opts.Section)
		for _, s := range sections {
			if strings.ToLower(s.Name) == sectionLower || s.ID == opts.Section {
				return s, nil
			}
		}
		for _, s := range sections {
			if strings.Contains(strings.ToLower(s.Name), sectionLower) {
				return s, nil
			}
		}
		return nil, fmt.Errorf("section %q not found in project", opts.Section)
	}

	// In non-interactive mode with no section specified, skip (task goes to default location)
	if ni {
		return nil, nil
	}

	names := format.MapToStrings(sections, func(s *asana.Section) string {
		return s.Name
	})

	selected, err := opts.Prompter.Select("Select target section:", names)
	if err != nil {
		return nil, fmt.Errorf("section selection failed: %w", err)
	}
	return sections[selected], nil
}
