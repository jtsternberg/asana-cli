package add

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/jtsternberg/asana-cli/internal/api/asana"
	"github.com/jtsternberg/asana-cli/pkg/cmdutils"
	"github.com/jtsternberg/asana-cli/pkg/factory"
	"github.com/jtsternberg/asana-cli/pkg/htmlnotes"
	"github.com/jtsternberg/asana-cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

type AddOptions struct {
	IO     *iostreams.IOStreams
	Client func() (*asana.Client, error)

	TaskID       string
	Text         string
	HTMLText     string
	MarkdownText string
	JSON         bool
	DryRun       bool

	// Accepted so agents that pass it to every write command don't trip on an
	// unknown flag; this command never prompts.
	NonInteractive bool
}

func NewCmdAdd(f factory.Factory, runF func(*AddOptions) error) *cobra.Command {
	opts := &AddOptions{
		IO:     f.IOStreams,
		Client: f.Client,
	}

	cmd := &cobra.Command{
		Use:   "add <task-id>",
		Short: "Add a comment to a task",
		Long: heredoc.Doc(`
			Post a comment on a task.

			The comment is given in one of three mutually exclusive forms:
			--text for plain text, --markdown-text for Markdown, or --html-text
			for Asana-flavored HTML. Markdown is converted to rich text, so
			links, lists and emphasis work.

			Each form accepts the same locations as the --*-notes flags on
			"tasks create" and "tasks update": @path reads a file, - reads
			stdin, and @@ escapes a literal leading @ (so "@@Tom" posts "@Tom").
			Use a file or stdin for multi-line text.

			--html-text follows the same rules as --html-notes: only body
			strong em u s code ol ul li a blockquote pre h1 h2 hr img, with
			attributes only on <a> and <img>, checked locally before any
			request. <a data-asana-gid="123"/> becomes an @-mention.

			This command never prompts; --non-interactive is accepted and
			changes nothing.
		`),
		Args: cobra.ExactArgs(1),
		Example: heredoc.Doc(`
			# Plain text
			$ asana tasks comments add 1234567890 --text "Ready when you are."

			# Multi-line text from a file or stdin
			$ asana tasks comments add 1234567890 --text @comment.txt
			$ generate-update | asana tasks comments add 1234567890 --text -

			# Rich text
			$ asana tasks comments add 1234567890 --markdown-text "See [the PR](https://example.com)"

			# Rehearse: print the request, post nothing
			$ asana tasks comments add 1234567890 --text @comment.txt --dry-run

			# Machine-readable receipt
			$ asana tasks comments add 1234567890 --text "Done" --json
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.TaskID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return runAdd(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Text, "text", "m", "", "Comment as plain text; pass @file to read a file or - for stdin")
	cmd.Flags().StringVar(&opts.HTMLText, "html-text", "", "Comment as Asana-flavored HTML; pass @file to read a file or - for stdin")
	cmd.Flags().StringVar(&opts.MarkdownText, "markdown-text", "", "Comment as Markdown, converted to rich text; pass @file to read a file or - for stdin")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output the posted comment as JSON")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Print the request without posting the comment")
	cmd.Flags().BoolVar(&opts.NonInteractive, "non-interactive", false, "Accepted for consistency; this command never prompts")

	cmd.MarkFlagsMutuallyExclusive("text", "html-text", "markdown-text")

	return cmd
}

// buildStory resolves whichever text flag was given into the request body. It
// needs no network access, so bad input never costs a round trip.
func buildStory(opts *AddOptions) (*asana.StoryBase, error) {
	var value string
	switch {
	case opts.Text != "":
		value = opts.Text
	case opts.MarkdownText != "":
		value = opts.MarkdownText
	case opts.HTMLText != "":
		value = opts.HTMLText
	default:
		return nil, fmt.Errorf("no comment given; pass --text, --markdown-text or --html-text")
	}

	raw, err := htmlnotes.ResolveLabeled(value, "comment text", opts.IO.In)
	if err != nil {
		return nil, err
	}
	// Files and heredocs end in a newline nobody means to post.
	raw = strings.TrimRight(raw, " \t\r\n")
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("comment text is empty")
	}

	switch {
	case opts.Text != "":
		return &asana.StoryBase{Text: raw}, nil
	case opts.MarkdownText != "":
		html, err := htmlnotes.FromMarkdown(raw)
		if err != nil {
			return nil, err
		}
		return &asana.StoryBase{HTMLText: html}, nil
	default:
		html, err := htmlnotes.Normalize(raw)
		if err != nil {
			return nil, err
		}
		return &asana.StoryBase{HTMLText: html}, nil
	}
}

func runAdd(opts *AddOptions) error {
	story, err := buildStory(opts)
	if err != nil {
		return err
	}

	client, err := opts.Client()
	if err != nil {
		return fmt.Errorf("failed to initialize Asana client: %w", err)
	}

	// Fetched on dry runs too: a rehearsal that cannot find the task should fail
	// the same way the real post would.
	task := &asana.Task{ID: opts.TaskID}
	if err := task.Fetch(client); err != nil {
		return fmt.Errorf("task %q not found: %w", opts.TaskID, err)
	}

	if opts.DryRun {
		if err := cmdutils.PrintDryRun(opts.IO, fmt.Sprintf("POST /tasks/%s/stories", opts.TaskID), story); err != nil {
			return err
		}
		opts.IO.Printf("  %s %s\n", opts.IO.ColorScheme().Gray("Task:"), task.Name)
		return nil
	}

	created, err := task.CreateComment(client, story)
	if err != nil {
		return fmt.Errorf("failed to add comment: %w", err)
	}

	return displayReceipt(opts, task, created)
}

type jsonComment struct {
	ID        string `json:"id"`
	TaskID    string `json:"task_id"`
	Author    string `json:"author,omitempty"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at,omitempty"`
	TaskURL   string `json:"task_url,omitempty"`
}

// displayReceipt names the task's URL rather than the comment's: Asana's story
// record carries no permalink of its own.
func displayReceipt(opts *AddOptions, task *asana.Task, story *asana.Story) error {
	if opts.JSON {
		jc := jsonComment{
			ID:      story.ID,
			TaskID:  task.ID,
			Text:    story.Text,
			TaskURL: task.PermalinkURL,
		}
		if story.CreatedBy != nil {
			jc.Author = story.CreatedBy.Name
		}
		if story.CreatedAt != nil {
			jc.CreatedAt = story.CreatedAt.Format(time.RFC3339)
		}
		enc := json.NewEncoder(opts.IO.Out)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		return enc.Encode(jc)
	}

	cs := opts.IO.ColorScheme()
	opts.IO.Printf("%s Added comment %s to task %s\n", cs.SuccessIcon, story.ID, cs.Bold(task.Name))
	if task.PermalinkURL != "" {
		opts.IO.Printf("  %s %s\n", cs.Gray("URL:"), task.PermalinkURL)
	}
	return nil
}
