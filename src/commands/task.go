package cmd

import (
	functions "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/functions"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Run a workspace-level task against a single repo",
	Long: `Resolve a workspace task by --name, expand its template against the
target repo's variable context (env, var, ocean, repo namespaces), and
execute the resulting shell command from the workspace root.

Iteration across multiple repos is intentionally not supported in this
release; the command runs once against the repo identified by --target
(plus --app for service or app-scoped library targets).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}
		target, err := cmd.Flags().GetString("target")
		if err != nil {
			return err
		}
		app, err := cmd.Flags().GetString("app")
		if err != nil {
			return err
		}
		return functions.RunWorkspaceTask(afero.NewOsFs(), cmd.OutOrStdout(), cmd.ErrOrStderr(), name, target, app)
	},
}

func init() {
	taskCmd.Flags().String("name", "", "Workspace task name (key in ocean.workspace.json#tasks)")
	taskCmd.Flags().String("target", "", "Repo name to run the task against")
	taskCmd.Flags().String("app", "", "Parent app name (required for service or app-scoped library targets)")
}
