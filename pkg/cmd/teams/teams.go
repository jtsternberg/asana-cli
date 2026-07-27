package teams

import (
	"github.com/jtsternberg/asana-cli/pkg/cmd/teams/list"
	"github.com/jtsternberg/asana-cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdTeams(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "teams <subcommand>",
		Short: "Manage your Asana teams",
		Long:  "Perform operations related to your Asana teams.",
	}

	cmd.AddCommand(list.NewCmdList(f, nil))

	return cmd
}
