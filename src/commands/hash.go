package cmd

import (
	functions "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/functions"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var hashCmd = &cobra.Command{
	Use:   "hash",
	Short: "Compute directory hashes for all repos and update .ocean/manifest.json",
	RunE: func(cmd *cobra.Command, args []string) error {
		target, err := cmd.Flags().GetString("target")
		if err != nil {
			return err
		}
		fs := afero.NewOsFs()
		root, err := functions.EnsureWorkspaceRoot(fs)
		if err != nil {
			return err
		}
		config, err := functions.ReadWorkspaceConfig(fs)
		if err != nil {
			return err
		}
		if target != "" {
			return functions.HashSingleRepo(cmd, fs, root, config, target)
		}
		return functions.HashAllRepos(cmd, fs, root, config)
	},
}

func init() {
	hashCmd.Flags().String("target", "", "Name of a specific repository to hash (optional; hashes all repos when omitted)")
}
