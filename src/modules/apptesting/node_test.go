package apptesting

import (
	"testing"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/deps"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"
)

func TestMakeTestNode(t *testing.T) {
	t.Run("domain__registered_test__returns_node", func(t *testing.T) {
		config := workspace.WorkspaceConfig{
			Apps: []workspace.WorkspaceApp{
				{
					Name: "plexus",
					Testing: []workspace.WorkspaceTest{
						{
							Name: "sdk-e2e-ts",
							Deps: []workspace.WorkspaceDep{{Lib: "plexus-core", From: "plexus"}},
						},
					},
				},
			},
		}

		node, err := MakeTestNode(config, "plexus", "sdk-e2e-ts")
		if err != nil {
			t.Fatalf("MakeTestNode: %v", err)
		}
		if node.Kind != deps.NodeAppTest || node.App != "plexus" || node.Name != "sdk-e2e-ts" {
			t.Fatalf("unexpected node: %#v", node)
		}
	})
}
