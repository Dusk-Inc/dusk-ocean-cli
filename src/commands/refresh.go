package cmd

import (
	"fmt"

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
		repo, err := cmd.Flags().GetString("repo")
		if err != nil {
			return err
		}
		noDeps, err := cmd.Flags().GetBool("no-deps")
		if err != nil {
			return err
		}
		if noDeps && repo == "" {
			return fmt.Errorf("--no-deps requires --repo: no target to strip dependencies from")
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

		if repo != "" {
			if err := functions.RunScopedRefresh(cmd, fs, root, config, repo, noDeps); err != nil {
				return err
			}
		} else {
			if err := functions.RunRefresh(cmd, fs, root, config); err != nil {
				return err
			}
		}
		return functions.CleanupStaleHashes(fs, cmd, root)
	},
}

func init() {
	refreshCmd.Flags().Bool("clear-hashes", false, "Remove all build/check hashes")
	refreshCmd.Flags().String("repo", "", "Scope the refresh to one workspace repo (and, by default, its transitive dependencies)")
	refreshCmd.Flags().Bool("no-deps", false, "With --repo, refresh only that repo and skip its transitive dependencies")
}
