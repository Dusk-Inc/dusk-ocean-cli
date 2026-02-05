package cmd

import (
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/install"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Add a local dependency into a target repository",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return install.RunInstallPrompt(cmd, afero.NewOsFs())
	},
}
