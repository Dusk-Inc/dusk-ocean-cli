package cmd

import (
	"fmt"
	functions "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/functions"
	"os"
	"path/filepath"
	"strings"

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
			selected, err := functions.PromptForApp()
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

		return functions.RemoveApp(afero.NewOsFs(), name)
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

		root, err := functions.GetRoot()
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
				selectedApp, err := functions.PromptForApp()
				if err != nil {
					return err
				}
				appName = selectedApp
			}
			if name == "" {
				selectedName, err := functions.PromptForAppLib(appName)
				if err != nil {
					return err
				}
				name = selectedName
			}
		} else {
			if name == "" {
				selectedName, err := functions.PromptForGlobalLib()
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
		source := "global"
		if location == "app" {
			source = appName
		}

		workspaceConfig, err := functions.ReadWorkspaceConfig(fs)
		if err != nil {
			return err
		}
		dependents := functions.CollectLibraryDependents(workspaceConfig, root, name, source)
		if len(dependents) > 0 {
			for _, target := range dependents {
				if _, err := fs.Stat(target.Path); err != nil {
					if os.IsNotExist(err) {
						return fmt.Errorf("dependency target does not exist: %s", target.Path)
					}
					return err
				}
			}
			if err := functions.RunUninstallForTargets(cmd, fs, path, name, dependents, functions.UninstallOptions{}); err != nil {
				return err
			}
		}

		if err := fs.RemoveAll(path); err != nil {
			return err
		}

		return functions.UpdateConfig(fs, func(config functions.WorkspaceConfig) (functions.WorkspaceConfig, error) {
			config = functions.RemoveLibraryDeps(config, name, source)
			if location == "app" {
				return functions.RemoveAppLibraryFromConfig(config, appName, name), nil
			}
			return functions.RemoveGlobalLibraryFromConfig(config, name), nil
		})
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
		deleteDir, err := cmd.Flags().GetBool("delete")
		if err != nil {
			return err
		}
		root, err := functions.GetRoot()
		if err != nil {
			return err
		}
		if name == "" {
			selectedName, err := functions.PromptForProject()
			if err != nil {
				return err
			}
			name = selectedName
		}

		var label string
		if deleteDir {
			label = fmt.Sprintf("Remove project %q AND its directory on disk? Files are permanently deleted", name)
		} else {
			label = fmt.Sprintf("Remove project %q from workspace registration? (directory on disk is kept; pass --delete to also remove it)", name)
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
		if deleteDir {
			path := filepath.Join(root, "repos", "projects", name)
			if _, err := fs.Stat(path); err != nil {
				if !os.IsNotExist(err) {
					return err
				}
			} else if err := fs.RemoveAll(path); err != nil {
				return err
			}
		}
		return functions.RemoveProjectFromWorkspace(fs, name)
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

		root, err := functions.GetRoot()
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
		return functions.RemoveServiceFromWorkspace(fs, appName, name)
	},
}

var removeTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Remove a testing project from an app",
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
			selected, err := functions.PromptForTest(appName)
			if err != nil {
				return err
			}
			name = selected
		}

		prompt := promptui.Prompt{
			Label:     fmt.Sprintf("Remove test %q from app %q? This action is permanent", name, appName),
			IsConfirm: true,
		}
		confirm, err := prompt.Run()
		if err != nil {
			return err
		}
		if strings.ToLower(confirm) != "y" {
			return fmt.Errorf("aborted")
		}

		root, err := functions.GetRoot()
		if err != nil {
			return err
		}
		path := filepath.Join(root, "repos", "apps", appName, "testing", name)
		fs := afero.NewOsFs()
		if _, err := fs.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("test does not exist: %s", name)
			}
			return err
		}
		if err := fs.RemoveAll(path); err != nil {
			return err
		}
		return functions.RemoveTestFromWorkspace(fs, appName, name)
	},
}

var removeInfraCmd = &cobra.Command{
	Use:   "infra",
	Short: "Remove an infrastructure repo from repos/infra",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRemoveNonCodeRepo(cmd, "infra")
	},
}

var removeDocsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Remove a docs repo from repos/docs",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRemoveNonCodeRepo(cmd, "docs")
	},
}

func runRemoveNonCodeRepo(cmd *cobra.Command, kind string) error {
	name, err := cmd.Flags().GetString("name")
	if err != nil {
		return err
	}
	fs := afero.NewOsFs()
	if name == "" {
		config, err := functions.ReadWorkspaceConfig(fs)
		if err != nil {
			return err
		}
		var names []string
		switch kind {
		case "infra":
			names = functions.InfraNames(config)
		case "docs":
			names = functions.DocsNames(config)
		}
		if len(names) == 0 {
			return fmt.Errorf("no %s repos registered", kind)
		}
		selected, err := functions.SelectFromList(fmt.Sprintf("Select %s repo", kind), names)
		if err != nil {
			return err
		}
		name = selected
	}

	prompt := promptui.Prompt{
		Label:     fmt.Sprintf("Remove %s repo %q? This action is permanent", kind, name),
		IsConfirm: true,
	}
	confirm, err := prompt.Run()
	if err != nil {
		return err
	}
	if strings.ToLower(confirm) != "y" {
		return fmt.Errorf("aborted")
	}

	switch kind {
	case "infra":
		return functions.RemoveInfra(fs, name)
	case "docs":
		return functions.RemoveDocs(fs, name)
	}
	return fmt.Errorf("unsupported kind: %s", kind)
}

func init() {
	removeAppCmd.Flags().String("name", "", "Name of the app")
	removeLibCmd.Flags().String("name", "", "Name of the library")
	removeLibCmd.Flags().String("in", "", "App name for the library")
	removePkgCmd.Flags().String("name", "", "Name of the project")
	removePkgCmd.Flags().Bool("delete", false, "Also delete the project directory on disk (default: keep)")
	removeServiceCmd.Flags().String("name", "", "Name of the service")
	removeServiceCmd.Flags().String("in", "", "App name for the service")
	removeTestCmd.Flags().String("name", "", "Name of the test")
	removeTestCmd.Flags().String("in", "", "App name for the test")
	removeInfraCmd.Flags().String("name", "", "Name of the infra repo")
	removeDocsCmd.Flags().String("name", "", "Name of the docs repo")
}
