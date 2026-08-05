package list

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/jtsternberg/asana-cli/internal/api/asana"
	"github.com/jtsternberg/asana-cli/internal/config"
	"github.com/jtsternberg/asana-cli/pkg/factory"
	"github.com/jtsternberg/asana-cli/pkg/iostreams"
	"github.com/jtsternberg/asana-cli/pkg/sorting"
	"github.com/spf13/cobra"
)

type ListOptions struct {
	IO *iostreams.IOStreams

	Config func() (*config.Config, error)
	Client func() (*asana.Client, error)

	Query  string
	Limit  int
	Sort   string
	WithID bool
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
		Short:   "List users in your Asana workspace",
		Args:    cobra.NoArgs,
		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
			# List all users
			$ asana users list
			
			# List first 10 users
			$ asana users list --limit 10

			# List users sorted by name (descending)
			$ asana users list --sort desc

			# Find the people a first name could mean, with their IDs
			$ asana users list -q David
			$ asana users list -q alyssa --json
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF != nil {
				return runF(opts)
			}

			return runList(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Query, "query", "q", "", "Only show users whose name or email contains this text (case-insensitive)")
	cmd.Flags().IntVarP(&opts.Limit, "limit", "l", 0, "Limit the number of users to display")
	cmd.Flags().StringVarP(&opts.Sort, "sort", "s", "", "Sort users by name (asc, desc)")
	cmd.Flags().BoolVar(&opts.WithID, "with-id", false, "Show users with their user IDs")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output in JSON format")

	return cmd
}

func runList(opts *ListOptions) error {
	cfg, err := opts.Config()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	ws, err := cfg.RequireWorkspace()
	if err != nil {
		return err
	}

	client, err := opts.Client()
	if err != nil {
		return fmt.Errorf("failed to create Asana client: %w", err)
	}

	// With a query, --limit has to cap the *matches*, not the rows scanned, so
	// the fetch stays unbounded and the cap is applied after filtering.
	fetchLimit := opts.Limit
	if opts.Query != "" {
		fetchLimit = 0
	}

	users, err := fetchUsers(client, ws.ID, fetchLimit)
	if err != nil {
		return fmt.Errorf("failed to fetch users: %w", err)
	}

	if opts.Query != "" {
		users = filterUsers(users, opts.Query)
		if opts.Limit > 0 && len(users) > opts.Limit {
			users = users[:opts.Limit]
		}
	}

	if err := sortUsers(users, opts.Sort); err != nil {
		return err
	}

	if opts.JSON {
		return printUsersJSON(opts.IO, users)
	}

	return printUsers(opts.IO, ws.Name, users, opts.WithID || opts.Query != "")
}

func sortUsers(users []*asana.User, sortOrder string) error {
	if sortOrder == "" {
		return nil
	}

	switch strings.ToLower(sortOrder) {
	case "asc":
		sorting.Sort(users, func(a, b *asana.User) bool {
			return a.Name < b.Name
		})
	case "desc":
		sorting.Sort(users, func(a, b *asana.User) bool {
			return a.Name > b.Name
		})
	default:
		return fmt.Errorf("invalid sort order: %q, valid values are: asc, desc", sortOrder)
	}

	return nil
}

// filterUsers keeps the users whose name or email contains query.
//
// This exists because the workspace has 241 people and 22 duplicated first
// names; "which David?" is a question the CLI has to be able to answer, and it
// is the question an ambiguity error from pkg/userref sends you here to ask.
func filterUsers(users []*asana.User, query string) []*asana.User {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return users
	}

	out := make([]*asana.User, 0, 8)
	for _, u := range users {
		if strings.Contains(strings.ToLower(u.Name), q) ||
			strings.Contains(strings.ToLower(u.Email), q) {
			out = append(out, u)
		}
	}
	return out
}

func fetchUsers(client *asana.Client, workspaceID string, limit int) ([]*asana.User, error) {
	initialCapacity := 100
	if limit > 0 {
		initialCapacity = limit
	}

	users := make([]*asana.User, 0, initialCapacity)
	options := &asana.Options{
		Fields: []string{"name", "email", "photo", "workspaces"},
	}
	if limit > 0 {
		options.Limit = limit
	}

	workspace := &asana.Workspace{ID: workspaceID}

	for {
		batch, nextPage, err := workspace.Users(client, options)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch users: %w", err)
		}

		users = append(users, batch...)

		if limit > 0 && len(users) >= limit {
			users = users[:limit]
			break
		}

		if nextPage == nil || nextPage.Offset == "" {
			break
		}

		options.Offset = nextPage.Offset
	}

	return users, nil
}

func printUsersJSON(io *iostreams.IOStreams, users []*asana.User) error {
	type jsonWorkspace struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	type jsonUser struct {
		ID         string            `json:"id"`
		Name       string            `json:"name"`
		Email      string            `json:"email"`
		Photo      map[string]string `json:"photo,omitempty"`
		Workspaces []jsonWorkspace   `json:"workspaces,omitempty"`
	}
	out := make([]jsonUser, len(users))
	for i, u := range users {
		ju := jsonUser{ID: u.ID, Name: u.Name, Email: u.Email}
		if u.Photo != nil {
			ju.Photo = u.Photo
		}
		if len(u.Workspaces) > 0 {
			ju.Workspaces = make([]jsonWorkspace, len(u.Workspaces))
			for j, ws := range u.Workspaces {
				ju.Workspaces[j] = jsonWorkspace{ID: ws.ID, Name: ws.Name}
			}
		}
		out[i] = ju
	}
	enc := json.NewEncoder(io.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func printUsers(io *iostreams.IOStreams, workspaceName string, users []*asana.User, showID bool) error {
	cs := io.ColorScheme()
	io.Printf("\nUsers in workspace %s:\n\n", cs.Bold(workspaceName))

	for i, user := range users {
		emailPart := ""
		if user.Email != "" {
			emailPart = fmt.Sprintf(" <%s>", user.Email)
		}

		if showID {
			io.Printf("%2d. %s%s (%s)\n", i+1, cs.Bold(user.Name), emailPart, cs.Gray(user.ID))
		} else {
			io.Printf("%2d. %s%s\n", i+1, cs.Bold(user.Name), emailPart)
		}
	}

	return nil
}
