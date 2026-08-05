package move

import (
	"encoding/json"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/jtsternberg/asana-cli/internal/api/asana"
	"github.com/jtsternberg/asana-cli/internal/config"
	"github.com/jtsternberg/asana-cli/pkg/cmd/projects/shared"
	"github.com/jtsternberg/asana-cli/pkg/factory"
	"github.com/jtsternberg/asana-cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

type MoveOptions struct {
	IO     *iostreams.IOStreams
	Config func() (*config.Config, error)
	Client func() (*asana.Client, error)

	ProjectName string
	SectionName string
	Before      string
	After       string
	First       bool
	Last        bool
	JSON        bool
}

func NewCmdMove(f factory.Factory, runF func(*MoveOptions) error) *cobra.Command {
	opts := &MoveOptions{
		IO:     f.IOStreams,
		Config: f.Config,
		Client: f.Client,
	}

	cmd := &cobra.Command{
		Use:   "move <project> <section>",
		Short: "Reorder a section within a project",
		Long: heredoc.Doc(`
			Move a section to a different position in its project.

			New sections are always appended to the bottom of a project, so this is
			how a freshly created section gets to the top without dragging it in
			the Asana web UI.

			Exactly one destination is required: --before, --after, --first or
			--last. Sections are identified by name or ID, and an ambiguous name is
			an error listing every candidate rather than a first-match guess.
		`),
		Args: cobra.ExactArgs(2),
		Example: heredoc.Doc(`
			# Move a section to the top of the project
			$ asana projects sections move Lindris "Q3 2026 Rocks - Shared" --first

			# Position one section relative to another
			$ asana projects sections move Lindris "Q3 2026 Rocks - Ben" --after "Q3 2026 Rocks - Shared"
			$ asana projects sections move Lindris 1217122949444271 --before "Untitled section"

			# Send a section to the bottom
			$ asana projects sections move Lindris "Old Ideas" --last
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ProjectName = args[0]
			opts.SectionName = args[1]
			if runF != nil {
				return runF(opts)
			}
			return runMove(opts)
		},
	}

	cmd.Flags().StringVar(&opts.Before, "before", "", "Place the section immediately before this section (name or ID)")
	cmd.Flags().StringVar(&opts.After, "after", "", "Place the section immediately after this section (name or ID)")
	cmd.Flags().BoolVar(&opts.First, "first", false, "Move the section to the top of the project")
	cmd.Flags().BoolVar(&opts.Last, "last", false, "Move the section to the bottom of the project")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output in JSON format")

	cmd.MarkFlagsMutuallyExclusive("before", "after", "first", "last")

	return cmd
}

func runMove(opts *MoveOptions) error {
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

	project, err := shared.ResolveProject(client, &asana.Workspace{ID: ws.ID}, opts.ProjectName)
	if err != nil {
		return err
	}

	return moveSection(opts, client, project)
}

// destination describes where the section should land, in the terms the API
// takes: exactly one of before/after, as a section gid.
type destination struct {
	before string
	after  string
	// label describes the move for the success line ("to the top of the
	// project", `after "Q3 Rocks"`).
	label string
}

func moveSection(opts *MoveOptions, client *asana.Client, project *asana.Project) error {
	if opts.Before == "" && opts.After == "" && !opts.First && !opts.Last {
		return fmt.Errorf("a destination is required: pass one of --first, --last, --before <section> or --after <section>")
	}

	sections, err := shared.FetchAllSections(client, project)
	if err != nil {
		return err
	}
	if len(sections) == 0 {
		return fmt.Errorf("project %q has no sections", project.Name)
	}

	section, err := shared.FindSection(sections, project.Name, opts.SectionName)
	if err != nil {
		return err
	}

	dest, err := resolveDestination(opts, sections, project, section)
	if err != nil {
		return err
	}
	if dest == nil {
		// Already where it was asked to go. Reporting success without a request
		// is honest here, and saves an API call that would be a no-op anyway.
		return displayMoved(opts, project, section, "already in position")
	}

	request := &asana.SectionInsertRequest{
		Section:       section.ID,
		BeforeSection: dest.before,
		AfterSection:  dest.after,
	}

	if err := project.InsertSection(client, request); err != nil {
		return fmt.Errorf("failed to move section %q: %w", section.Name, err)
	}

	return displayMoved(opts, project, section, dest.label)
}

// resolveDestination turns the flags into a before/after gid pair. A nil
// destination means the section is already in the requested position.
func resolveDestination(
	opts *MoveOptions,
	sections []*asana.Section,
	project *asana.Project,
	section *asana.Section,
) (*destination, error) {
	switch {
	case opts.First:
		if sections[0].ID == section.ID {
			return nil, nil
		}
		return &destination{before: sections[0].ID, label: "to the top of the project"}, nil

	case opts.Last:
		last := sections[len(sections)-1]
		if last.ID == section.ID {
			return nil, nil
		}
		return &destination{after: last.ID, label: "to the bottom of the project"}, nil

	case opts.Before != "":
		anchor, err := shared.FindSection(sections, project.Name, opts.Before)
		if err != nil {
			return nil, fmt.Errorf("--before: %w", err)
		}
		if anchor.ID == section.ID {
			return nil, fmt.Errorf("--before %q is the section being moved", opts.Before)
		}
		return &destination{before: anchor.ID, label: fmt.Sprintf("before %q", anchor.Name)}, nil

	default:
		anchor, err := shared.FindSection(sections, project.Name, opts.After)
		if err != nil {
			return nil, fmt.Errorf("--after: %w", err)
		}
		if anchor.ID == section.ID {
			return nil, fmt.Errorf("--after %q is the section being moved", opts.After)
		}
		return &destination{after: anchor.ID, label: fmt.Sprintf("after %q", anchor.Name)}, nil
	}
}

type jsonMoved struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ProjectID string `json:"project_id"`
	Moved     string `json:"moved"`
}

func displayMoved(opts *MoveOptions, project *asana.Project, section *asana.Section, label string) error {
	if opts.JSON {
		enc := json.NewEncoder(opts.IO.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(jsonMoved{
			ID:        section.ID,
			Name:      section.Name,
			ProjectID: project.ID,
			Moved:     label,
		})
	}

	cs := opts.IO.ColorScheme()
	fmt.Fprintf(opts.IO.Out, "%s Section %s in %s: %s\n",
		cs.SuccessIcon, cs.Bold(section.Name), cs.Bold(project.Name), label)
	return nil
}
