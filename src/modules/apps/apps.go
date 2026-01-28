package apps

import "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"

// FindAppIndex returns the index of an app name, or -1 if not found.
func FindAppIndex(config workspace.WorkspaceConfig, appName string) int {
	return workspace.FindAppIndex(config, appName)
}
