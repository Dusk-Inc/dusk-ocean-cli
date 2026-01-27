package cmd

import (
	"fmt"
	"strings"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/install"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install [dependency]",
	Short: "Install a local dependency into the current target",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dependencyName := strings.TrimSpace(args[0])
		if dependencyName == "" {
			return fmt.Errorf("dependency name is required")
		}
		return install.RunInstallFromCwd(cmd, afero.NewOsFs(), dependencyName)
	},
}
