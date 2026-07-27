package auth

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/jtsternberg/asana-cli/pkg/cmd/auth/login"
	"github.com/jtsternberg/asana-cli/pkg/cmd/auth/logout"
	"github.com/jtsternberg/asana-cli/pkg/cmd/auth/status"
	"github.com/jtsternberg/asana-cli/pkg/cmd/auth/update"
	"github.com/jtsternberg/asana-cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdAuth(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth <subcommand>",
		Short: "Authenticate with Asana",
		Long: heredoc.Doc(`
			Manage authentication for the Asana CLI, including login
			logout and checking authentication status.`),
	}

	cmd.AddCommand(status.NewCmdStatus(f, nil))
	cmd.AddCommand(login.NewCmdLogin(f, nil))
	cmd.AddCommand(logout.NewCmdLogout(f, nil))
	cmd.AddCommand(update.NewCmdUpdate(f, nil))

	return cmd
}
