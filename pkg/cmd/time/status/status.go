package status

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/pkg/cmdutils"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/format"
	"github.com/timwehrle/asana/pkg/iostreams"
)

type StatusOptions struct {
	cmdutils.BaseOptions

	JSON bool
}

func NewCmdStatus(f factory.Factory, runF func(*StatusOptions) error) *cobra.Command {
	opts := &StatusOptions{
		BaseOptions: cmdutils.BaseOptions{
			IO:       f.IOStreams,
			Prompter: f.Prompter,
			Config:   f.Config,
			Client:   f.Client,
		},
	}

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show tracked time for a task",
		Long: heredoc.Doc(`
				Display all time entries logged on a selected Asana task, grouped by date,
				along with the total tracked time.
			`),
		Example: heredoc.Doc(`
				# Show the tracked time of a selected task
				$ asana timer status

				# Output as JSON
				$ asana timer status --json
			`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF == nil {
				return runStatus(opts)
			}
			return runF(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output in JSON format")

	return cmd
}

type GroupedEntries struct {
	Date    time.Time
	Label   string
	Entries []*asana.TimeTrackingEntry
}

func runStatus(opts *StatusOptions) error {
	io := opts.IO

	client, err := opts.Client()
	if err != nil {
		return err
	}

	task, err := cmdutils.SelectTask(&opts.BaseOptions, client)
	if err != nil {
		return err
	}

	entries, _, err := task.GetTimeTrackingEntries(client, &asana.Options{
		Fields: []string{
			"created_by.name", "created_by.gid",
			"duration_minutes", "entered_on",
			"task.name", "task.gid",
			"description", "approval_status", "billable_status",
			"created_at",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to get time tracking entries: %w", err)
	}

	return renderOutput(opts, io, task, entries)
}

func renderOutput(opts *StatusOptions, io *iostreams.IOStreams, task *asana.Task, entries []*asana.TimeTrackingEntry) error {
	if opts.JSON {
		return renderJSON(io, task, entries)
	}
	return renderText(io, task, entries)
}

func renderJSON(io *iostreams.IOStreams, task *asana.Task, entries []*asana.TimeTrackingEntry) error {
	type jsonRef struct {
		ID   string `json:"id"`
		Name string `json:"name,omitempty"`
	}
	type jsonEntry struct {
		ID              string   `json:"id"`
		DurationMinutes int      `json:"duration_minutes"`
		EnteredOn       string   `json:"entered_on,omitempty"`
		CreatedBy       *jsonRef `json:"created_by"`
		Task            *jsonRef `json:"task"`
		Description     string   `json:"description,omitempty"`
		ApprovalStatus  string   `json:"approval_status,omitempty"`
		BillableStatus  string   `json:"billable_status,omitempty"`
		CreatedAt       string   `json:"created_at,omitempty"`
	}
	type jsonOutput struct {
		TaskName     string       `json:"task_name"`
		TotalMinutes int          `json:"total_minutes"`
		Entries      []*jsonEntry `json:"entries"`
	}

	total := 0
	jsonEntries := make([]*jsonEntry, 0, len(entries))

	for _, e := range entries {
		total += e.DurationMinutes

		je := &jsonEntry{
			ID:              e.ID,
			DurationMinutes: e.DurationMinutes,
			Description:     e.Description,
			ApprovalStatus:  e.ApprovalStatus,
			BillableStatus:  e.BillableStatus,
		}
		if e.EnteredOn != nil {
			je.EnteredOn = time.Time(*e.EnteredOn).Format(time.DateOnly)
		}
		if e.CreatedBy != nil {
			je.CreatedBy = &jsonRef{ID: e.CreatedBy.ID, Name: e.CreatedBy.Name}
		}
		if e.Task != nil {
			je.Task = &jsonRef{ID: e.Task.ID, Name: e.Task.Name}
		}
		if e.CreatedAt != nil {
			je.CreatedAt = e.CreatedAt.Format(time.RFC3339)
		}
		jsonEntries = append(jsonEntries, je)
	}

	out := jsonOutput{
		TaskName:     task.Name,
		TotalMinutes: total,
		Entries:      jsonEntries,
	}

	enc := json.NewEncoder(io.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func renderText(io *iostreams.IOStreams, task *asana.Task, entries []*asana.TimeTrackingEntry) error {
	cs := io.ColorScheme()

	if len(entries) == 0 {
		io.Println("No time entries found for this task.")
		return nil
	}

	groups, total, err := groupEntries(entries)
	if err != nil {
		return err
	}

	io.Printf("\nTime entries for task: %s\n", cs.Bold(task.Name))
	for _, g := range groups {
		io.Printf("\n[%s]\n", g.Label)
		for _, entry := range g.Entries {
			line := fmt.Sprintf(" • %s — %s",
				entry.CreatedBy.Name,
				cs.Bold(format.Duration(entry.DurationMinutes)),
			)
			if entry.Description != "" {
				line += fmt.Sprintf(" — %s", entry.Description)
			}
			io.Printf("%s\n", line)
		}
	}

	io.Printf("\nTotal: %s\n", cs.Bold(format.Duration(total)))
	return nil
}

func groupEntries(entries []*asana.TimeTrackingEntry) ([]GroupedEntries, int, error) {
	m := map[string]*GroupedEntries{}
	total := 0

	for _, e := range entries {
		if e.EnteredOn == nil {
			continue
		}

		key := time.Time(*e.EnteredOn).Format(time.DateOnly)
		t, err := time.Parse(time.DateOnly, key)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid entered_on date: %w", err)
		}

		if _, ok := m[key]; !ok {
			m[key] = &GroupedEntries{
				Date:  t,
				Label: format.HumanDate(t),
			}
		}

		m[key].Entries = append(m[key].Entries, e)
		total += e.DurationMinutes
	}

	groups := make([]GroupedEntries, 0, len(m))
	for _, g := range m {
		groups = append(groups, *g)
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Date.After(groups[j].Date)
	})

	return groups, total, nil
}
