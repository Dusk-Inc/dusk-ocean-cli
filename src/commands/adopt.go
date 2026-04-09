package cmd

import (
	"fmt"

	functions "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/functions"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var adoptCmd = &cobra.Command{
	Use:   "adopt <remote-url>",
	Short: "Clone an external git repo into the workspace and register it",
	Long: `Clone an external repo from <remote-url> into the deterministic
workspace path chosen by --kind, write a starter ocean.config.json at
the repo root, and add a new entry to ocean.workspace.json with the
remote URL populated.

If --name is omitted, it defaults to the basename of <remote-url>.
Authentication for private repositories is inherited from the user's
ambient git credentials; Dusk Ocean does not manage secrets.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		remoteURL := args[0]
		if remoteURL == "" {
			return fmt.Errorf("remote URL is required")
		}
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
		templateKind, err := cmd.Flags().GetString("template-kind")
		if err != nil {
			return err
		}
		return functions.AdoptRepo(afero.NewOsFs(), cmd.OutOrStdout(), cmd.ErrOrStderr(), remoteURL, kind, name, app, templateKind)
	},
}

func init() {
	adoptCmd.Flags().String("kind", "", "Repo kind (project, library, app, service, template)")
	adoptCmd.Flags().String("name", "", "Repo name (defaults to the basename of the remote URL)")
	adoptCmd.Flags().String("app", "", "Parent app name (required for service; optional for library)")
	adoptCmd.Flags().String("template-kind", "", "When --kind=template, the kind it scaffolds (service, library, project)")
}
