package create

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/internal/config"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/iostreams"
)

type CreateOptions struct {
	IO     *iostreams.IOStreams
	Config func() (*config.Config, error)
	Client func() (*asana.Client, error)

	ProjectName string
	SectionName string
	JSON        bool
}

func NewCmdCreate(f factory.Factory, runF func(*CreateOptions) error) *cobra.Command {
	opts := &CreateOptions{
		IO:     f.IOStreams,
		Config: f.Config,
		Client: f.Client,
	}

	cmd := &cobra.Command{
		Use:   "create <project> <section-name>",
		Short: "Create a new section in a project",
		Long:  "Create a new section in the given project.",
		Args:  cobra.ExactArgs(2),
		Example: heredoc.Doc(`
			$ asana projects sections create "Outgoing Tasks" Ben
			$ asana projects sections create 1204651307630741 Ben --json
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ProjectName = args[0]
			opts.SectionName = args[1]
			if runF != nil {
				return runF(opts)
			}
			return runCreate(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output in JSON format")

	return cmd
}

func runCreate(opts *CreateOptions) error {
	if strings.TrimSpace(opts.SectionName) == "" {
		return fmt.Errorf("section name cannot be empty")
	}

	cfg, err := opts.Config()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	ws, err := cfg.RequireWorkspace()
	if err != nil {
		return err
	}

	client, err := opts.Client()
	if err != nil {
		return fmt.Errorf("failed to initialize Asana client: %w", err)
	}

	project, err := resolveProject(client, ws.ID, opts.ProjectName)
	if err != nil {
		return err
	}

	section, err := project.CreateSection(client, &asana.SectionBase{Name: opts.SectionName})
	if err != nil {
		return fmt.Errorf("failed to create section: %w", err)
	}

	return displaySection(opts, project, section)
}

func resolveProject(client *asana.Client, workspaceID, nameOrID string) (*asana.Project, error) {
	ws := &asana.Workspace{ID: workspaceID}
	projects, err := ws.AllProjects(client)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch projects: %w", err)
	}

	nameLower := strings.ToLower(nameOrID)
	for _, p := range projects {
		if strings.ToLower(p.Name) == nameLower || p.ID == nameOrID {
			return p, nil
		}
	}
	for _, p := range projects {
		if strings.Contains(strings.ToLower(p.Name), nameLower) {
			return p, nil
		}
	}
	return nil, fmt.Errorf("project %q not found in workspace", nameOrID)
}

type jsonSection struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ProjectID string `json:"project_id,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

func displaySection(opts *CreateOptions, project *asana.Project, section *asana.Section) error {
	if opts.JSON {
		js := jsonSection{ID: section.ID, Name: section.Name, ProjectID: project.ID}
		if section.CreatedAt != nil {
			js.CreatedAt = section.CreatedAt.Format(time.RFC3339)
		}
		enc := json.NewEncoder(opts.IO.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(js)
	}

	cs := opts.IO.ColorScheme()
	fmt.Fprintf(opts.IO.Out, "%s Created section %s in %s (ID: %s)\n",
		cs.SuccessIcon, cs.Bold(section.Name), cs.Bold(project.Name), section.ID)
	return nil
}
