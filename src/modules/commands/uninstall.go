package cmd

import (
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/uninstall"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall a local dependency from a target repository",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return uninstall.RunUninstallPrompt(cmd, afero.NewOsFs())
	},
}
