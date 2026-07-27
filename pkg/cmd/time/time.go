package time

import (
	"github.com/jtsternberg/asana-cli/pkg/cmd/time/create"
	"github.com/jtsternberg/asana-cli/pkg/cmd/time/delete"
	"github.com/jtsternberg/asana-cli/pkg/cmd/time/status"
	"github.com/jtsternberg/asana-cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdTimer(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "time",
		Short: "Manage time tracking for your Asana tasks",
		Long:  "Commands to track, delete and inspect time entries on your Asana tasks.",
	}

	cmd.AddCommand(status.NewCmdStatus(f, nil), create.NewCmdCreate(f, nil), delete.NewCmdDelete(f, nil))

	return cmd
}
