package cmd

import
(
	functions "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/functions"
	"os/exec"
	"path/filepath"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var containCmd = &cobra.Command{
	Use:   "contain",
	Short: "Build and publish container images",
}

var containServiceCmd = &cobra.Command{
	Use:   "service",
	Short: "Build and publish a service container image",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}
		appName, err := cmd.Flags().GetString("in")
		if err != nil {
			return err
		}
		if appName == "" {
			selected, err := functions.PromptForApp()
			if err != nil {
				return err
			}
			appName = selected
		}
		if name == "" {
			selected, err := functions.PromptForService(appName)
			if err != nil {
				return err
			}
			name = selected
		}

		root, err := functions.GetRoot()
		if err != nil {
			return err
		}
		servicePath := filepath.Join(root, "repos", "apps", appName, "services", name)

		imageName, err := functions.ServiceImageReference(afero.NewOsFs(), appName, name)
		if err != nil {
			return err
		}

		buildCmd := exec.Command("docker", "build", "-t", imageName, ".")
		buildCmd.Dir = servicePath
		buildCmd.Stdout = cmd.OutOrStdout()
		buildCmd.Stderr = cmd.ErrOrStderr()
		buildCmd.Stdin = cmd.InOrStdin()
		if err := buildCmd.Run(); err != nil {
			return err
		}

		pushCmd := exec.Command("docker", "push", imageName)
		pushCmd.Stdout = cmd.OutOrStdout()
		pushCmd.Stderr = cmd.ErrOrStderr()
		pushCmd.Stdin = cmd.InOrStdin()
		return pushCmd.Run()
	},
}

func init() {
	containCmd.AddCommand(containServiceCmd)
	containServiceCmd.Flags().String("name", "", "Name of the service")
	containServiceCmd.Flags().String("in", "", "App name for the service")
}
