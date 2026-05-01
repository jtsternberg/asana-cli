package sections

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/internal/config"
	"github.com/timwehrle/asana/pkg/cmd/projects/sections/create"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/iostreams"
)

type SectionsOptions struct {
	IO     *iostreams.IOStreams
	Config func() (*config.Config, error)
	Client func() (*asana.Client, error)

	ProjectName string
	JSON        bool
}

func NewCmdSections(f factory.Factory, runF func(*SectionsOptions) error) *cobra.Command {
	opts := &SectionsOptions{
		IO:     f.IOStreams,
		Config: f.Config,
		Client: f.Client,
	}

	cmd := &cobra.Command{
		Use:   "sections <project-name>",
		Short: "List sections of a project",
		Long:  "Retrieve and display all sections under a project.",
		Args:  cobra.ExactArgs(1),
		Example: heredoc.Doc(`
			$ asana projects sections Lindris
			$ asana projects sections "Outgoing Tasks"
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ProjectName = args[0]
			if runF != nil {
				return runF(opts)
			}
			return runSections(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output in JSON format")

	cmd.AddCommand(create.NewCmdCreate(f, nil))

	return cmd
}

func runSections(opts *SectionsOptions) error {
	cfg, err := opts.Config()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	client, err := opts.Client()
	if err != nil {
		return fmt.Errorf("failed to initialize Asana client: %w", err)
	}

	ws := &asana.Workspace{ID: cfg.Workspace.ID}
	projects, err := ws.AllProjects(client)
	if err != nil {
		return fmt.Errorf("failed to fetch projects: %w", err)
	}

	var project *asana.Project
	nameLower := strings.ToLower(opts.ProjectName)
	for _, p := range projects {
		if strings.ToLower(p.Name) == nameLower || p.ID == opts.ProjectName {
			project = p
			break
		}
	}
	if project == nil {
		for _, p := range projects {
			if strings.Contains(strings.ToLower(p.Name), nameLower) {
				project = p
				break
			}
		}
	}
	if project == nil {
		return fmt.Errorf("project %q not found in workspace", opts.ProjectName)
	}

	sections := make([]*asana.Section, 0, 20)
	options := &asana.Options{Limit: 100}
	for {
		batch, nextPage, err := project.Sections(client, options)
		if err != nil {
			return fmt.Errorf("failed to fetch sections: %w", err)
		}
		sections = append(sections, batch...)
		if nextPage == nil || nextPage.Offset == "" {
			break
		}
		options.Offset = nextPage.Offset
	}

	return displaySections(opts, project, sections)
}

type jsonSection struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at,omitempty"`
}

func displaySections(opts *SectionsOptions, project *asana.Project, sections []*asana.Section) error {
	if opts.JSON {
		out := make([]jsonSection, len(sections))
		for i, s := range sections {
			js := jsonSection{ID: s.ID, Name: s.Name}
			if s.CreatedAt != nil {
				js.CreatedAt = s.CreatedAt.Format(time.RFC3339)
			}
			out[i] = js
		}
		enc := json.NewEncoder(opts.IO.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	cs := opts.IO.ColorScheme()

	fmt.Fprintf(opts.IO.Out, "\nSections in %s:\n\n", cs.Bold(project.Name))
	if len(sections) == 0 {
		fmt.Fprintln(opts.IO.Out, "No sections found")
		return nil
	}
	for i, s := range sections {
		fmt.Fprintf(opts.IO.Out, "  %d. %s (ID: %s)\n", i+1, s.Name, s.ID)
	}

	return nil
}
