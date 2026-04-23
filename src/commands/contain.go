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
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName, err := cmd.Flags().GetString("service")
		if err != nil {
			return err
		}
		appName, err := cmd.Flags().GetString("app")
		if err != nil {
			return err
		}
		projectName, err := cmd.Flags().GetString("project")
		if err != nil {
			return err
		}
		if serviceName == "" && projectName == "" {
			return fmt.Errorf("one of --service or --project is required")
		}
		if serviceName != "" && projectName != "" {
			return fmt.Errorf("--service and --project are mutually exclusive")
		}
		if projectName != "" {
			if appName != "" {
				return fmt.Errorf("--app cannot be combined with --project")
			}
			return functions.ContainProject(cmd, afero.NewOsFs(), projectName)
		}
		return functions.ContainService(cmd, afero.NewOsFs(), appName, serviceName)
	},
}

func init() {
	containCmd.Flags().String("service", "", "Name of the service to containerize")
	containCmd.Flags().String("app", "", "App name (required if service name is ambiguous)")
	containCmd.Flags().String("project", "", "Name of the project to containerize")
}
