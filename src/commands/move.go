package cmd

import (
	"fmt"

	functions "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/functions"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var moveCmd = &cobra.Command{
	Use:   "move",
	Short: "Move a library to a different location in the workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		library, err := cmd.Flags().GetString("library")
		if err != nil {
			return err
		}
		fromApp, err := cmd.Flags().GetString("from-app")
		if err != nil {
			return err
		}
		toApp, err := cmd.Flags().GetString("to-app")
		if err != nil {
			return err
		}
		fromGlobal, err := cmd.Flags().GetBool("from-global")
		if err != nil {
			return err
		}
		toGlobal, err := cmd.Flags().GetBool("to-global")
		if err != nil {
			return err
		}

		if library == "" {
			return fmt.Errorf("--library is required")
		}

		hasFrom := fromApp != "" || fromGlobal
		hasTo := toApp != "" || toGlobal
		if !hasFrom {
			return fmt.Errorf("a source is required: use --from-app or --from-global")
		}
		if !hasTo {
			return fmt.Errorf("a destination is required: use --to-app or --to-global")
		}
		if fromApp != "" && fromGlobal {
			return fmt.Errorf("--from-app and --from-global are mutually exclusive")
		}
		if toApp != "" && toGlobal {
			return fmt.Errorf("--to-app and --to-global are mutually exclusive")
		}
		if fromGlobal && toGlobal {
			return fmt.Errorf("source and destination are the same")
		}
		if fromApp != "" && fromApp == toApp {
			return fmt.Errorf("source and destination are the same")
		}

		return functions.MoveLibrary(cmd, afero.NewOsFs(), functions.MoveLibraryOptions{
			Library:    library,
			FromApp:    fromApp,
			ToApp:      toApp,
			FromGlobal: fromGlobal,
			ToGlobal:   toGlobal,
		})
	},
}

func init() {
	moveCmd.Flags().String("library", "", "Name of the library to move")
	moveCmd.Flags().String("from-app", "", "Source app name")
	moveCmd.Flags().String("to-app", "", "Destination app name")
	moveCmd.Flags().Bool("from-global", false, "Source is global library scope")
	moveCmd.Flags().Bool("to-global", false, "Destination is global library scope")
}
