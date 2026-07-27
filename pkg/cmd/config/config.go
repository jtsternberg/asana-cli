package config

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/jtsternberg/asana-cli/pkg/cmd/config/get"
	"github.com/jtsternberg/asana-cli/pkg/cmd/config/set"
	"github.com/jtsternberg/asana-cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdConfig(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config <subcommand>",
		Short: "Manage Asana CLI configuration",
		Long: heredoc.Doc(`
				Set and retrieve configuration settings for the Asana CLI tool.
		`),
	}

	cmd.AddCommand(set.NewCmdConfigSet(f, nil))
	cmd.AddCommand(get.NewCmdGet(f, nil))

	return cmd
}
