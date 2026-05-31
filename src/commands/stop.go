package cmd

import (
	"fmt"

	functions "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/functions"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var stopAppCmd = &cobra.Command{
	Use:   "app",
	Short: "Stop an app by running its `stop` task",
	RunE: func(cmd *cobra.Command, args []string) error {
		appName, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}
		if appName == "" {
			return fmt.Errorf("--name is required")
		}
		return functions.StopApp(cmd, afero.NewOsFs(), appName)
	},
}

var stopServiceCmd = &cobra.Command{
	Use:   "service",
	Short: "Stop a service by running its `stop` task",
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
		return functions.StopService(cmd, afero.NewOsFs(), appName, serviceName)
	},
}

func init() {
	stopAppCmd.Flags().String("name", "", "Name of the app")
	stopServiceCmd.Flags().String("name", "", "Name of the service")
	stopServiceCmd.Flags().String("app", "", "App name (required if service name is ambiguous)")
}
