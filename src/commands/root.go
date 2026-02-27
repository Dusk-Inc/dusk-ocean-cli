package cmd

import
(
	functions "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/functions"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "Dusk-Inc Ocean",
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
	Short: "Use to scaffold new components (apps, services, libs)",
}

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Use to remove apps, services, libs from the repo.",
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
	rootCmd.AddCommand(containCmd)
	rootCmd.AddCommand(refreshCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(versionCmd)

	addCmd.AddCommand(addAppCmd)
	addCmd.AddCommand(addServiceCmd)
	addCmd.AddCommand(addLibCmd)
	addCmd.AddCommand(addPkgCmd)
	addCmd.AddCommand(addTestCmd)

	removeCmd.AddCommand(removeAppCmd)
	removeCmd.AddCommand(removeLibCmd)
	removeCmd.AddCommand(removePkgCmd)
	removeCmd.AddCommand(removeServiceCmd)
	removeCmd.AddCommand(removeTestCmd)

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

	initCmd.Flags().String("name", "", "Workspace name")
}
