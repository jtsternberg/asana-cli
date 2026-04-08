package create

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/pkg/cmdutils"
	"github.com/timwehrle/asana/pkg/convert"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/format"
	"github.com/timwehrle/asana/pkg/iostreams"
)

type CreateOptions struct {
	cmdutils.BaseOptions

	Minutes int
	DateStr string
	Date    *asana.Date
	JSON    bool
}

func NewCmdCreate(f factory.Factory, runF func(*CreateOptions) error) *cobra.Command {
	opts := &CreateOptions{
		BaseOptions: cmdutils.BaseOptions{
			IO:       f.IOStreams,
			Prompter: f.Prompter,
			Config:   f.Config,
			Client:   f.Client,
		},
	}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Log time to a task",
		Long:  "Record a new time entry on a selected Asana task.",
		Example: heredoc.Doc(`
			# Log time via flags
			asana time create --minutes 30 --date 2025-01-06

			# Log time interactively
			asana time create --date 2025-01-06
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF == nil {
				return runCreate(opts)
			}

			return runF(opts)
		},
	}

	cmd.Flags().IntVarP(&opts.Minutes, "minutes", "m", 0, "Minutes to log (prompted if not set)")
	cmd.Flags().StringVar(&opts.DateStr, "date", "", "Entry date (YYYY-MM-DD, defaults to today)")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output in JSON format")

	return cmd
}

func (o *CreateOptions) Validate() error {
	if o.Minutes < 0 {
		return fmt.Errorf("minutes must be zero or a positive integer")
	}

	if o.DateStr != "" {
		date, err := convert.ToDate(o.DateStr, time.DateOnly)
		if err != nil {
			return fmt.Errorf("invalid date: %w", err)
		}
		o.Date = date
	} else {
		today := asana.Date(time.Now())
		o.Date = &today
	}
	return nil
}

func runCreate(opts *CreateOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	client, err := opts.Client()
	if err != nil {
		return err
	}

	task, err := cmdutils.SelectTask(&opts.BaseOptions, client)
	if err != nil {
		return err
	}

	var minutes int
	if opts.Minutes > 0 {
		minutes = opts.Minutes
	} else {
		minutes, err = promptDuration(opts)
		if err != nil {
			return err
		}
	}

	result, err := task.CreateTimeTrackingEntry(client, &asana.CreateTimeTrackingEntryRequest{
		DurationMinutes: minutes,
		EnteredOn:       opts.Date,
	})
	if err != nil {
		return fmt.Errorf("failed to create time tracking entry: %w", err)
	}

	return renderCreateResult(opts.IO, result, task.Name, opts.JSON)
}

func renderCreateResult(io *iostreams.IOStreams, result *asana.TimeTrackingEntry, taskName string, jsonOutput bool) error {
	if jsonOutput {
		return renderCreateJSON(io, result, taskName)
	}

	io.Printf("%s Logged %s to %q on %s\n",
		io.ColorScheme().SuccessIcon,
		format.Duration(result.DurationMinutes),
		taskName,
		format.Date(result.EnteredOn),
	)
	return nil
}

func renderCreateJSON(io *iostreams.IOStreams, entry *asana.TimeTrackingEntry, taskName string) error {
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

	je := &jsonEntry{
		ID:              entry.ID,
		DurationMinutes: entry.DurationMinutes,
		Description:     entry.Description,
		ApprovalStatus:  entry.ApprovalStatus,
		BillableStatus:  entry.BillableStatus,
	}
	if entry.EnteredOn != nil {
		je.EnteredOn = time.Time(*entry.EnteredOn).Format(time.DateOnly)
	}
	if entry.CreatedBy != nil {
		je.CreatedBy = &jsonRef{ID: entry.CreatedBy.ID, Name: entry.CreatedBy.Name}
	}
	if entry.Task != nil {
		je.Task = &jsonRef{ID: entry.Task.ID, Name: entry.Task.Name}
	} else {
		// The create response may not include the task object,
		// but we know the task name from context.
		je.Task = &jsonRef{Name: taskName}
	}
	if entry.CreatedAt != nil {
		je.CreatedAt = entry.CreatedAt.Format(time.RFC3339)
	}

	enc := json.NewEncoder(io.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(je)
}

func promptDuration(opts *CreateOptions) (int, error) {
	minutesStr, err := opts.Prompter.Input("Enter minutes to log (e.g., 30):", "")
	if err != nil {
		return 0, fmt.Errorf("failed to read duration: %w", err)
	}

	minutes, err := strconv.Atoi(minutesStr)
	if err != nil || minutes <= 0 {
		return 0, fmt.Errorf("invalid input: please enter a positive number")
	}
	return minutes, nil
}
