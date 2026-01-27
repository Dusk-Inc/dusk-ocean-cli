package cmd

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/prompts"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/tree"
)

var runAppCmd = &cobra.Command{
	Use:   "app",
	Short: "Run an app locally",
	RunE: func(cmd *cobra.Command, args []string) error {
		appName, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}
		if appName == "" {
			selected, err := prompts.PromptForApp()
			if err != nil {
				return err
			}
			appName = selected
		}
		noDev, err := cmd.Flags().GetBool("no-dev")
		if err != nil {
			return err
		}

		root, err := tree.GetRoot()
		if err != nil {
			return err
		}
		appPath := filepath.Join(root, "repos", "apps", appName)

		composeArgs := []string{"compose", "-f", "docker-compose.yml"}
		if !noDev {
			composeArgs = append(composeArgs, "-f", "docker-compose.dev.yml")
		}
		composeArgs = append(composeArgs, "up")

		execCmd := exec.Command("docker", composeArgs...)
		execCmd.Dir = appPath
		execCmd.Stdout = cmd.OutOrStdout()
		execCmd.Stderr = cmd.ErrOrStderr()
		execCmd.Stdin = cmd.InOrStdin()
		return execCmd.Run()
	},
}

var runServiceCmd = &cobra.Command{
	Use:   "service",
	Short: "Run one or more services within an app",
	RunE: func(cmd *cobra.Command, args []string) error {
		appName, err := cmd.Flags().GetString("in")
		if err != nil {
			return err
		}
		if appName == "" {
			selected, err := prompts.PromptForApp()
			if err != nil {
				return err
			}
			appName = selected
		}
		noDev, err := cmd.Flags().GetBool("no-dev")
		if err != nil {
			return err
		}

		services, err := tree.GetAppServices(appName)
		if err != nil {
			return err
		}
		if len(services) == 0 {
			return fmt.Errorf("no services found for app: %s", appName)
		}

		items := make([]string, 0, len(services)+1)
		for _, service := range services {
			items = append(items, service.Name)
		}
		items = append(items, "confirm")

		selected := []string{}
		selectedSet := map[string]bool{}
		for {
			prompt := promptui.Select{
				Label: "Select services",
				Items: items,
			}
			_, name, err := prompt.Run()
			if err != nil {
				return err
			}
			if name == "confirm" {
				if len(selected) == 0 {
					return fmt.Errorf("select at least one service")
				}
				break
			}
			if selectedSet[name] {
				selectedSet[name] = false
				next := make([]string, 0, len(selected)-1)
				for _, entry := range selected {
					if entry != name {
						next = append(next, entry)
					}
				}
				selected = next
			} else {
				selectedSet[name] = true
				selected = append(selected, name)
			}
		}

		root, err := tree.GetRoot()
		if err != nil {
			return err
		}
		appPath := filepath.Join(root, "repos", "apps", appName)

		composeArgs := []string{"compose", "-f", "docker-compose.yml"}
		if !noDev {
			composeArgs = append(composeArgs, "-f", "docker-compose.dev.yml")
		}
		composeArgs = append(composeArgs, "up")
		composeArgs = append(composeArgs, selected...)

		execCmd := exec.Command("docker", composeArgs...)
		execCmd.Dir = appPath
		execCmd.Stdout = cmd.OutOrStdout()
		execCmd.Stderr = cmd.ErrOrStderr()
		execCmd.Stdin = cmd.InOrStdin()
		return execCmd.Run()
	},
}

func init() {
	runAppCmd.Flags().String("name", "", "Name of the app")
	runAppCmd.Flags().Bool("no-dev", false, "Run without docker-compose.dev.yml")
	runServiceCmd.Flags().String("in", "", "App name for the service")
	runServiceCmd.Flags().Bool("no-dev", false, "Run without docker-compose.dev.yml")
}
