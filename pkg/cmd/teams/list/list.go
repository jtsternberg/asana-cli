package list

import (
	"encoding/json"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/internal/config"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/iostreams"
)

type ListOptions struct {
	IO     *iostreams.IOStreams
	Config func() (*config.Config, error)
	Client func() (*asana.Client, error)
	JSON   bool
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
		Short:   "List all teams",
		Long: heredoc.Doc(`
				Retrieve and display a list of all teams assigned to your default workspace.
			`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF == nil {
				return listRun(opts)
			}
			return runF(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output in JSON format")

	return cmd
}

func listRun(opts *ListOptions) error {
	cfg, err := opts.Config()
	if err != nil {
		return err
	}

	ws, err := cfg.RequireWorkspace()
	if err != nil {
		return err
	}

	client, err := opts.Client()
	if err != nil {
		return err
	}

	teams, err := ws.AllTeams(client)
	if err != nil {
		return fmt.Errorf("failed to fetch teams: %w", err)
	}

	if opts.JSON {
		return displayJSON(teams, opts.IO)
	}

	displayText(teams, opts.IO, ws.Name)
	return nil
}

func displayJSON(teams []*asana.Team, io *iostreams.IOStreams) error {
	type jsonRef struct {
		ID   string `json:"id"`
		Name string `json:"name,omitempty"`
	}
	type jsonTeam struct {
		ID           string   `json:"id"`
		Name         string   `json:"name"`
		Description  string   `json:"description"`
		Organization *jsonRef `json:"organization"`
	}

	out := make([]jsonTeam, 0, len(teams))
	for _, team := range teams {
		jt := jsonTeam{
			ID:          team.ID,
			Name:        team.Name,
			Description: team.Description,
		}
		if team.Organization != nil {
			jt.Organization = &jsonRef{
				ID:   team.Organization.ID,
				Name: team.Organization.Name,
			}
		}
		out = append(out, jt)
	}

	enc := json.NewEncoder(io.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func displayText(teams []*asana.Team, io *iostreams.IOStreams, workspaceName string) {
	cs := io.ColorScheme()
	io.Printf("\nTeams in workspace %s:\n\n", cs.Bold(workspaceName))

	for i, team := range teams {
		desc := truncate(team.Description, 50)
		if desc != "" {
			io.Printf("%2d. %s  %s  %s\n", i+1, cs.Bold(team.Name), cs.Gray(desc), cs.Gray(team.ID))
		} else {
			io.Printf("%2d. %s  %s\n", i+1, cs.Bold(team.Name), cs.Gray(team.ID))
		}
	}
}

// truncate shortens s to max characters, appending "..." if truncated.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
