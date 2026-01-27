package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/libraries"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/projects"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/prompts"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/scaffold"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/services"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/tree"
	"github.com/manifoldco/promptui"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var removeAppCmd = &cobra.Command{
	Use:   "app",
	Short: "Remove an app from repos/apps",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}
		if name == "" {
			selected, err := prompts.PromptForApp()
			if err != nil {
				return err
			}
			name = selected
		}

		prompt := promptui.Prompt{
			Label:     fmt.Sprintf("Remove app %q? This action is permanent", name),
			IsConfirm: true,
		}
		confirm, err := prompt.Run()
		if err != nil {
			return err
		}
		if strings.ToLower(confirm) != "y" {
			return fmt.Errorf("aborted")
		}

		return scaffold.RemoveApp(afero.NewOsFs(), name)
	},
}

var removeLibCmd = &cobra.Command{
	Use:   "library",
	Short: "Remove a library from repos/libs",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}
		appName, err := cmd.Flags().GetString("in")
		if err != nil {
			return err
		}

		root, err := tree.GetRoot()
		if err != nil {
			return err
		}
		location := "global"
		if appName != "" {
			location = "app"
		} else if name == "" {
			locationPrompt := promptui.Select{
				Label: "Remove library from",
				Items: []string{"global", "app"},
			}
			_, choice, err := locationPrompt.Run()
			if err != nil {
				return err
			}
			location = choice
		}

		if location == "app" {
			if appName == "" {
				selectedApp, err := prompts.PromptForApp()
				if err != nil {
					return err
				}
				appName = selectedApp
			}
			if name == "" {
				selectedName, err := prompts.PromptForAppLib(appName)
				if err != nil {
					return err
				}
				name = selectedName
			}
		} else {
			if name == "" {
				selectedName, err := prompts.PromptForGlobalLib()
				if err != nil {
					return err
				}
				name = selectedName
			}
		}

		label := fmt.Sprintf("Remove library %q? This action is permanent", name)
		if location == "app" {
			label = fmt.Sprintf("Remove library %q from app %q? This action is permanent", name, appName)
		}
		prompt := promptui.Prompt{
			Label:     label,
			IsConfirm: true,
		}
		confirm, err := prompt.Run()
		if err != nil {
			return err
		}
		if strings.ToLower(confirm) != "y" {
			return fmt.Errorf("aborted")
		}

		fs := afero.NewOsFs()
		path := ""
		if location == "app" {
			path = filepath.Join(root, "repos", "apps", appName, "libs", name)
		} else {
			path = filepath.Join(root, "repos", "libs", name)
		}
		if _, err := fs.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("library does not exist: %s", name)
			}
			return err
		}
		if err := fs.RemoveAll(path); err != nil {
			return err
		}
		if location == "app" {
			return libraries.RemoveAppLibraryFromWorkspace(fs, appName, name)
		}
		return libraries.RemoveGlobalLibraryFromWorkspace(fs, name)
	},
}

var removePkgCmd = &cobra.Command{
	Use:   "project",
	Short: "Remove a project from repos/projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}
		root, err := tree.GetRoot()
		if err != nil {
			return err
		}
		if name == "" {
			selectedName, err := prompts.PromptForProject()
			if err != nil {
				return err
			}
			name = selectedName
		}

		prompt := promptui.Prompt{
			Label:     fmt.Sprintf("Remove project %q? This action is permanent", name),
			IsConfirm: true,
		}
		confirm, err := prompt.Run()
		if err != nil {
			return err
		}
		if strings.ToLower(confirm) != "y" {
			return fmt.Errorf("aborted")
		}

		path := filepath.Join(root, "repos", "projects", name)
		fs := afero.NewOsFs()
		if _, err := fs.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("project does not exist: %s", name)
			}
			return err
		}
		if err := fs.RemoveAll(path); err != nil {
			return err
		}
		return projects.RemoveProjectFromWorkspace(fs, name)
	},
}

var removeServiceCmd = &cobra.Command{
	Use:   "service",
	Short: "Remove a service from repos/apps",
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
			selected, err := prompts.PromptForApp()
			if err != nil {
				return err
			}
			appName = selected
		}
		if name == "" {
			selected, err := prompts.PromptForService(appName)
			if err != nil {
				return err
			}
			name = selected
		}

		prompt := promptui.Prompt{
			Label:     fmt.Sprintf("Remove service %q from app %q? This action is permanent", name, appName),
			IsConfirm: true,
		}
		confirm, err := prompt.Run()
		if err != nil {
			return err
		}
		if strings.ToLower(confirm) != "y" {
			return fmt.Errorf("aborted")
		}

		root, err := tree.GetRoot()
		if err != nil {
			return err
		}
		path := filepath.Join(root, "repos", "apps", appName, "services", name)
		fs := afero.NewOsFs()
		if _, err := fs.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("service does not exist: %s", name)
			}
			return err
		}
		if err := fs.RemoveAll(path); err != nil {
			return err
		}
		return services.RemoveServiceFromWorkspace(fs, appName, name)
	},
}

func init() {
	removeAppCmd.Flags().String("name", "", "Name of the app")
	removeLibCmd.Flags().String("name", "", "Name of the library")
	removeLibCmd.Flags().String("in", "", "App name for the library")
	removePkgCmd.Flags().String("name", "", "Name of the project")
	removeServiceCmd.Flags().String("name", "", "Name of the service")
	removeServiceCmd.Flags().String("in", "", "App name for the service")
}
