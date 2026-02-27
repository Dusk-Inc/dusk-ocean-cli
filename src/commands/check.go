package cmd

import
(
	functions "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/functions"
	"fmt"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"github.com/spf13/afero"
)

var checkAppCmd = &cobra.Command{
	Use:   "app",
	Short: "Test all services and libs within an app",
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

		passThrough, err := collectPassThroughArgs(cmd, args)
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

		for _, service := range appServices {
			node, err := functions.MakeServiceNode(config, appName, service.Name)
			if err != nil {
				return err
			}
			if err := functions.RunCheckWithDependencies(cmd, root, config, node, built, passThrough); err != nil {
				return err
			}
		}
		for _, lib := range libs {
			node, err := functions.MakeAppLibNode(config, appName, lib.Name)
			if err != nil {
				return err
			}
			if err := functions.RunCheckWithDependencies(cmd, root, config, node, built, passThrough); err != nil {
				return err
			}
		}
		for _, test := range tests {
			node, err := functions.MakeTestNode(config, appName, test.Name)
			if err != nil {
				return err
			}
			if err := functions.RunCheckWithDependencies(cmd, root, config, node, built, passThrough); err != nil {
				return err
			}
		}
		return nil
	},
}

var checkServiceCmd = &cobra.Command{
	Use:   "service",
	Short: "Test a service",
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

		passThrough, err := collectPassThroughArgs(cmd, args)
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
		node, err := functions.MakeServiceNode(config, appName, name)
		if err != nil {
			return err
		}
		return functions.RunCheckWithDependencies(cmd, root, config, node, map[string]struct{}{}, passThrough)
	},
}

var checkLibCmd = &cobra.Command{
	Use:   "library",
	Short: "Test a library",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}
		appName, err := cmd.Flags().GetString("in")
		if err != nil {
			return err
		}

		passThrough, err := collectPassThroughArgs(cmd, args)
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
			return functions.RunCheckWithDependencies(cmd, root, config, node, map[string]struct{}{}, passThrough)
		}

		if name != "" {
			node, err := functions.MakeGlobalLibNode(config, name)
			if err != nil {
				return err
			}
			return functions.RunCheckWithDependencies(cmd, root, config, node, map[string]struct{}{}, passThrough)
		}

		locationPrompt := promptui.Select{
			Label: "Test library from",
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
			return functions.RunCheckWithDependencies(cmd, root, config, node, map[string]struct{}{}, passThrough)
		}

		libName, err := functions.PromptForGlobalLib()
		if err != nil {
			return err
		}
		node, err := functions.MakeGlobalLibNode(config, libName)
		if err != nil {
			return err
		}
		return functions.RunCheckWithDependencies(cmd, root, config, node, map[string]struct{}{}, passThrough)
	},
}

var checkPkgCmd = &cobra.Command{
	Use:   "project",
	Short: "Test a project",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}
		passThrough, err := collectPassThroughArgs(cmd, args)
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

		node, err := functions.MakeProjectNode(config, name)
		if err != nil {
			return err
		}
		return functions.RunCheckWithDependencies(cmd, root, config, node, map[string]struct{}{}, passThrough)
	},
}

var checkTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test a testing project",
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

		passThrough, err := collectPassThroughArgs(cmd, args)
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
		node, err := functions.MakeTestNode(config, appName, name)
		if err != nil {
			return err
		}
		return functions.RunCheckWithDependencies(cmd, root, config, node, map[string]struct{}{}, passThrough)
	},
}

func init() {
	checkAppCmd.Flags().String("name", "", "Name of the app")
	checkLibCmd.Flags().String("name", "", "Name of the library")
	checkLibCmd.Flags().String("in", "", "App name for app libraries")
	checkServiceCmd.Flags().String("name", "", "Name of the service")
	checkServiceCmd.Flags().String("in", "", "App name for the service")
	checkPkgCmd.Flags().String("name", "", "Name of the project")
	checkTestCmd.Flags().String("name", "", "Name of the test")
	checkTestCmd.Flags().String("in", "", "App name for the test")
}

func collectPassThroughArgs(cmd *cobra.Command, args []string) ([]string, error) {
	argsLenAtDash := cmd.ArgsLenAtDash()
	if argsLenAtDash == -1 {
		if len(args) > 0 {
			return nil, fmt.Errorf("pass-through args require a -- separator")
		}
		return nil, nil
	}
	if argsLenAtDash > 0 {
		return nil, fmt.Errorf("unexpected args before --")
	}
	if argsLenAtDash >= len(args) {
		return nil, nil
	}
	return args[argsLenAtDash:], nil
}
