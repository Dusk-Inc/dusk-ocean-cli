package cmd

import (
	"fmt"

	functions "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/functions"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var addScopeCmd = &cobra.Command{
	Use:   "add-scope",
	Short: "Add a scope name to a repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		scopeName, err := cmd.Flags().GetString("scope-name")
		if err != nil {
			return err
		}
		target, err := cmd.Flags().GetString("target")
		if err != nil {
			return err
		}
		if scopeName == "" || target == "" {
			return fmt.Errorf("--scope-name and --target are required")
		}
		return functions.AddScope(cmd, afero.NewOsFs(), scopeName, target)
	},
}

var removeScopeCmd = &cobra.Command{
	Use:   "remove-scope",
	Short: "Remove a scope name from a repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		scopeName, err := cmd.Flags().GetString("scope-name")
		if err != nil {
			return err
		}
		target, err := cmd.Flags().GetString("target")
		if err != nil {
			return err
		}
		if scopeName == "" || target == "" {
			return fmt.Errorf("--scope-name and --target are required")
		}
		return functions.RemoveScope(cmd, afero.NewOsFs(), scopeName, target)
	},
}

func init() {
	addScopeCmd.Flags().String("scope-name", "", "Scope name to add")
	addScopeCmd.Flags().String("target", "", "Target repo name")
	removeScopeCmd.Flags().String("scope-name", "", "Scope name to remove")
	removeScopeCmd.Flags().String("target", "", "Target repo name")
}
