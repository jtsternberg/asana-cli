package comments

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/iostreams"
)

// commentSubtype is the resource_subtype Asana assigns to comment stories.
const commentSubtype = "comment_added"

type CommentsOptions struct {
	IO     *iostreams.IOStreams
	Client func() (*asana.Client, error)

	TaskID string
	JSON   bool
}

func NewCmdComments(f factory.Factory, runF func(*CommentsOptions) error) *cobra.Command {
	opts := &CommentsOptions{
		IO:     f.IOStreams,
		Client: f.Client,
	}

	cmd := &cobra.Command{
		Use:   "comments <task-id>",
		Short: "View comments on a task",
		Long: heredoc.Doc(`
			Display the comments on a task.

			Asana stores comments as "stories" alongside system-generated activity
			(assignments, due-date changes, etc.). This command fetches the task's
			stories and shows only the comment-type ones, printing the author, text,
			and creation time of each.`),
		Args: cobra.ExactArgs(1),
		Example: heredoc.Doc(`
			# View comments on a task
			$ asana tasks comments 1234567890
			$ asana ts comments 1234567890

			# Output as JSON
			$ asana tasks comments 1234567890 --json`),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.TaskID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return runComments(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output in JSON format")

	return cmd
}

func runComments(opts *CommentsOptions) error {
	client, err := opts.Client()
	if err != nil {
		return fmt.Errorf("failed to initialize Asana client: %w", err)
	}

	task := &asana.Task{ID: opts.TaskID}
	if err := task.Fetch(client); err != nil {
		return fmt.Errorf("task %q not found: %w", opts.TaskID, err)
	}

	comments, err := fetchComments(client, task)
	if err != nil {
		return err
	}

	return displayComments(task, comments, opts.IO, opts.JSON)
}

// fetchComments retrieves all stories for a task (paging through results) and
// returns only the comment-type stories.
func fetchComments(client *asana.Client, task *asana.Task) ([]*asana.Story, error) {
	options := &asana.Options{
		Fields: []string{
			"text", "created_at", "created_by", "created_by.name",
			"resource_subtype", "is_edited", "is_pinned",
		},
		Limit: 100,
	}

	var stories []*asana.Story
	for {
		batch, nextPage, err := task.Stories(client, options)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch comments: %w", err)
		}

		stories = append(stories, batch...)

		if nextPage == nil || nextPage.Offset == "" {
			break
		}
		options.Offset = nextPage.Offset
	}

	return filterComments(stories), nil
}

// filterComments keeps only the comment-type stories from a list of stories.
func filterComments(stories []*asana.Story) []*asana.Story {
	var comments []*asana.Story
	for _, s := range stories {
		if s.ResourceSubtype == commentSubtype {
			comments = append(comments, s)
		}
	}
	return comments
}

func displayComments(task *asana.Task, comments []*asana.Story, io *iostreams.IOStreams, jsonOutput bool) error {
	if jsonOutput {
		return displayJSON(comments, io)
	}
	return displayText(task, comments, io)
}

func displayJSON(comments []*asana.Story, io *iostreams.IOStreams) error {
	type jsonComment struct {
		ID        string `json:"id"`
		Author    string `json:"author"`
		Text      string `json:"text"`
		CreatedAt string `json:"created_at,omitempty"`
		IsEdited  bool   `json:"is_edited,omitempty"`
		IsPinned  bool   `json:"is_pinned,omitempty"`
	}

	// Always emit an array (never null) for predictable scripting.
	out := make([]jsonComment, 0, len(comments))
	for _, c := range comments {
		jc := jsonComment{
			ID:       c.ID,
			Text:     c.Text,
			IsEdited: c.IsEdited,
			IsPinned: c.IsPinned,
		}
		if c.CreatedBy != nil {
			jc.Author = c.CreatedBy.Name
		}
		if c.CreatedAt != nil {
			jc.CreatedAt = c.CreatedAt.Format(time.RFC3339)
		}
		out = append(out, jc)
	}

	enc := json.NewEncoder(io.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func displayText(task *asana.Task, comments []*asana.Story, io *iostreams.IOStreams) error {
	cs := io.ColorScheme()

	if len(comments) == 0 {
		fmt.Fprintf(io.Out, "No comments on %s\n", cs.Bold(task.Name))
		return nil
	}

	fmt.Fprintf(io.Out, "%s\n", cs.Bold(fmt.Sprintf("Comments on %s (%d)", task.Name, len(comments))))

	for _, c := range comments {
		author := "Unknown"
		if c.CreatedBy != nil && c.CreatedBy.Name != "" {
			author = c.CreatedBy.Name
		}

		// Byline: author, timestamp, and edited/pinned markers.
		fmt.Fprintf(io.Out, "\n%s", cs.Bold(author))
		if c.CreatedAt != nil {
			fmt.Fprintf(io.Out, " %s", cs.Gray(c.CreatedAt.Format("Jan 02, 2006 3:04 PM")))
		}
		if c.IsEdited {
			fmt.Fprintf(io.Out, " %s", cs.Gray("(edited)"))
		}
		if c.IsPinned {
			fmt.Fprintf(io.Out, " %s", cs.Gray("(pinned)"))
		}
		fmt.Fprintln(io.Out)

		for _, line := range strings.Split(strings.TrimRight(c.Text, "\n"), "\n") {
			fmt.Fprintf(io.Out, "  %s\n", line)
		}
	}

	return nil
}
