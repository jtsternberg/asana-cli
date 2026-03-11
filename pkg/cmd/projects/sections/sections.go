package sections

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/internal/config"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/iostreams"
)

type SectionsOptions struct {
	IO     *iostreams.IOStreams
	Config func() (*config.Config, error)
	Client func() (*asana.Client, error)

	ProjectName string
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

	return cmd
}

func runSections(opts *SectionsOptions) error {
	cs := opts.IO.ColorScheme()

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
	options := &asana.Options{}
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

	fmt.Fprintf(opts.IO.Out, "\nSections in %s:\n\n", cs.Bold(project.Name))
	if len(sections) == 0 {
		fmt.Fprintln(opts.IO.Out, "No sections found")
		return nil
	}
	for i, s := range sections {
		fmt.Fprintf(opts.IO.Out, "  %d. %s\n", i+1, s.Name)
	}

	return nil
}
