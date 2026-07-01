package cmd

import (
	"fmt"

	functions "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/functions"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var containCmd = &cobra.Command{
	Use:   "contain",
	Short: "Build and publish a service or project container image",
}

var containProjectCmd = &cobra.Command{
	Use:   "project",
	Short: "Containerize a project",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}
		if name == "" {
			return fmt.Errorf("--name is required")
		}
		selection := groupSelection(cmd)
		if !selection.IsBase {
			return functions.RunProjectLifecycleTask(cmd, afero.NewOsFs(), name, "contain", selection)
		}
		return functions.ContainProject(cmd, afero.NewOsFs(), name)
	},
}

var containServiceCmd = &cobra.Command{
	Use:   "service",
	Short: "Containerize a service",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}
		if name == "" {
			return fmt.Errorf("--name is required")
		}
		appName, err := cmd.Flags().GetString("in")
		if err != nil {
			return err
		}
		return functions.ContainService(cmd, afero.NewOsFs(), appName, name)
	},
}

func init() {
	containProjectCmd.Flags().String("name", "", "Name of the project")
	containServiceCmd.Flags().String("name", "", "Name of the service")
	containServiceCmd.Flags().String("in", "", "App name (required when service name is ambiguous)")
}
