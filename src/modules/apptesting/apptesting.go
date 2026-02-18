package apptesting

import (
	"fmt"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/deps"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"
)

func MakeTestNode(config workspace.WorkspaceConfig, appName string, name string) (deps.Node, error) {
	appIndex := workspace.FindAppIndex(config, appName)
	if appIndex == -1 {
		return deps.Node{}, fmt.Errorf("app not registered in workspace: %s", appName)
	}
	testIndex := workspace.FindAppTestIndex(config.Apps[appIndex], name)
	if testIndex == -1 {
		return deps.Node{}, fmt.Errorf("test not registered in workspace: %s", name)
	}
	test := config.Apps[appIndex].Testing[testIndex]
	return deps.Node{
		Kind: deps.NodeAppTest,
		App:  appName,
		Name: test.Name,
		Deps: test.Deps,
	}, nil
}
