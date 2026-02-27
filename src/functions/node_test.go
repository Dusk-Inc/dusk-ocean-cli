package functions

import (
	"testing"

)

func TestMakeTestNode(t *testing.T) {
	t.Run("domain__registered_test__returns_node", func(t *testing.T) {
		config := WorkspaceConfig{
			Apps: []WorkspaceApp{
				{
					Name: "plexus",
					Testing: []WorkspaceTest{
						{
							Name: "sdk-e2e-ts",
							Deps: []WorkspaceDep{{Lib: "plexus-core", From: "plexus"}},
						},
					},
				},
			},
		}

		node, err := MakeTestNode(config, "plexus", "sdk-e2e-ts")
		if err != nil {
			t.Fatalf("MakeTestNode: %v", err)
		}
		if node.Kind != NodeAppTest || node.App != "plexus" || node.Name != "sdk-e2e-ts" {
			t.Fatalf("unexpected node: %#v", node)
		}
	})
}
