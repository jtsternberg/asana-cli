package tags

import (
	"github.com/jtsternberg/asana-cli/pkg/cmd/tags/list"
	"github.com/jtsternberg/asana-cli/pkg/cmd/tags/tasks"
	"github.com/jtsternberg/asana-cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdTags(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tags <subcommand>",
		Short: "Manage your Asana tags",
		Long:  "Perform operations related to your Asana tags.",
	}

	cmd.AddCommand(list.NewCmdList(f, nil))
	cmd.AddCommand(tasks.NewCmdTasks(f, nil))

	return cmd
}
