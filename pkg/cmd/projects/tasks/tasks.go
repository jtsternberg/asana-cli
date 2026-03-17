package tasks

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/MakeNowJust/heredoc"
	"golang.org/x/sync/errgroup"

	"github.com/timwehrle/asana/internal/config"
	"github.com/timwehrle/asana/internal/prompter"

	"github.com/spf13/cobra"
	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/pkg/cmd/projects/shared"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/iostreams"
)

const defaultPageSize = 100

type TasksOptions struct {
	IO       *iostreams.IOStreams
	Prompter prompter.Prompter

	Config func() (*config.Config, error)
	Client func() (*asana.Client, error)

	ProjectName  string
	WithSections bool
	Limit        int
	JSON         bool
}

type sectionTasks struct {
	section *asana.Section
	tasks   []*asana.Task
}

func NewCmdTasks(f factory.Factory, runF func(*TasksOptions) error) *cobra.Command {
	opts := &TasksOptions{
		IO:       f.IOStreams,
		Prompter: f.Prompter,
		Config:   f.Config,
		Client:   f.Client,
	}
	cmd := &cobra.Command{
		Use:   "tasks [project-name]",
		Short: "List tasks of a project",
		Long: heredoc.Doc(`
			Retrieve and display a list of all tasks under a project.

			If a project name or ID is provided, it will be used directly.
			Otherwise, you will be prompted to select a project interactively.
		`),
		Args: cobra.MaximumNArgs(1),
		Example: heredoc.Doc(`
			# List all tasks of a project by name
			$ asana projects tasks "Outgoing Tasks"

			# List tasks interactively (prompts for project)
			$ asana projects tasks

			# List tasks grouped by section
			$ asana projects tasks "My Project" --sections

			# Limit total tasks returned
			$ asana projects tasks "My Project" --limit 50
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.ProjectName = args[0]
			}

			if opts.Limit < 0 {
				return fmt.Errorf("invalid limit: %v", opts.Limit)
			}

			if runF != nil {
				return runF(opts)
			}

			return runTasks(opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.WithSections, "sections", "s", false, "Group tasks by sections")
	cmd.Flags().IntVarP(&opts.Limit, "limit", "l", 0, "Limit the total number of tasks returned (0 = no limit)")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output in JSON format")
	return cmd
}

func runTasks(opts *TasksOptions) error {
	cfg, err := opts.Config()
	if err != nil {
		return err
	}

	client, err := opts.Client()
	if err != nil {
		return err
	}

	project, err := selectProject(opts, client, cfg.Workspace.ID)
	if err != nil {
		return err
	}

	if opts.WithSections {
		return listTasksWithSections(opts, client, project)
	}

	return listAllTasks(opts, client, project)
}

func selectProject(
	opts *TasksOptions,
	client *asana.Client,
	workspaceID string,
) (*asana.Project, error) {
	projects, err := shared.FetchAllProjects(client, &asana.Workspace{ID: workspaceID}, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch projects: %w", err)
	}

	if len(projects) == 0 {
		fmt.Fprintln(opts.IO.Out, "No projects found")
		return nil, errors.New("no projects found")
	}

	// If a project name/ID was provided, find it directly
	if opts.ProjectName != "" {
		return findProject(projects, opts.ProjectName)
	}

	// Otherwise, prompt interactively
	projectNames := make([]string, len(projects))
	for i, project := range projects {
		projectNames[i] = project.Name
	}

	index, err := opts.Prompter.Select("Select a project:", projectNames)
	if err != nil {
		return nil, fmt.Errorf("failed to select a project: %w", err)
	}

	return projects[index], nil
}

func findProject(projects []*asana.Project, name string) (*asana.Project, error) {
	nameLower := strings.ToLower(name)

	// Exact match on name or ID
	for _, p := range projects {
		if strings.ToLower(p.Name) == nameLower || p.ID == name {
			return p, nil
		}
	}

	// Fuzzy match (contains)
	for _, p := range projects {
		if strings.Contains(strings.ToLower(p.Name), nameLower) {
			return p, nil
		}
	}

	return nil, fmt.Errorf("project %q not found in workspace", name)
}

func listAllTasks(opts *TasksOptions, client *asana.Client, project *asana.Project) error {
	tasks := make([]*asana.Task, 0, 50)
	options := &asana.Options{Limit: defaultPageSize}

	for {
		batch, nextPage, err := project.Tasks(client, options)
		if err != nil {
			return fmt.Errorf("failed to fetch tasks for project %q: %w", project.Name, err)
		}

		tasks = append(tasks, batch...)

		if opts.Limit > 0 && len(tasks) >= opts.Limit {
			tasks = tasks[:opts.Limit]
			break
		}

		if nextPage == nil || nextPage.Offset == "" {
			break
		}

		options.Offset = nextPage.Offset
	}

	return displayTasks(opts, project, tasks)
}

const sectionConcurrency = 5

func listTasksWithSections(opts *TasksOptions, client *asana.Client, project *asana.Project) error {
	sections := make([]*asana.Section, 0, 20)
	options := &asana.Options{Limit: defaultPageSize}

	for {
		batch, nextPage, err := project.Sections(client, options)
		if err != nil {
			return err
		}

		sections = append(sections, batch...)

		if nextPage == nil || nextPage.Offset == "" {
			break
		}

		options.Offset = nextPage.Offset
	}

	results := make([][]*asana.Task, len(sections))
	var totalFetched atomic.Int64

	g := new(errgroup.Group)
	g.SetLimit(sectionConcurrency)

	for i, section := range sections {
		g.Go(func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic fetching tasks for section %q: %v", section.Name, r)
				}
			}()

			// Skip if earlier goroutines already collected enough tasks
			if opts.Limit > 0 && totalFetched.Load() >= int64(opts.Limit) {
				return nil
			}

			tasks := make([]*asana.Task, 0, 50)
			sectionOpts := &asana.Options{Limit: defaultPageSize}

			for {
				var batch []*asana.Task
				var nextPage *asana.NextPage
				var fetchErr error

				for attempt := range 3 {
					batch, nextPage, fetchErr = section.Tasks(client, sectionOpts)
					if fetchErr == nil || !asana.IsRateLimited(fetchErr) {
						break
					}
					delay := asana.RetryAfter(fetchErr)
					if delay <= 0 {
						delay = time.Duration(attempt+1) * 5 * time.Second
					}
					time.Sleep(delay)
				}
				if fetchErr != nil {
					return fmt.Errorf("failed to fetch tasks for section %q: %w", section.Name, fetchErr)
				}

				tasks = append(tasks, batch...)

				if nextPage == nil || nextPage.Offset == "" {
					break
				}

				sectionOpts.Offset = nextPage.Offset
			}

			results[i] = tasks
			totalFetched.Add(int64(len(tasks)))
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	sectionsWithTasks := make([]sectionTasks, 0, len(sections))
	totalTasks := 0

	for i, section := range sections {
		tasks := results[i]

		if opts.Limit > 0 && totalTasks+len(tasks) >= opts.Limit {
			remaining := min(opts.Limit-totalTasks, len(tasks))
			tasks = tasks[:remaining]
		}

		totalTasks += len(tasks)
		sectionsWithTasks = append(sectionsWithTasks, sectionTasks{
			section: section,
			tasks:   tasks,
		})

		if opts.Limit > 0 && totalTasks >= opts.Limit {
			break
		}
	}

	return displayTasksBySection(opts, project, sectionsWithTasks)
}

type jsonTask struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type jsonSectionTasks struct {
	Section string     `json:"section"`
	Tasks   []jsonTask `json:"tasks"`
}

func displayTasks(opts *TasksOptions, project *asana.Project, tasks []*asana.Task) error {
	if opts.JSON {
		out := make([]jsonTask, len(tasks))
		for i, t := range tasks {
			out[i] = jsonTask{ID: t.ID, Name: t.Name}
		}
		enc := json.NewEncoder(opts.IO.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	cs := opts.IO.ColorScheme()
	out := opts.IO.Out

	fmt.Fprintf(out, "\nTasks in %s:\n\n", cs.Bold(project.Name))

	if len(tasks) == 0 {
		fmt.Fprintf(opts.IO.Out, "No tasks found\n")
		return nil
	}

	for i, task := range tasks {
		fmt.Fprintf(out, "%d. %s (ID: %s)\n", i+1, cs.Bold(task.Name), task.ID)
	}

	return nil
}

func displayTasksBySection(
	opts *TasksOptions,
	project *asana.Project,
	sections []sectionTasks,
) error {
	if opts.JSON {
		out := make([]jsonSectionTasks, len(sections))
		for i, st := range sections {
			tasks := make([]jsonTask, len(st.tasks))
			for j, t := range st.tasks {
				tasks[j] = jsonTask{ID: t.ID, Name: t.Name}
			}
			out[i] = jsonSectionTasks{Section: st.section.Name, Tasks: tasks}
		}
		enc := json.NewEncoder(opts.IO.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	cs := opts.IO.ColorScheme()
	out := opts.IO.Out

	fmt.Fprintf(out, "\nTasks in %s:\n\n", cs.Bold(project.Name))

	if len(sections) == 0 {
		fmt.Fprintln(out, "No sections found")
		return nil
	}

	for _, st := range sections {
		fmt.Fprintf(out, "%s:\n", cs.Bold(st.section.Name))

		if len(st.tasks) == 0 {
			fmt.Fprintln(out, "  No tasks in this section")
		}

		for i, task := range st.tasks {
			fmt.Fprintf(out, "  %d. %s (ID: %s)\n", i+1, task.Name, task.ID)
		}
		fmt.Fprintln(out)
	}

	return nil
}
