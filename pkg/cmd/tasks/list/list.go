package list

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/timwehrle/asana/internal/config"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/format"
	"github.com/timwehrle/asana/pkg/iostreams"
	"github.com/timwehrle/asana/pkg/sorting"
)

type SortOption string

const (
	SortAsc       SortOption = "asc"
	SortDesc      SortOption = "desc"
	SortDue       SortOption = "due"
	SortDueDesc   SortOption = "due-desc"
	SortCreatedAt SortOption = "created-at"
)

var validSortOptions = map[SortOption]struct{}{
	SortAsc:       {},
	SortDesc:      {},
	SortDue:       {},
	SortDueDesc:   {},
	SortCreatedAt: {},
}

type ListOptions struct {
	IO *iostreams.IOStreams

	Config func() (*config.Config, error)
	Client func() (*asana.Client, error)

	Sort  SortOption
	Limit int
	User  string
	JSON  bool
}

func (o *ListOptions) ResolveUser() string {
	if o.User == "" {
		return "me"
	}
	return o.User
}

func NewCmdList(f factory.Factory, runF func(*ListOptions) error) *cobra.Command {
	opts := &ListOptions{
		IO:     f.IOStreams,
		Config: f.Config,
		Client: f.Client,
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all tasks",
		Long: heredoc.Doc(`
				Retrieve and display a list of all tasks assigned to your Asana account.
				Tasks can be sorted by name, due date, or creation date.
			`),
		Example: heredoc.Doc(`
				# List all tasks
				$ asana tasks list

				# List tasks sorted by due date (descending)
				$ asana task list --sort due-desc
			`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateSortOption(opts.Sort); err != nil {
				return err
			}

			if runF != nil {
				return runF(opts)
			}

			return listRun(opts)
		},
	}

	cmd.Flags().
		StringVarP((*string)(&opts.Sort), "sort", "s", "", "Sort tasks by name, due date, creation date (options: asc, desc, due, due-desc, created-at)")
	cmd.Flags().IntVarP(&opts.Limit, "limit", "l", 0, "Limit the tasks to display")
	cmd.Flags().StringVarP(&opts.User, "user", "u", "", "Show the task list of the provided user")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output in JSON format")

	return cmd
}

func validateSortOption(opt SortOption) error {
	if opt == "" {
		return nil
	}

	if _, ok := validSortOptions[opt]; !ok {
		return fmt.Errorf(
			"invalid sort option %q. Available options: asc, desc, due, due-desc, created-at",
			opt,
		)
	}
	return nil
}

func listRun(opts *ListOptions) error {
	cfg, err := opts.Config()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	ws, err := cfg.RequireWorkspace()
	if err != nil {
		return err
	}

	tasks, err := fetchTasks(opts, ws.ID, opts.Limit)
	if err != nil {
		return err
	}

	if len(tasks) == 0 {
		return printEmptyMessage(opts.IO)
	}

	sortTasks(tasks, opts.Sort)

	if opts.JSON {
		return displayJSON(opts, tasks)
	}

	return printTasks(opts.IO, cfg.Username, tasks)
}

func fetchTasks(opts *ListOptions, workspaceID string, limit int) ([]*asana.Task, error) {
	initialCapacity := 100
	if limit > 0 {
		initialCapacity = limit
	}

	client, err := opts.Client()
	if err != nil {
		return nil, fmt.Errorf("failed to create Asana client: %w", err)
	}

	query := &asana.TaskQuery{
		Assignee:       opts.ResolveUser(),
		Workspace:      workspaceID,
		CompletedSince: "now",
	}

	tasks := make([]*asana.Task, 0, initialCapacity)
	options := &asana.Options{
		Fields: []string{
			"name", "due_on", "created_at", "assignee", "assignee.name",
			"completed", "projects", "projects.name", "tags", "tags.name",
			"permalink_url", "resource_subtype", "notes", "start_on",
			"due_at", "custom_fields", "custom_fields.name",
			"custom_fields.display_value", "num_subtasks", "parent",
			"parent.name",
		},
		Limit: limit,
	}

	for {
		batch, nextPage, err := client.QueryTasks(query, options)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch tasks: %w", err)
		}

		tasks = append(tasks, batch...)

		if limit > 0 && len(tasks) > limit {
			tasks = tasks[:limit]
			break
		}

		if nextPage == nil || nextPage.Offset == "" {
			break
		}

		options.Offset = nextPage.Offset
	}

	return tasks, nil
}

func sortTasks(tasks []*asana.Task, sortOption SortOption) {
	switch sortOption {
	case SortAsc:
		sorting.TaskSort.ByName(tasks)
	case SortDesc:
		sorting.TaskSort.ByNameDesc(tasks)
	case SortDue:
		sorting.TaskSort.ByDueDate(tasks)
	case SortDueDesc:
		sorting.TaskSort.ByDueDateDesc(tasks)
	case SortCreatedAt:
		sorting.TaskSort.ByCreatedAt(tasks)
	case "":
		// No sorting requested
	}
}

func printEmptyMessage(io *iostreams.IOStreams) error {
	fmt.Fprintln(io.Out, "No tasks found.")
	return nil
}

func displayJSON(opts *ListOptions, tasks []*asana.Task) error {
	type jsonRef struct {
		ID   string `json:"id"`
		Name string `json:"name,omitempty"`
	}
	type jsonCustomField struct {
		ID           string  `json:"id"`
		Name         string  `json:"name"`
		DisplayValue *string `json:"display_value"`
	}
	type jsonTask struct {
		ID              string             `json:"id"`
		Name            string             `json:"name"`
		ResourceSubtype string             `json:"resource_subtype,omitempty"`
		Assignee        *jsonRef           `json:"assignee"`
		Completed       *bool              `json:"completed"`
		DueOn           string             `json:"due_on,omitempty"`
		DueAt           string             `json:"due_at,omitempty"`
		StartOn         string             `json:"start_on,omitempty"`
		Notes           string             `json:"notes,omitempty"`
		Parent          *jsonRef           `json:"parent,omitempty"`
		Projects        []*jsonRef         `json:"projects,omitempty"`
		Tags            []*jsonRef         `json:"tags,omitempty"`
		CustomFields    []*jsonCustomField `json:"custom_fields,omitempty"`
		NumSubtasks     int32              `json:"num_subtasks"`
		PermalinkURL    string             `json:"permalink_url,omitempty"`
	}

	out := make([]jsonTask, len(tasks))
	for i, t := range tasks {
		jt := jsonTask{
			ID:              t.ID,
			Name:            t.Name,
			ResourceSubtype: t.ResourceSubtype,
			Completed:       t.Completed,
			Notes:           t.Notes,
			NumSubtasks:     t.NumSubtasks,
			PermalinkURL:    t.PermalinkURL,
		}

		if t.Assignee != nil {
			jt.Assignee = &jsonRef{ID: t.Assignee.ID, Name: t.Assignee.Name}
		}
		if t.Parent != nil {
			jt.Parent = &jsonRef{ID: t.Parent.ID, Name: t.Parent.Name}
		}
		if t.DueOn != nil {
			jt.DueOn = time.Time(*t.DueOn).Format("2006-01-02")
		}
		if t.DueAt != nil {
			jt.DueAt = t.DueAt.Format(time.RFC3339)
		}
		if t.StartOn != nil {
			jt.StartOn = time.Time(*t.StartOn).Format("2006-01-02")
		}
		for _, p := range t.Projects {
			jt.Projects = append(jt.Projects, &jsonRef{ID: p.ID, Name: p.Name})
		}
		for _, tag := range t.Tags {
			jt.Tags = append(jt.Tags, &jsonRef{ID: tag.ID, Name: tag.Name})
		}
		for _, cf := range t.CustomFields {
			jt.CustomFields = append(jt.CustomFields, &jsonCustomField{
				ID:           cf.ID,
				Name:         cf.Name,
				DisplayValue: cf.DisplayValue,
			})
		}
		out[i] = jt
	}

	enc := json.NewEncoder(opts.IO.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func printTasks(io *iostreams.IOStreams, username string, tasks []*asana.Task) error {
	cs := io.ColorScheme()

	fmt.Fprintf(io.Out, "\nTasks for %s:\n\n", cs.Bold(username))

	for i, task := range tasks {
		assignee := "Unassigned"
		if task.Assignee != nil && task.Assignee.Name != "" {
			assignee = task.Assignee.Name
		}

		due := format.Date(task.DueOn)

		projects := "-"
		if len(task.Projects) > 0 {
			names := make([]string, len(task.Projects))
			for j, p := range task.Projects {
				names[j] = p.Name
			}
			projects = strings.Join(names, ", ")
		}

		status := "Incomplete"
		if task.Completed != nil && *task.Completed {
			status = "Completed"
		}

		fmt.Fprintf(io.Out, "%d. %s | %s | %s | %s | %s | ID: %s\n",
			i+1,
			cs.Bold(task.Name),
			assignee,
			due,
			projects,
			status,
			task.ID,
		)
	}

	return nil
}
