package add

import (
	"encoding/json"
	stdio "io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/h2non/gock"
	"github.com/jtsternberg/asana-cli/internal/api/asana"
	"github.com/jtsternberg/asana-cli/pkg/factory"
	"github.com/jtsternberg/asana-cli/pkg/iostreams"
)

type obj map[string]any

func TestNewCmdAdd_ParsesTaskIDAndFlags(t *testing.T) {
	f, _, _ := factory.NewTestFactory()

	var saw *AddOptions
	cmd := NewCmdAdd(f, func(opts *AddOptions) error {
		saw = opts
		return nil
	})
	cmd.SetArgs([]string{"123", "--text", "hello", "--dry-run", "--json", "--non-interactive"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if saw.TaskID != "123" || saw.Text != "hello" || !saw.DryRun || !saw.JSON {
		t.Errorf("parsed opts = %+v", saw)
	}
}

func TestNewCmdAdd_RequiresTaskID(t *testing.T) {
	f, _, _ := factory.NewTestFactory()
	cmd := NewCmdAdd(f, func(opts *AddOptions) error { return nil })
	cmd.SetOut(stdio.Discard)
	cmd.SetErr(stdio.Discard)
	cmd.SetArgs([]string{"--text", "hello"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected an error without a task ID")
	}
}

func TestNewCmdAdd_TextFlagsAreMutuallyExclusive(t *testing.T) {
	pairs := [][]string{
		{"--text", "plain", "--html-text", "<body>x</body>"},
		{"--text", "plain", "--markdown-text", "x"},
		{"--html-text", "<body>x</body>", "--markdown-text", "x"},
	}

	for _, pair := range pairs {
		f, _, _ := factory.NewTestFactory()
		cmd := NewCmdAdd(f, func(opts *AddOptions) error { return nil })
		cmd.SetOut(stdio.Discard)
		cmd.SetErr(stdio.Discard)
		cmd.SetArgs(append([]string{"123"}, pair...))

		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "none of the others can be") {
			t.Fatalf("%v: expected a mutual-exclusion error, got %v", pair, err)
		}
	}
}

func TestBuildStory(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "comment.txt")
	if err := os.WriteFile(file, []byte("line one\nline two\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		opts     AddOptions
		stdin    string
		wantText string
		wantHTML string
		wantErr  string
	}{
		{
			name:     "plain text verbatim",
			opts:     AddOptions{Text: "Ready when you are."},
			wantText: "Ready when you are.",
		},
		{
			name:     "plain text from a file, trailing newline dropped",
			opts:     AddOptions{Text: "@" + file},
			wantText: "line one\nline two",
		},
		{
			name:     "plain text from stdin",
			opts:     AddOptions{Text: "-"},
			stdin:    "from a pipe\n\n",
			wantText: "from a pipe",
		},
		{
			name:     "double @ escapes a literal leading @",
			opts:     AddOptions{Text: "@@Tom can you look?"},
			wantText: "@Tom can you look?",
		},
		{
			name:     "markdown becomes html_text",
			opts:     AddOptions{MarkdownText: "See **this**"},
			wantHTML: "<body>See <strong>this</strong></body>",
		},
		{
			name:     "html fragment is wrapped in body",
			opts:     AddOptions{HTMLText: "<em>hi</em>"},
			wantHTML: "<body><em>hi</em></body>",
		},
		{
			name:    "html outside Asana's rules is refused",
			opts:    AddOptions{HTMLText: "<body><p>nope</p></body>"},
			wantErr: "<p> is not allowed",
		},
		{
			name:    "no text at all",
			opts:    AddOptions{},
			wantErr: "--text, --markdown-text or --html-text",
		},
		{
			name:    "whitespace-only text",
			opts:    AddOptions{Text: "  \n "},
			wantErr: "comment text is empty",
		},
		{
			name:    "empty file",
			opts:    AddOptions{Text: "-"},
			stdin:   "",
			wantErr: "comment text is empty",
		},
		{
			name:    "missing file names the comment, not notes",
			opts:    AddOptions{Text: "@" + filepath.Join(dir, "nope.txt")},
			wantErr: "failed to read comment text from",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			io, in, _, _ := iostreams.Test()
			in.WriteString(tt.stdin)
			opts := tt.opts
			opts.IO = io

			story, err := buildStory(&opts)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("buildStory() error = %v; want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("buildStory() error = %v", err)
			}
			if story.Text != tt.wantText {
				t.Errorf("Text = %q; want %q", story.Text, tt.wantText)
			}
			if story.HTMLText != tt.wantHTML {
				t.Errorf("HTMLText = %q; want %q", story.HTMLText, tt.wantHTML)
			}
		})
	}
}

func mockTask() {
	gock.New("https://app.asana.com").
		Get("/api/1.0/tasks/T1").
		Reply(200).
		JSON(obj{"data": obj{
			"gid":           "T1",
			"name":          "Runner token",
			"permalink_url": "https://app.asana.com/0/0/T1",
		}})
}

func newOpts(io *iostreams.IOStreams) *AddOptions {
	return &AddOptions{
		IO:     io,
		TaskID: "T1",
		Client: func() (*asana.Client, error) { return asana.NewClient(http.DefaultClient), nil },
	}
}

// A dry run must reach Asana only to read the task: gock rejects any request
// it has no mock for, so an attempted POST fails the run.
func TestRunAdd_DryRunPrintsRequestAndSendsNothing(t *testing.T) {
	defer gock.Off()
	mockTask()

	io, _, out, _ := iostreams.Test()
	opts := newOpts(io)
	opts.Text = "Ready now & thanks"
	opts.DryRun = true

	if err := runAdd(opts); err != nil {
		t.Fatalf("runAdd() error = %v", err)
	}

	got := out.String()
	for _, want := range []string{"no request was made", "POST /tasks/T1/stories", `"text": "Ready now & thanks"`} {
		if !strings.Contains(got, want) {
			t.Errorf("dry-run output missing %q:\n%s", want, got)
		}
	}
	if !gock.IsDone() {
		t.Error("expected the task to be fetched")
	}
}

func TestRunAdd_PostsCommentAndPrintsReceipt(t *testing.T) {
	defer gock.Off()
	mockTask()

	var sentBody []byte
	gock.Observe(func(req *http.Request, _ gock.Mock) {
		if req.Method == http.MethodPost && req.Body != nil {
			sentBody, _ = stdio.ReadAll(req.Body)
		}
	})
	defer gock.Observe(nil)

	gock.New("https://app.asana.com").
		Post("/api/1.0/tasks/T1/stories").
		Reply(201).
		JSON(obj{"data": obj{
			"gid":              "S9",
			"text":             "multi\nline",
			"resource_subtype": "comment_added",
			"created_at":       "2026-11-02T10:00:00Z",
			"created_by":       obj{"gid": "U1", "name": "Justin Sternberg"},
		}})

	io, _, out, _ := iostreams.Test()
	opts := newOpts(io)
	opts.Text = "multi\nline"

	if err := runAdd(opts); err != nil {
		t.Fatalf("runAdd() error = %v", err)
	}
	if !gock.IsDone() {
		t.Fatal("expected the comment to be posted")
	}

	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(sentBody, &body); err != nil {
		t.Fatalf("request body is not JSON: %v\n%s", err, sentBody)
	}
	if body.Data["text"] != "multi\nline" {
		t.Errorf("sent text = %v", body.Data["text"])
	}
	if _, ok := body.Data["html_text"]; ok {
		t.Errorf("plain text must not send html_text: %s", sentBody)
	}

	got := out.String()
	for _, want := range []string{"S9", "Runner token", "https://app.asana.com/0/0/T1"} {
		if !strings.Contains(got, want) {
			t.Errorf("receipt missing %q:\n%s", want, got)
		}
	}
}

func TestRunAdd_InvalidTextFailsBeforeAnyRequest(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	opts := &AddOptions{
		IO:       io,
		TaskID:   "T1",
		HTMLText: "<body><p>nope</p></body>",
		Client: func() (*asana.Client, error) {
			t.Fatal("client should not be built when the text is invalid")
			return nil, nil
		},
	}

	if err := runAdd(opts); err == nil {
		t.Fatal("expected an error for invalid html")
	}
}

func TestDisplayReceipt_JSON(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	opts := &AddOptions{IO: io, JSON: true}

	task := &asana.Task{ID: "T1", PermalinkURL: "https://app.asana.com/0/0/T1"}
	task.Name = "Runner token"
	story := &asana.Story{ID: "S9", CreatedBy: &asana.User{ID: "U1", Name: "Justin Sternberg"}}
	story.Text = "hello"

	if err := displayReceipt(opts, task, story); err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	want := map[string]any{
		"id":       "S9",
		"task_id":  "T1",
		"text":     "hello",
		"author":   "Justin Sternberg",
		"task_url": "https://app.asana.com/0/0/T1",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v; want %v", k, got[k], v)
		}
	}
}
