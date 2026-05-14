package cmd

import (
	functions "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/functions"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Install, build, and test the workspace dependency graph",
	RunE: func(cmd *cobra.Command, args []string) error {
		clearHashes, err := cmd.Flags().GetBool("clear-hashes")
		if err != nil {
			return err
		}
		fs := afero.NewOsFs()
		root, err := functions.GetRoot()
		if err != nil {
			return err
		}

		if clearHashes {
			if err := functions.ClearHashes(fs, cmd, root); err != nil {
				return err
			}
		}

		config, err := functions.ReadWorkspaceConfig(fs)
		if err != nil {
			return err
		}
		if err := functions.RunRefresh(cmd, fs, root, config); err != nil {
			return err
		}
		return functions.CleanupStaleHashes(fs, cmd, root)
	},
}

func init() {
	refreshCmd.Flags().Bool("clear-hashes", false, "Remove all build/check hashes")
}
