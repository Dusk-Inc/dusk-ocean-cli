package cmd

import (
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/hash"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/refresh"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/tree"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"
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
		root, err := tree.GetRoot()
		if err != nil {
			return err
		}

		if clearHashes {
			if err := hash.ClearHashes(fs, cmd, root); err != nil {
				return err
			}
		}

		if err := workspace.ValidateComposeConsistency(fs, root); err != nil {
			return err
		}

		config, err := workspace.ReadWorkspaceConfig(fs)
		if err != nil {
			return err
		}
		if err := refresh.RunRefresh(cmd, fs, root, config); err != nil {
			return err
		}
		return hash.CleanupStaleHashes(fs, cmd, root)
	},
}

func init() {
	refreshCmd.Flags().Bool("clear-hashes", false, "Remove all build/check hashes")
}
