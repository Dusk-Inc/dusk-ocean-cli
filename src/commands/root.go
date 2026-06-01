package cmd

import (
	"fmt"
	functions "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/functions"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "dusk-ocean",
	Short: "Dusk Ocean CLI - Manage the Dusk Inc Monorepo",
	Long:  `A polyglot monorepo tool for scaffolding, testing, and deploying functions.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "version" || cmd.Name() == "init" || cmd.Name() == "help" {
			return nil
		}
		_, err := functions.EnsureWorkspaceRoot(afero.NewOsFs())
		return err
	},
}

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Scaffold new components, or wire a local dependency with --payload and --target",
	RunE: func(cmd *cobra.Command, args []string) error {
		payload, err := cmd.Flags().GetString("payload")
		if err != nil {
			return err
		}
		target, err := cmd.Flags().GetString("target")
		if err != nil {
			return err
		}
		app, err := cmd.Flags().GetString("app")
		if err != nil {
			return err
		}
		if payload == "" && target == "" {
			return cmd.Help()
		}
		if payload == "" || target == "" {
			return fmt.Errorf("both --payload and --target are required")
		}
		return functions.WireLocalDependency(cmd, afero.NewOsFs(), payload, target, app)
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove apps, services, libs, or unwire a dependency with --payload and --target",
	RunE: func(cmd *cobra.Command, args []string) error {
		payload, err := cmd.Flags().GetString("payload")
		if err != nil {
			return err
		}
		target, err := cmd.Flags().GetString("target")
		if err != nil {
			return err
		}
		app, err := cmd.Flags().GetString("app")
		if err != nil {
			return err
		}
		if payload == "" && target == "" {
			return cmd.Help()
		}
		if payload == "" || target == "" {
			return fmt.Errorf("both --payload and --target are required")
		}
		return functions.UnwireLocalDependency(cmd, afero.NewOsFs(), payload, target, app)
	},
}

var detachCmd = &cobra.Command{
	Use:   "detach",
	Short: "Use to create a git subtree for a component of the repo.",
}

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Use to build components made from compiled languages.",
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Used to test components within the repo.",
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Use to run a service, app, or collection.",
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Use to stop a service or app via its `stop` task.",
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a Dusk Ocean workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}
		return functions.InitWorkspace(afero.NewOsFs(), cmd.OutOrStdout(), functions.InitOptions{
			Name: name,
		})
	},
}

func init() {
	rootCmd.Version = functions.Version
	rootCmd.SetVersionTemplate("{{.Version}}\n")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(detachCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(containCmd)
	rootCmd.AddCommand(publishCmd)
	rootCmd.AddCommand(refreshCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(menuCmd)
	rootCmd.AddCommand(addScopeCmd)
	rootCmd.AddCommand(removeScopeCmd)
	rootCmd.AddCommand(renameCmd)
	rootCmd.AddCommand(moveCmd)
	rootCmd.AddCommand(hashCmd)
	rootCmd.AddCommand(adoptCmd)
	rootCmd.AddCommand(registerCmd)
	rootCmd.AddCommand(taskCmd)

	menuCmd.AddCommand(menuCreateCmd)
	menuCmd.AddCommand(menuRemoveCmd)

	addCmd.AddCommand(addAppCmd)
	addCmd.AddCommand(addServiceCmd)
	addCmd.AddCommand(addLibCmd)
	addCmd.AddCommand(addPkgCmd)
	addCmd.AddCommand(addTestCmd)
	addCmd.AddCommand(addInfraCmd)
	addCmd.AddCommand(addDocsCmd)

	removeCmd.AddCommand(removeAppCmd)
	removeCmd.AddCommand(removeLibCmd)
	removeCmd.AddCommand(removePkgCmd)
	removeCmd.AddCommand(removeServiceCmd)
	removeCmd.AddCommand(removeTestCmd)
	removeCmd.AddCommand(removeInfraCmd)
	removeCmd.AddCommand(removeDocsCmd)

	detachCmd.AddCommand(detachAppCmd)
	detachCmd.AddCommand(detachPkgCmd)

	buildCmd.AddCommand(buildAppCmd)
	buildCmd.AddCommand(buildLibCmd)
	buildCmd.AddCommand(buildServiceCmd)
	buildCmd.AddCommand(buildPkgCmd)
	buildCmd.AddCommand(buildTestCmd)

	checkCmd.AddCommand(checkAppCmd)
	checkCmd.AddCommand(checkLibCmd)
	checkCmd.AddCommand(checkPkgCmd)
	checkCmd.AddCommand(checkServiceCmd)
	checkCmd.AddCommand(checkTestCmd)

	runCmd.AddCommand(runAppCmd)
	runCmd.AddCommand(runServiceCmd)

	stopCmd.AddCommand(stopAppCmd)
	stopCmd.AddCommand(stopServiceCmd)

	initCmd.Flags().String("name", "", "Workspace name")
	addCmd.Flags().String("payload", "", "Library to wire as a dependency")
	addCmd.Flags().String("target", "", "Target repo to receive the dependency")
	addCmd.Flags().String("app", "", "App that contains the target (required if target name is ambiguous across apps)")
	removeCmd.Flags().String("payload", "", "Library to unwire from the target")
	removeCmd.Flags().String("target", "", "Target repo to remove the dependency from")
	removeCmd.Flags().String("app", "", "App that contains the target (required if target name is ambiguous across apps)")
}
