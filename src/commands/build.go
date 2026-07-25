package cmd

import (
	"fmt"
	functions "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/functions"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"github.com/spf13/afero"
)

var buildAppCmd = &cobra.Command{
	Use:   "app",
	Short: "Build all services and libs within an app",
	RunE: func(cmd *cobra.Command, args []string) error {
		appName, err := cmd.Flags().GetString("name")
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

		appServices, err := functions.GetAppServices(appName)
		if err != nil {
			return err
		}
		libs, err := functions.GetAppLibs(appName)
		if err != nil {
			return err
		}
		tests, err := functions.GetAppTests(appName)
		if err != nil {
			return err
		}
		if len(appServices) == 0 && len(libs) == 0 && len(tests) == 0 {
			return fmt.Errorf("no services, libs, or tests found for app: %s", appName)
		}

		root, err := functions.GetRoot()
		if err != nil {
			return err
		}

		fs := afero.NewOsFs()
		config, err := functions.ReadWorkspaceConfig(fs)
		if err != nil {
			return err
		}

		built := map[string]struct{}{}
		for _, service := range appServices {
			node, err := functions.MakeServiceNode(config, appName, service.Name)
			if err != nil {
				return err
			}
			if err := functions.RunBuildWithDependencies(cmd, root, config, node, built); err != nil {
				return err
			}
		}
		for _, lib := range libs {
			node, err := functions.MakeAppLibNode(config, appName, lib.Name)
			if err != nil {
				return err
			}
			if err := functions.RunBuildWithDependencies(cmd, root, config, node, built); err != nil {
				return err
			}
		}
		for _, test := range tests {
			node, err := functions.MakeTestNode(config, appName, test.Name)
			if err != nil {
				return err
			}
			if err := functions.RunBuildWithDependencies(cmd, root, config, node, built); err != nil {
				return err
			}
		}
		return nil
	},
}

var buildLibCmd = &cobra.Command{
	Use:   "library",
	Short: "Build a library",
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
		fs := afero.NewOsFs()
		config, err := functions.ReadWorkspaceConfig(fs)
		if err != nil {
			return err
		}
		built := map[string]struct{}{}

		if appName != "" {
			if name == "" {
				selected, err := functions.PromptForAppLib(appName)
				if err != nil {
					return err
				}
				name = selected
			}
			node, err := functions.MakeAppLibNode(config, appName, name)
			if err != nil {
				return err
			}
			return functions.RunBuildWithDependencies(cmd, root, config, node, built)
		}

		if name != "" {
			node, err := functions.MakeGlobalLibNode(config, name)
			if err != nil {
				return err
			}
			return functions.RunBuildWithDependencies(cmd, root, config, node, built)
		}

		locationPrompt := promptui.Select{
			Label: "Build library from",
			Items: []string{"global", "app"},
		}
		_, location, err := locationPrompt.Run()
		if err != nil {
			return err
		}
		if location == "app" {
			selectedApp, err := functions.PromptForApp()
			if err != nil {
				return err
			}
			selectedLib, err := functions.PromptForAppLib(selectedApp)
			if err != nil {
				return err
			}
			node, err := functions.MakeAppLibNode(config, selectedApp, selectedLib)
			if err != nil {
				return err
			}
			return functions.RunBuildWithDependencies(cmd, root, config, node, built)
		}

		libName, err := functions.PromptForGlobalLib()
		if err != nil {
			return err
		}
		node, err := functions.MakeGlobalLibNode(config, libName)
		if err != nil {
			return err
		}
		return functions.RunBuildWithDependencies(cmd, root, config, node, built)
	},
}

var buildServiceCmd = &cobra.Command{
	Use:   "service",
	Short: "Build a service",
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
		fs := afero.NewOsFs()
		config, err := functions.ReadWorkspaceConfig(fs)
		if err != nil {
			return err
		}
		node, err := functions.MakeServiceNode(config, appName, name)
		if err != nil {
			return err
		}
		return functions.RunBuildWithDependencies(cmd, root, config, node, map[string]struct{}{})
	},
}

var buildPkgCmd = &cobra.Command{
	Use:   "project",
	Short: "Build a project",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}
		root, err := functions.GetRoot()
		if err != nil {
			return err
		}
		fs := afero.NewOsFs()
		config, err := functions.ReadWorkspaceConfig(fs)
		if err != nil {
			return err
		}

		if name == "" {
			selected, err := functions.PromptForProject()
			if err != nil {
				return err
			}
			name = selected
		}

		selection := groupSelection(cmd)
		if !selection.IsBase {
			return functions.RunProjectLifecycleTask(cmd, fs, name, "build", selection)
		}

		node, err := functions.MakeProjectNode(config, name)
		if err != nil {
			return err
		}
		if err := functions.EnsureManifestEntryForNode(fs, root, node); err != nil {
			return err
		}
		return functions.RunBuildWithDependencies(cmd, root, config, node, map[string]struct{}{})
	},
}

var buildTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Build a testing project",
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

		root, err := functions.GetRoot()
		if err != nil {
			return err
		}
		fs := afero.NewOsFs()
		config, err := functions.ReadWorkspaceConfig(fs)
		if err != nil {
			return err
		}
		node, err := functions.MakeTestNode(config, appName, name)
		if err != nil {
			return err
		}
		return functions.RunBuildWithDependencies(cmd, root, config, node, map[string]struct{}{})
	},
}

func init() {
	buildAppCmd.Flags().String("name", "", "Name of the app")
	buildLibCmd.Flags().String("name", "", "Name of the library")
	buildLibCmd.Flags().String("in", "", "App name for app libraries")
	buildServiceCmd.Flags().String("name", "", "Name of the service")
	buildServiceCmd.Flags().String("in", "", "App name for the service")
	buildPkgCmd.Flags().String("name", "", "Name of the project")
	buildTestCmd.Flags().String("name", "", "Name of the test")
	buildTestCmd.Flags().String("in", "", "App name for the test")
}
