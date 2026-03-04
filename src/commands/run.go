package cmd

import (
	"fmt"

	functions "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/functions"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var runAppCmd = &cobra.Command{
	Use:   "app",
	Short: "Run an app with pre-flight build, check, and contain",
	RunE: func(cmd *cobra.Command, args []string) error {
		appName, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}
		if appName == "" {
			return fmt.Errorf("--name is required")
		}
		return functions.RunApp(cmd, afero.NewOsFs(), appName)
	},
}

var runServiceCmd = &cobra.Command{
	Use:   "service",
	Short: "Run a service with pre-flight build, check, and contain",
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}
		appName, err := cmd.Flags().GetString("app")
		if err != nil {
			return err
		}
		if serviceName == "" {
			return fmt.Errorf("--name is required")
		}
		return functions.RunService(cmd, afero.NewOsFs(), appName, serviceName)
	},
}

func init() {
	runAppCmd.Flags().String("name", "", "Name of the app")
	runServiceCmd.Flags().String("name", "", "Name of the service")
	runServiceCmd.Flags().String("app", "", "App name (required if service name is ambiguous)")
}
