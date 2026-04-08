package list

import (
	"encoding/json"
	"fmt"

	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/internal/config"
	"github.com/timwehrle/asana/internal/prompter"

	"github.com/MakeNowJust/heredoc"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/iostreams"

	"github.com/spf13/cobra"
)

type ListOptions struct {
	IO       *iostreams.IOStreams
	Prompter prompter.Prompter

	Config func() (*config.Config, error)
	Client func() (*asana.Client, error)

	JSON bool
}

func NewCmdList(f factory.Factory, runF func(*ListOptions) error) *cobra.Command {
	opts := &ListOptions{
		IO:       f.IOStreams,
		Prompter: f.Prompter,
		Config:   f.Config,
		Client:   f.Client,
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List available workspaces",
		Long: heredoc.Doc(`
				Retrieve and display a list of all workspaces associated 
				with your Asana account.`),
		Example: heredoc.Doc(`
				$ asana workspaces list
				$ asana workspaces ls
				$ asana ws ls
			`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF != nil {
				return runF(opts)
			}

			return runList(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output in JSON format")

	return cmd
}

func runList(opts *ListOptions) error {
	client, err := opts.Client()
	if err != nil {
		return err
	}

	workspaces, err := client.AllWorkspaces()
	if err != nil {
		return err
	}

	return displayWorkspaces(opts, workspaces)
}

func displayWorkspaces(opts *ListOptions, workspaces []*asana.Workspace) error {
	if len(workspaces) == 0 {
		cfg, err := opts.Config()
		if err != nil {
			return err
		}
		cs := opts.IO.ColorScheme()
		fmt.Fprintf(opts.IO.Out, "No workspaces found for %s", cs.Bold(cfg.Username))
		return nil
	}

	if opts.JSON {
		return displayWorkspacesJSON(opts.IO, workspaces)
	}
	return displayWorkspacesText(opts, workspaces)
}

func displayWorkspacesJSON(io *iostreams.IOStreams, workspaces []*asana.Workspace) error {
	type jsonWorkspace struct {
		ID             string   `json:"id"`
		Name           string   `json:"name"`
		IsOrganization bool     `json:"is_organization"`
		EmailDomains   []string `json:"email_domains,omitempty"`
	}

	out := make([]jsonWorkspace, len(workspaces))
	for i, ws := range workspaces {
		out[i] = jsonWorkspace{
			ID:             ws.ID,
			Name:           ws.Name,
			IsOrganization: ws.IsOrganization,
			EmailDomains:   ws.EmailDomains,
		}
	}

	enc := json.NewEncoder(io.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func displayWorkspacesText(opts *ListOptions, workspaces []*asana.Workspace) error {
	cs := opts.IO.ColorScheme()

	cfg, err := opts.Config()
	if err != nil {
		return err
	}

	fmt.Fprintf(opts.IO.Out, "\nWorkspaces of %s:\n\n", cs.Bold(cfg.Username))
	for i, ws := range workspaces {
		wsType := "Workspace"
		if ws.IsOrganization {
			wsType = "Organization"
		}
		fmt.Fprintf(opts.IO.Out, "%d. %s | %s | %s\n", i+1, cs.Bold(ws.Name), wsType, ws.ID)
	}

	return nil
}
