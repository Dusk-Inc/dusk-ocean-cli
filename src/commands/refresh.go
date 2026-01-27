package cmd

import (
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/hash"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/tree"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Validate workspace state and orchestration configs",
	RunE: func(cmd *cobra.Command, args []string) error {
		clearHashes, err := cmd.Flags().GetBool("clear-hashes")
		if err != nil {
			return err
		}
		root, err := tree.GetRoot()
		if err != nil {
			return err
		}
		fs := afero.NewOsFs()

		if clearHashes {
			return hash.ClearHashes(fs, cmd, root)
		}
		if err := workspace.ValidateComposeConsistency(fs, root); err != nil {
			return err
		}
		if err := hash.CleanupStaleHashes(fs, cmd, root); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	refreshCmd.Flags().Bool("clear-hashes", false, "Remove all build/check hashes")
}
