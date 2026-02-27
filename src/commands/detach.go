package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var detachAppCmd = &cobra.Command{
	Use:   "app",
	Short: "Detach an app",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("detach app is not yet implemented")
	},
}

var detachPkgCmd = &cobra.Command{
	Use:   "project",
	Short: "Detach a project",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("detach project is not yet implemented")
	},
}
