package root

import (
	"os"

	"github.com/timwehrle/asana/pkg/cmd/teams"
	"github.com/timwehrle/asana/pkg/cmd/time"
	"github.com/timwehrle/asana/pkg/cmd/upgrade"

	"github.com/MakeNowJust/heredoc"
	"github.com/timwehrle/asana/pkg/cmd/tags"

	"github.com/spf13/cobra"
	service "github.com/timwehrle/asana/internal/auth"
	"github.com/timwehrle/asana/internal/build"
	"github.com/timwehrle/asana/pkg/cmd/auth"
	"github.com/timwehrle/asana/pkg/cmd/config"
	"github.com/timwehrle/asana/pkg/cmd/projects"
	"github.com/timwehrle/asana/pkg/cmd/tasks"
	"github.com/timwehrle/asana/pkg/cmd/users"
	"github.com/timwehrle/asana/pkg/cmd/workspaces"
	"github.com/timwehrle/asana/pkg/factory"
)

func NewCmdRoot(f factory.Factory, buildVersion string) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "asana <command> <subcommand> [flags]",
		Short: "The Asana CLI tool",
		Long:  `Work with Asana from the command line.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip all checks for auth and upgrade commands
			if isNoAuthCommand(os.Args) {
				return nil
			}

			// For non-auth commands, check authentication and load config
			err := service.Check()
			if err != nil {
				return err
			}

			return nil
		},
	}

	// Add auth command first
	cmd.AddCommand(auth.NewCmdAuth(f))

	// Only load config for non-auth/upgrade commands
	if !isNoAuthCommand(os.Args) {
		cfg, err := f.Config()
		if err != nil {
			return nil, err
		}

		err = cfg.Load()
		if err != nil {
			return nil, err
		}

		err = cfg.Set("build", build.Version)
		if err != nil {
			return nil, err
		}
	}

	// Add other commands
	cmd.AddCommand(tasks.NewCmdTasks(f))
	cmd.AddCommand(projects.NewCmdProjects(f))
	cmd.AddCommand(workspaces.NewCmdWorkspace(f))
	cmd.AddCommand(users.NewCmdUsers(f))
	cmd.AddCommand(config.NewCmdConfig(f))
	cmd.AddCommand(tags.NewCmdTags(f))
	cmd.AddCommand(teams.NewCmdTeams(f))
	cmd.AddCommand(time.NewCmdTimer(f))
	cmd.AddCommand(upgrade.NewCmdUpgrade(f, nil))

	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	cmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		showHelp(command, strings, os.Stdout)
	})

	cmd.SetUsageFunc(showRootUsage)

	cmd.Version = buildVersion
	cmd.SetVersionTemplate(heredoc.Doc(`
	asana build {{ .Version }}
	https://github.com/timwehrle/asana/releases/tag/{{ .Version }}
	`))

	return cmd, nil
}

// isNoAuthCommand reports whether an invocation must run without a token or a
// config file.
//
// `auth`, `upgrade` and `completion` manage the credential or the binary itself,
// so they cannot require a working one. Version and help are pure local output:
// an install script, a container health check and "what have I got installed"
// all reach for them first, and advice to run `asana upgrade` is useless if the
// user cannot read their current version. A bare `asana` prints help, so it
// belongs here too.
//
// The flag scan cannot distinguish `--help` as a request for help from `--help`
// as an argument value. Exempting either is the safe direction: the command
// still runs and fails on its own terms, whereas wrongly demanding auth would
// block a legitimate help request.
func isNoAuthCommand(args []string) bool {
	// A bare `asana`, which prints help.
	if len(args) < 2 {
		return true
	}

	switch args[1] {
	case "auth", "upgrade", "completion", "help":
		return true
	}

	for _, arg := range args[1:] {
		switch arg {
		case "--version", "-v", "--help", "-h":
			return true
		}
	}

	return false
}
