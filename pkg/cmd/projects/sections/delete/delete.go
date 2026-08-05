package delete

import (
	"encoding/json"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/jtsternberg/asana-cli/internal/api/asana"
	"github.com/jtsternberg/asana-cli/internal/config"
	"github.com/jtsternberg/asana-cli/internal/prompter"
	"github.com/jtsternberg/asana-cli/pkg/cmd/projects/shared"
	"github.com/jtsternberg/asana-cli/pkg/factory"
	"github.com/jtsternberg/asana-cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

type DeleteOptions struct {
	IO       *iostreams.IOStreams
	Prompter prompter.Prompter
	Config   func() (*config.Config, error)
	Client   func() (*asana.Client, error)

	ProjectName string
	SectionName string
	Force       bool
	Yes         bool
	JSON        bool
}

func NewCmdDelete(f factory.Factory, runF func(*DeleteOptions) error) *cobra.Command {
	opts := &DeleteOptions{
		IO:       f.IOStreams,
		Prompter: f.Prompter,
		Config:   f.Config,
		Client:   f.Client,
	}

	cmd := &cobra.Command{
		Use:   "delete <project> <section>",
		Short: "Delete a section from a project",
		Long: heredoc.Doc(`
			Delete a section from a project.

			The section is identified by name or ID. An ambiguous name is an error
			listing every candidate, not a first-match guess — deleting the wrong
			section cannot be undone from the CLI.

			A section that still holds tasks is refused, since deleting it moves
			those tasks to the project's default section rather than deleting them.
			Move the tasks out first (see 'asana tasks move'), or pass --force.
		`),
		Args: cobra.ExactArgs(2),
		Example: heredoc.Doc(`
			# Delete an empty section, confirming interactively
			$ asana projects sections delete Lindris "Q3 2026 Rocks - Ben"

			# Scripted: no prompt, still refuses a non-empty section
			$ asana projects sections delete Lindris 1217122949444271 --yes

			# Delete a section that still has tasks (they move to the default section)
			$ asana projects sections delete Lindris "Old Ideas" --force --yes
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ProjectName = args[0]
			opts.SectionName = args[1]
			if runF != nil {
				return runF(opts)
			}
			return runDelete(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.Force, "force", false, "Delete even if the section still has tasks")
	cmd.Flags().BoolVarP(&opts.Yes, "yes", "y", false, "Skip the confirmation prompt")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output in JSON format")

	return cmd
}

func runDelete(opts *DeleteOptions) error {
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

	return deleteSection(opts, client, project)
}

func deleteSection(opts *DeleteOptions, client *asana.Client, project *asana.Project) error {
	section, err := shared.ResolveSection(client, project, opts.SectionName)
	if err != nil {
		return err
	}

	// Count first, so the refusal can say how many tasks are in the way. Asana
	// also refuses server-side, but only after the CLI has already asked the
	// user to confirm, and with an error that names neither the count nor the
	// way out.
	if !opts.Force {
		count, err := countSectionTasks(client, section)
		if err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf(
				"section %q still has %s — move them out first (asana tasks move), or pass --force to delete the section anyway (the tasks are not deleted; they move to the project's default section)",
				section.Name, pluralTasks(count))
		}
	}

	if !opts.Yes {
		confirmed, err := opts.Prompter.Confirm(
			fmt.Sprintf("Delete section %q from %q?", section.Name, project.Name), "No")
		if err != nil {
			if prompter.IsNoInput(err) {
				return fmt.Errorf("cannot confirm without a terminal: pass --yes to delete %q without prompting", section.Name)
			}
			return fmt.Errorf("failed to confirm: %w", err)
		}
		if !confirmed {
			return fmt.Errorf("cancelled: section %q was not deleted", section.Name)
		}
	}

	if err := section.Delete(client); err != nil {
		return fmt.Errorf("failed to delete section %q: %w", section.Name, err)
	}

	return displayDeleted(opts, project, section)
}

// countSectionTasks reports how many tasks the section holds, stopping as soon
// as it knows the answer is non-zero. Only the count matters, so it asks for the
// smallest page the API allows and no optional fields.
func countSectionTasks(client *asana.Client, section *asana.Section) (int, error) {
	count := 0
	options := &asana.Options{Limit: 100, Fields: []string{}}

	for {
		batch, nextPage, err := section.Tasks(client, options)
		if err != nil {
			return 0, fmt.Errorf("failed to check whether section %q is empty: %w", section.Name, err)
		}

		count += len(batch)

		if nextPage == nil || nextPage.Offset == "" {
			return count, nil
		}

		options.Offset = nextPage.Offset
	}
}

func pluralTasks(n int) string {
	if n == 1 {
		return "1 task"
	}
	return fmt.Sprintf("%d tasks", n)
}

type jsonDeleted struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ProjectID string `json:"project_id"`
	Deleted   bool   `json:"deleted"`
}

func displayDeleted(opts *DeleteOptions, project *asana.Project, section *asana.Section) error {
	if opts.JSON {
		enc := json.NewEncoder(opts.IO.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(jsonDeleted{
			ID:        section.ID,
			Name:      section.Name,
			ProjectID: project.ID,
			Deleted:   true,
		})
	}

	cs := opts.IO.ColorScheme()
	fmt.Fprintf(opts.IO.Out, "%s Deleted section %s from %s (ID: %s)\n",
		cs.SuccessIcon, cs.Bold(section.Name), cs.Bold(project.Name), section.ID)
	return nil
}
