package users

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/jtsternberg/asana-cli/pkg/cmd/users/list"
	"github.com/jtsternberg/asana-cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdUsers(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users <command>",
		Short: "Manage users of your Asana workspace",
		Long: heredoc.Doc(`
				Manage and interact with users in your Asana workspace.

				This command provides functionality to list all users visible within your permission scope,
				allowing you to better understand and organize your workspace members.
			`),
	}

	cmd.AddCommand(list.NewCmdList(f, nil))

	return cmd
}
