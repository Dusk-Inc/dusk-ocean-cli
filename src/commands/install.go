package cmd

import (
	functions "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/functions"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Add a local dependency into a target repository, or run a repo's install task with --library",
	RunE: func(cmd *cobra.Command, args []string) error {
		library, err := cmd.Flags().GetString("library")
		if err != nil {
			return err
		}
		if library != "" {
			return functions.RunInstall(cmd, afero.NewOsFs(), library)
		}
		return functions.RunInstallPrompt(cmd, afero.NewOsFs())
	},
}

func init() {
	installCmd.Flags().String("library", "", "Name of the repo to run the install task for")
}
