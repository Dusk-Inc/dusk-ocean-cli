package apps

import "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"

func FindAppIndex(config workspace.WorkspaceConfig, appName string) int {
	for i, app := range config.Apps {
		if app.Name == appName {
			return i
		}
	}
	return -1
}
