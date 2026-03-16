package cmd

import (
	"fmt"

	functions "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/functions"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:   "rename",
	Short: "Rename a repository and update all workspace references",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := cmd.Flags().GetString("repo")
		if err != nil {
			return err
		}
		newName, err := cmd.Flags().GetString("new-name")
		if err != nil {
			return err
		}
		inApp, err := cmd.Flags().GetString("in")
		if err != nil {
			return err
		}
		if repo == "" || newName == "" {
			return fmt.Errorf("--repo and --new-name are required")
		}
		if inApp != "" {
			return functions.RenameRepo(cmd, afero.NewOsFs(), repo, newName, inApp)
		}
		return functions.RenameRepo(cmd, afero.NewOsFs(), repo, newName)
	},
}

func init() {
	renameCmd.Flags().String("repo", "", "Current repository name")
	renameCmd.Flags().String("new-name", "", "New repository name")
	renameCmd.Flags().String("in", "", "App name (required when renaming a service or app library)")
}
