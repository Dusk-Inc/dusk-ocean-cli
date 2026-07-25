package functions

import (
	"fmt"
)

func MakeTestNode(config WorkspaceConfig, appName string, name string) (Node, error) {
	appIndex := FindAppIndex(config, appName)
	if appIndex == -1 {
		return Node{}, fmt.Errorf("app not registered in workspace: %s", appName)
	}
	testIndex := FindAppTestIndex(config.Apps[appIndex], name)
	if testIndex == -1 {
		return Node{}, fmt.Errorf("test not registered in workspace: %s", name)
	}
	test := config.Apps[appIndex].Testing[testIndex]
	return Node{
		Kind: NodeAppTest,
		App:  appName,
		Name: test.Name,
		Deps: test.Deps,
	}, nil
}
