package main

import (
	"os"

	cmd "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/commands"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
