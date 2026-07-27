package projects

import (
	"github.com/jtsternberg/asana-cli/pkg/cmd/projects/list"
	"github.com/jtsternberg/asana-cli/pkg/cmd/projects/sections"
	"github.com/jtsternberg/asana-cli/pkg/cmd/projects/tasks"
	"github.com/jtsternberg/asana-cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdProjects(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projects <subcommand>",
		Short: "Manage your Asana projects",
		Long:  "Perform operations related to your Asana projects.",
	}

	cmd.AddCommand(list.NewCmdList(f, nil))
	cmd.AddCommand(sections.NewCmdSections(f, nil))
	cmd.AddCommand(tasks.NewCmdTasks(f, nil))

	return cmd
}
