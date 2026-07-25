package cmd

import (
	"fmt"

	functions "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/functions"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var publishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Publish a project or service artifact (e.g. npm publish)",
}

var publishProjectCmd = &cobra.Command{
	Use:   "project",
	Short: "Publish a project artifact",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}
		if name == "" {
			return fmt.Errorf("--name is required")
		}
		skipPreflight, err := cmd.Flags().GetBool("skip-preflight")
		if err != nil {
			return err
		}
		selection := groupSelection(cmd)
		if !selection.IsBase {
			return functions.RunProjectLifecycleTask(cmd, afero.NewOsFs(), name, "publish", selection)
		}
		return functions.PublishProject(cmd, afero.NewOsFs(), name, skipPreflight)
	},
}

var publishServiceCmd = &cobra.Command{
	Use:   "service",
	Short: "Publish a service artifact (not yet implemented)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("publish service is not yet implemented")
	},
}

func init() {
	publishCmd.PersistentFlags().Bool("skip-preflight", false, "Skip pre-flight build/contain manifest checks")

	publishProjectCmd.Flags().String("name", "", "Name of the project")
	publishServiceCmd.Flags().String("name", "", "Name of the service")
	publishServiceCmd.Flags().String("in", "", "App name (required when service name is ambiguous)")
}
