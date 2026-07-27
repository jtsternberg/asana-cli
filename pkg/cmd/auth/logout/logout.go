package logout

import (
	"fmt"

	"github.com/timwehrle/asana/pkg/cmdutils"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/iostreams"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/timwehrle/asana/internal/auth"
)

type LogoutOptions struct {
	IO *iostreams.IOStreams
}

func NewCmdLogout(f factory.Factory, runF func(options *LogoutOptions) error) *cobra.Command {
	opts := &LogoutOptions{
		IO: f.IOStreams,
	}

	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Log out of your Asana account",
		Long: heredoc.Docf(`
				Log out of your current Asana account by removing locally
				stored credentials.

				This action revokes CLI access to the Asana API.

				Only the keyring entry is removed. A token in $%[1]s or
				$%[2]s takes precedence over the keyring, so the CLI stays
				authenticated while one of those is set.`, auth.EnvVarToken, auth.EnvVarPAT),
		Example: heredoc.Doc(`$ asana auth logout`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF != nil {
				return runF(opts)
			}

			return runLogout(opts)
		},
	}

	return cmd
}

func runLogout(opts *LogoutOptions) error {
	cs := opts.IO.ColorScheme()

	err := auth.Check()
	if err != nil {
		return err
	}

	removed, err := auth.DeleteStored()
	if err != nil {
		return err
	}

	if removed {
		fmt.Fprintln(opts.IO.Out, cs.SuccessIcon, "Logged out")
	} else {
		fmt.Fprintln(opts.IO.Out, cs.WarningIcon, "No stored token to remove")
	}

	cmdutils.WarnEnvTokenAfterLogout(opts.IO)

	return nil
}
