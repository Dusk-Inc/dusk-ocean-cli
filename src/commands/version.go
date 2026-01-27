package cmd

import (
	"fmt"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/config"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the CLI version",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), config.Version)
		return err
	},
}
