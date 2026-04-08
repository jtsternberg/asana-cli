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
	"github.com/timwehrle/asana/pkg/cmd/projects/shared"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/iostreams"
	"github.com/timwehrle/asana/pkg/sorting"
)

type ListOptions struct {
	IO *iostreams.IOStreams

	Config func() (*config.Config, error)
	Client func() (*asana.Client, error)

	Limit    int
	Sort     string
	Search   string
	Favorite bool
	JSON     bool
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
		Short:   "List projects from your default workspace",
		Long: heredoc.Doc(
			`Retrieve and display a list of all projects under your default workspace.`,
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Limit < 0 {
				return fmt.Errorf("invalid limit: %v", opts.Limit)
			}

			if runF != nil {
				return runF(opts)
			}

			return runList(opts)
		},
	}

	cmd.Flags().IntVarP(&opts.Limit, "limit", "l", 0, "Max number of projects to display")
	cmd.Flags().
		StringVarP(&opts.Sort, "sort", "s", "", "Sort projects by name (options: asc, desc)")
	cmd.Flags().StringVarP(&opts.Search, "search", "q", "", "Search projects by name")
	cmd.Flags().BoolVarP(&opts.Favorite, "favorite", "f", false, "List your favorite projects")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output in JSON format")

	return cmd
}

func runList(opts *ListOptions) error {
	cfg, err := opts.Config()
	if err != nil {
		return err
	}

	client, err := opts.Client()
	if err != nil {
		return err
	}

	var projects []*asana.Project
	workspace := &asana.Workspace{
		ID: cfg.Workspace.ID,
	}

	if opts.Search != "" {
		projects, err = workspace.SearchProjects(client, opts.Search, opts.Limit)
	} else if opts.Favorite {
		projects, err = fetchFavoriteProjects(client, workspace, opts.Limit)
	} else {
		projects, err = shared.FetchAllProjects(client, workspace, opts.Limit)
	}
	if err != nil {
		return err
	}

	if opts.Sort != "" {
		switch opts.Sort {
		case "asc":
			sorting.ProjectSort.ByName(projects)
		case "desc":
			sorting.ProjectSort.ByNameDesc(projects)
		}
	}

	return renderOutput(projects, opts.IO, opts.JSON, cfg.Workspace.Name)
}

func renderOutput(projects []*asana.Project, io *iostreams.IOStreams, jsonOutput bool, workspaceName string) error {
	if jsonOutput {
		return renderJSON(projects, io)
	}
	return renderText(projects, io, workspaceName)
}

func renderJSON(projects []*asana.Project, io *iostreams.IOStreams) error {
	type jsonRef struct {
		ID   string `json:"id"`
		Name string `json:"name,omitempty"`
	}
	type jsonProject struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Archived    *bool    `json:"archived"`
		Color       string   `json:"color,omitempty"`
		DefaultView string   `json:"default_view,omitempty"`
		DueOn       string   `json:"due_on,omitempty"`
		StartOn     string   `json:"start_on,omitempty"`
		Notes       string   `json:"notes,omitempty"`
		Owner       *jsonRef `json:"owner"`
		Team        *jsonRef `json:"team"`
		Public      *bool    `json:"public"`
		CreatedAt   string   `json:"created_at,omitempty"`
		ModifiedAt  string   `json:"modified_at,omitempty"`
	}

	out := make([]jsonProject, len(projects))
	for i, p := range projects {
		jp := jsonProject{
			ID:          p.ID,
			Name:        p.Name,
			Archived:    p.Archived,
			Color:       p.Color,
			DefaultView: string(p.DefaultView),
			Notes:       p.Notes,
			Public:      p.Public,
		}

		if p.DueOn != nil {
			jp.DueOn = time.Time(*p.DueOn).Format("2006-01-02")
		}
		if p.StartOn != nil {
			jp.StartOn = time.Time(*p.StartOn).Format("2006-01-02")
		}
		if p.CreatedAt != nil {
			jp.CreatedAt = p.CreatedAt.Format(time.RFC3339)
		}
		if p.ModifiedAt != nil {
			jp.ModifiedAt = p.ModifiedAt.Format(time.RFC3339)
		}
		if p.Owner != nil {
			jp.Owner = &jsonRef{ID: p.Owner.ID, Name: p.Owner.Name}
		}
		if p.Team != nil {
			jp.Team = &jsonRef{ID: p.Team.ID, Name: p.Team.Name}
		}

		out[i] = jp
	}

	enc := json.NewEncoder(io.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func renderText(projects []*asana.Project, io *iostreams.IOStreams, workspaceName string) error {
	cs := io.ColorScheme()
	fmt.Fprintf(io.Out, "\nProjects in %s:\n\n", cs.Bold(workspaceName))

	if len(projects) == 0 {
		fmt.Fprintln(io.Out, "No projects found")
		return nil
	}

	for i, p := range projects {
		ownerName := ""
		if p.Owner != nil {
			ownerName = p.Owner.Name
		}
		teamName := ""
		if p.Team != nil {
			teamName = p.Team.Name
		}
		dueOn := ""
		if p.DueOn != nil {
			dueOn = time.Time(*p.DueOn).Format("2006-01-02")
		}

		// Line 1: number + bold name
		line := fmt.Sprintf("%d. %s", i+1, cs.Bold(p.Name))

		// Append metadata inline
		var meta []string
		if ownerName != "" {
			meta = append(meta, ownerName)
		}
		if teamName != "" {
			meta = append(meta, teamName)
		}
		if dueOn != "" {
			meta = append(meta, fmt.Sprintf("Due: %s", dueOn))
		}
		if len(meta) > 0 {
			line += "  " + cs.Gray("("+joinParts(meta)+")")
		}

		line += "  " + cs.Gray(p.ID)
		fmt.Fprintln(io.Out, line)
	}

	return nil
}

// joinParts joins non-empty strings with " | ".
func joinParts(parts []string) string {
	return fmt.Sprintf("%s", strings.Join(parts, " | "))
}

func fetchFavoriteProjects(
	client *asana.Client,
	workspace *asana.Workspace,
	limit int,
) ([]*asana.Project, error) {
	initialCapacity := 100
	if limit > 0 {
		initialCapacity = limit
	}

	if err := workspace.Fetch(client); err != nil {
		return nil, err
	}

	favorites := make([]*asana.Project, 0, initialCapacity)
	options := &asana.Options{
		Limit: limit,
	}

	for {
		batch, nextPage, err := workspace.FavoriteProjects(client, options)
		if err != nil {
			return nil, err
		}

		favorites = append(favorites, batch...)

		if limit > 0 && len(favorites) > limit {
			favorites = favorites[:limit]
			break
		}

		if nextPage == nil || nextPage.Offset == "" {
			break
		}

		options.Offset = nextPage.Offset
	}

	return favorites, nil
}
