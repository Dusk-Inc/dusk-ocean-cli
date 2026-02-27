package functions

import (
	"testing"

	"github.com/spf13/afero"
)

func TestResolveTarget(t *testing.T) {
	t.Run("domain__app_testing_path__returns_test_target", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/workspace"
		testPath := "/workspace/repos/apps/plexus/testing/sdk-e2e-ts"
		if err := fs.MkdirAll(testPath, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}

		target, err := ResolveTarget(fs, root, testPath)
		if err != nil {
			t.Fatalf("ResolveTarget: %v", err)
		}
		if target.Kind != TargetTest || target.App != "plexus" || target.Name != "sdk-e2e-ts" {
			t.Fatalf("unexpected target: %#v", target)
		}
	})

	t.Run("complement__invalid_path__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/workspace"
		if err := fs.MkdirAll("/workspace/repos/apps/plexus/testing", 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		_, err := ResolveTarget(fs, root, "/workspace/repos/apps/plexus/testing")
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestTestWorkspaceRegistration(t *testing.T) {
	t.Run("domain__add_and_remove_test__updates_workspace", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := WriteWorkspaceConfig(fs, WorkspaceConfig{
			Apps: []WorkspaceApp{
				{Name: "plexus", Services: []WorkspaceService{}, Libraries: []WorkspaceLibrary{}, Testing: []WorkspaceTest{}},
			},
		}); err != nil {
			t.Fatalf("WriteWorkspaceConfig: %v", err)
		}

		if err := AddTestToWorkspace(fs, "plexus", "sdk-e2e-ts"); err != nil {
			t.Fatalf("AddTestToWorkspace: %v", err)
		}
		config, err := ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("ReadWorkspaceConfig: %v", err)
		}
		if len(config.Apps[0].Testing) != 1 || config.Apps[0].Testing[0].Name != "sdk-e2e-ts" {
			t.Fatalf("expected test registration")
		}

		if err := RemoveTestFromWorkspace(fs, "plexus", "sdk-e2e-ts"); err != nil {
			t.Fatalf("RemoveTestFromWorkspace: %v", err)
		}
		config, err = ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("ReadWorkspaceConfig: %v", err)
		}
		if len(config.Apps[0].Testing) != 0 {
			t.Fatalf("expected test removal")
		}
	})
}
