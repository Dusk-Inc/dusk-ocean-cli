package cmd

import (
	functions "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/functions"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register an already-on-disk repo into the workspace",
	Long: `Register a repo that already exists at one of the allowed workspace
locations (repos/projects/<name>, repos/libs/<name>, repos/apps/<name>,
repos/apps/<app>/libs/<name>, or repos/apps/<app>/services/<name>) into
ocean.workspace.json. Writes a starter ocean.config.json at the repo
root if one does not already exist.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		kind, err := cmd.Flags().GetString("kind")
		if err != nil {
			return err
		}
		name, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}
		app, err := cmd.Flags().GetString("app")
		if err != nil {
			return err
		}
		remote, err := cmd.Flags().GetString("remote")
		if err != nil {
			return err
		}
		templateKind, err := cmd.Flags().GetString("template-kind")
		if err != nil {
			return err
		}
		return functions.RegisterRepo(afero.NewOsFs(), cmd.OutOrStdout(), kind, name, app, remote, templateKind)
	},
}

func init() {
	registerCmd.Flags().String("kind", "", "Repo kind (project, library, app, service, template)")
	registerCmd.Flags().String("name", "", "Repo name (also the directory basename)")
	registerCmd.Flags().String("app", "", "Parent app name (required for service; optional for library)")
	registerCmd.Flags().String("remote", "", "Upstream git URL; defaults to the literal \"None\" sentinel if omitted")
	registerCmd.Flags().String("template-kind", "", "When --kind=template, the kind it scaffolds (service, library, project)")
}
