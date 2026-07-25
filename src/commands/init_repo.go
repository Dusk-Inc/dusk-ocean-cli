package cmd

import (
	functions "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/functions"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var initRepoCmd = &cobra.Command{
	Use:   "init-repo",
	Short: "Initialize git for a registered repo and optionally create its remote",
	Long: `Initialize a git repository for an already-registered repo: it runs the
workspace 'init' and 'initial_commit' tasks (git init on the default branch plus a
first commit) and, unless --no-remote is given, the 'create_remote' task to create
and attach the remote derived from the workspace 'org' variable and the repo name.

The recorded remote is written to the repo's entry in ocean.workspace.json so it can
be edited later. A failed remote creation is non-fatal: the local repository is still
initialized and a warning is printed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		kind, err := cmd.Flags().GetString("kind")
		if err != nil {
			return err
		}
		name, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}
		app, err := cmd.Flags().GetString("app")
		if err != nil {
			return err
		}
		remote, err := cmd.Flags().GetString("remote")
		if err != nil {
			return err
		}
		noRemote, err := cmd.Flags().GetBool("no-remote")
		if err != nil {
			return err
		}
		return functions.InitRepoVcs(afero.NewOsFs(), cmd.OutOrStdout(), cmd.ErrOrStderr(), kind, name, app, remote, noRemote)
	},
}

func init() {
	initRepoCmd.Flags().String("kind", "", "Repo kind (project, library, app, infra, docs)")
	initRepoCmd.Flags().String("name", "", "Repo name (also the directory basename)")
	initRepoCmd.Flags().String("app", "", "Parent app name (optional; app-scoped libraries inherit their app's git history)")
	initRepoCmd.Flags().String("remote", "", "Override the recorded remote value; defaults to the derived \"<org>/<name>\"")
	initRepoCmd.Flags().Bool("no-remote", false, "Initialize locally only; skip create_remote and record the remote as \"None\"")
}
