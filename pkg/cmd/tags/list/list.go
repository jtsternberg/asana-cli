package list

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/internal/config"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/iostreams"
)

type ListOptions struct {
	IO *iostreams.IOStreams

	Config func() (*config.Config, error)
	Client func() (*asana.Client, error)

	Limit    int
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
		Use:   "list",
		Short: "List tags from your default workspace",
		Long: heredoc.Doc(
			`Retrieve and display a list of all tags under your default workspace.`,
		),
		Aliases: []string{"ls"},
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

	cmd.Flags().IntVarP(&opts.Limit, "limit", "l", 0, "Max number of tags to display")
	cmd.Flags().BoolVarP(&opts.Favorite, "favorite", "f", false, "List your favorite tags")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output in JSON format")

	return cmd
}

func runList(opts *ListOptions) error {
	cfg, err := opts.Config()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	client, err := opts.Client()
	if err != nil {
		return err
	}

	var tags []*asana.Tag
	workspace := &asana.Workspace{ID: cfg.Workspace.ID}

	if opts.Favorite {
		tags, err = fetchFavoriteTags(client, workspace)
	} else {
		tags, err = fetchTags(client, workspace, opts.Limit)
	}
	if err != nil {
		return err
	}

	return displayTags(tags, opts.IO, opts.JSON, cfg.Workspace.Name)
}

func displayTags(tags []*asana.Tag, io *iostreams.IOStreams, jsonOutput bool, workspaceName string) error {
	if jsonOutput {
		return displayTagsJSON(tags, io)
	}
	return displayTagsText(tags, io, workspaceName)
}

func displayTagsJSON(tags []*asana.Tag, io *iostreams.IOStreams) error {
	type jsonRef struct {
		ID   string `json:"id"`
		Name string `json:"name,omitempty"`
	}
	type jsonTag struct {
		ID        string     `json:"id"`
		Name      string     `json:"name"`
		Notes     string     `json:"notes,omitempty"`
		Color     string     `json:"color,omitempty"`
		CreatedAt string     `json:"created_at,omitempty"`
		Workspace *jsonRef   `json:"workspace,omitempty"`
		Followers []*jsonRef `json:"followers,omitempty"`
	}

	result := make([]jsonTag, 0, len(tags))
	for _, t := range tags {
		jt := jsonTag{
			ID:    t.ID,
			Name:  t.Name,
			Notes: t.Notes,
			Color: t.Color,
		}
		if t.CreatedAt != nil {
			jt.CreatedAt = t.CreatedAt.Format(time.RFC3339)
		}
		if t.Workspace != nil {
			jt.Workspace = &jsonRef{ID: t.Workspace.ID, Name: t.Workspace.Name}
		}
		for _, f := range t.Followers {
			jt.Followers = append(jt.Followers, &jsonRef{ID: f.ID, Name: f.Name})
		}
		result = append(result, jt)
	}

	enc := json.NewEncoder(io.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func displayTagsText(tags []*asana.Tag, io *iostreams.IOStreams, workspaceName string) error {
	cs := io.ColorScheme()

	fmt.Fprintf(io.Out, "\nTags in %s:\n\n", cs.Bold(workspaceName))
	if len(tags) == 0 {
		fmt.Fprintln(io.Out, "No tags found")
		return nil
	}
	for i, t := range tags {
		color := t.Color
		if color == "" {
			color = "-"
		}
		fmt.Fprintf(io.Out, "%d. %s | %s | %s\n", i+1, cs.Bold(t.Name), color, cs.Gray(t.ID))
	}

	return nil
}

func fetchFavoriteTags(client *asana.Client, workspace *asana.Workspace) ([]*asana.Tag, error) {
	user := &asana.User{
		ID: "me",
	}

	query := &asana.UserQuery{
		ResourceType: "tag",
		Workspace:    workspace.ID,
	}

	var tags []*asana.Tag
	err := user.Favorite(client, query, &tags)
	if err != nil {
		return nil, fmt.Errorf("failed fetching favorite tags: %w", err)
	}

	return tags, nil
}

func fetchTags(client *asana.Client, workspace *asana.Workspace, limit int) ([]*asana.Tag, error) {
	initialCapacity := 100
	if limit > 0 {
		initialCapacity = limit
	}

	if err := workspace.Fetch(client); err != nil {
		return nil, err
	}

	tags := make([]*asana.Tag, 0, initialCapacity)
	options := &asana.Options{
		Limit: limit,
	}

	for {
		batch, nextPage, err := workspace.Tags(client)
		if err != nil {
			return nil, err
		}

		tags = append(tags, batch...)

		if limit > 0 && len(tags) > limit {
			tags = tags[:limit]
			break
		}

		if nextPage == nil || nextPage.Offset == "" {
			break
		}

		options.Offset = nextPage.Offset
	}

	return tags, nil
}
