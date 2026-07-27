package login

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jtsternberg/asana-cli/internal/prompter"

	"github.com/jtsternberg/asana-cli/internal/api/asana"
	"github.com/jtsternberg/asana-cli/internal/config"
	"github.com/jtsternberg/asana-cli/pkg/cmdutils"
	"github.com/jtsternberg/asana-cli/pkg/factory"
	"github.com/jtsternberg/asana-cli/pkg/iostreams"

	"github.com/MakeNowJust/heredoc"
	"github.com/jtsternberg/asana-cli/internal/auth"
	"github.com/spf13/cobra"
)

type LoginOptions struct {
	IO       *iostreams.IOStreams
	Prompter prompter.Prompter

	Config func() (*config.Config, error)
	Client func() (*asana.Client, error)

	Workspace   string
	Token       string
	Interactive bool
}

func NewCmdLogin(f factory.Factory, runF func(*LoginOptions) error) *cobra.Command {
	opts := &LoginOptions{
		IO:       f.IOStreams,
		Config:   f.Config,
		Client:   f.Client,
		Prompter: f.Prompter,
	}

	var tokenStdin bool

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to your Asana account",
		Long: heredoc.Docf(`
				Authenticate with Asana using a Personal Access Token (PAT).
				
				To get started:
				1. Visit https://app.asana.com/0/my-apps
				2. Click "Create new token"
				3. Give your token a description (e.g., "CLI Access")
				4. Copy the generated token

				The token is stored in your OS keyring. $%[1]s (or its alias
				$%[2]s) takes precedence over the keyring when set, which is
				how unattended jobs and machines without a keyring
				authenticate.`, auth.EnvVarToken, auth.EnvVarPAT),
		Example: heredoc.Doc(`
					# Log in interactively and select a workspace
					$ asana auth login
					
					# Log in with a default workspace
					$ asana auth login --workspace "Test Workspace"
					
					# Log in with a token and set a default workspace
					$ asana auth login --workspace "Test Workspace" --with-token < mytoken.txt`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if tokenStdin {
				if opts.Workspace == "" {
					return fmt.Errorf(
						"workspace must be specified with --workspace when using --with-token",
					)
				}
				defer opts.IO.In.Close()
				token, err := io.ReadAll(opts.IO.In)
				if err != nil {
					return fmt.Errorf("failed to read token from standard input: %w", err)
				}
				opts.Token = strings.TrimSpace(string(token))
			}

			if opts.Token == "" {
				opts.Interactive = true
			}

			if runF != nil {
				return runF(opts)
			}

			return runLogin(opts)
		},
	}

	cmd.Flags().
		StringVarP(&opts.Workspace, "workspace", "w", "", "The default workspace to make calls to")
	cmd.Flags().BoolVar(&tokenStdin, "with-token", false, "Read token from standard input")

	return cmd
}

// credentialWriter is the pair of side effects a completed login leaves behind:
// the token in the keyring and the config on disk, plus the means to undo the
// first. It is a struct of functions so persistLogin's ordering and rollback can
// be tested without a real keyring or a real home directory.
type credentialWriter struct {
	setToken    func(string) error
	saveConfig  func() error
	deleteToken func() (bool, error)
}

func defaultCredentialWriter(cfg *config.Config) credentialWriter {
	return credentialWriter{
		setToken:    auth.Set,
		saveConfig:  cfg.Save,
		deleteToken: auth.DeleteStored,
	}
}

// persistLogin stores the token first and the config second.
//
// The other order leaves a config on disk with nothing behind it whenever the
// keyring write fails — a locked keyring, no Secret Service, or the 3s timeout in
// internal/auth. That is the one half-state nothing recovers from: `auth status`
// reads the config and reports a user and a workspace while every command fails
// to authenticate, and `auth login` finds no stored token so it starts over
// instead of naming the problem.
//
// Failing the other way is already handled: runLogin's opening check loads the
// config after finding a stored token and clears the token when that load fails.
// Should the config write fail anyway, the token is rolled back, so a failed
// login leaves nothing at all rather than half of a login.
func persistLogin(token string, w credentialWriter) error {
	if err := w.setToken(token); err != nil {
		return err
	}

	if err := w.saveConfig(); err != nil {
		if _, delErr := w.deleteToken(); delErr != nil {
			// Neither failure may be hidden: the token is still in the keyring
			// and the user is the only one who can clear it.
			return fmt.Errorf("%w (the stored token could not be rolled back: %v)", err, delErr)
		}
		return err
	}

	return nil
}

func runLogin(opts *LoginOptions) error {
	cs := opts.IO.ColorScheme()

	// An environment override wins over whatever we are about to store, so say so
	// before asking for a token rather than after.
	cmdutils.WarnEnvTokenBeforeStore(opts.IO)

	// Deliberately the keyring view, not the effective one: an env token must not
	// make this look "already logged in" and skip storing a credential.
	var token string
	token, err := auth.GetStored()
	if err == nil && token != "" {
		cfg := &config.Config{}
		if err := cfg.Load(); err != nil {
			if err := auth.Delete(); err != nil {
				return fmt.Errorf("failed to clear existing token: %w", err)
			}
		} else {
			fmt.Fprintln(opts.IO.Out, "You are already logged in")
			return nil
		}
	}

	if opts.Interactive {
		fmt.Fprint(opts.IO.Out, heredoc.Doc(`
		Tip: You can generate a Personal Access Token here: https://app.asana.com/0/my-apps
	`))
		token, err = opts.Prompter.Token()
		if err != nil {
			return err
		}
	} else {
		token = opts.Token
	}

	err = auth.ValidateToken(token)
	if err != nil {
		return err
	}

	client := asana.NewClientWithAccessToken(token)

	user, err := client.CurrentUser()
	if err != nil {
		return err
	}

	workspaces, err := client.AllWorkspaces()
	if err != nil {
		return err
	}

	if len(workspaces) == 0 {
		fmt.Fprintln(opts.IO.Out, "No workspaces found")
		return nil
	}

	var selectedWorkspace *asana.Workspace
	if opts.Workspace != "" {
		for _, ws := range workspaces {
			if ws.ID == opts.Workspace || strings.EqualFold(ws.Name, opts.Workspace) {
				selectedWorkspace = ws
				break
			}
		}

		if selectedWorkspace == nil {
			if !opts.Interactive {
				return fmt.Errorf(
					"%s Workspace '%s' not found. Please specify a valid workspace with --workspace",
					cs.ErrorIcon,
					opts.Workspace,
				)
			}

			fmt.Fprintf(
				opts.IO.ErrOut,
				"%s Workspace '%s' not found. Please select one from the list.\n",
				cs.ErrorIcon,
				opts.Workspace,
			)
		}
	}

	if selectedWorkspace == nil && opts.Interactive {
		names := make([]string, len(workspaces))
		for i, ws := range workspaces {
			names[i] = ws.Name
		}

		index, err := opts.Prompter.Select("Select a default workspace:", names)
		if err != nil {
			return err
		}

		selectedWorkspace = workspaces[index]
	}

	configDir := filepath.Join(os.Getenv("HOME"), ".config", "asana-cli")
	if err := os.MkdirAll(configDir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	cfg := &config.Config{
		Username:  user.Name,
		UserID:    user.ID,
		Workspace: selectedWorkspace,
	}

	if err := persistLogin(token, defaultCredentialWriter(cfg)); err != nil {
		return err
	}

	fmt.Fprintln(opts.IO.Out, cs.SuccessIcon, "Logged in")
	if selectedWorkspace != nil {
		fmt.Fprintf(
			opts.IO.Out,
			"%s Default workspace set to %s\n",
			cs.SuccessIcon,
			cs.Bold(selectedWorkspace.Name),
		)
	}

	return nil
}
