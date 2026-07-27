package tasks

import (
	"github.com/jtsternberg/asana-cli/pkg/cmd/tasks/comments"
	"github.com/jtsternberg/asana-cli/pkg/cmd/tasks/create"
	"github.com/jtsternberg/asana-cli/pkg/cmd/tasks/delete"
	"github.com/jtsternberg/asana-cli/pkg/cmd/tasks/list"
	"github.com/jtsternberg/asana-cli/pkg/cmd/tasks/move"
	"github.com/jtsternberg/asana-cli/pkg/cmd/tasks/search"
	"github.com/jtsternberg/asana-cli/pkg/cmd/tasks/update"
	"github.com/jtsternberg/asana-cli/pkg/cmd/tasks/view"
	"github.com/jtsternberg/asana-cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdTasks(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tasks <subcommand>",
		Aliases: []string{"ts"},
		Short:   "Manage your Asana tasks",
		Long:    "Perform operations related to your Asana tasks.",
	}

	cmd.AddCommand(list.NewCmdList(f, nil))
	cmd.AddCommand(view.NewCmdView(f, nil))
	cmd.AddCommand(comments.NewCmdComments(f, nil))
	cmd.AddCommand(update.NewCmdUpdate(f, nil))
	cmd.AddCommand(search.NewCmdSearch(f, nil))
	cmd.AddCommand(create.NewCmdCreate(f, nil))
	cmd.AddCommand(delete.NewCmdDelete(f, nil))
	cmd.AddCommand(move.NewCmdMove(f, nil))

	return cmd
}
