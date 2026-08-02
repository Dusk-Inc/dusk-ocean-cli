package functions

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
)

func TestAddAppProjectToWorkspace(t *testing.T) {
	t.Run("domain__existing_app__appends_project", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		writeWorkspaceConfig(t, fs, WorkspaceConfig{})
		if err := addAppToWorkspace(fs, "alpha"); err != nil {
			t.Fatalf("seed: %v", err)
		}

		if err := AddAppProjectToWorkspace(fs, "alpha", "cli"); err != nil {
			t.Fatalf("AddAppProjectToWorkspace: %v", err)
		}

		config := readWorkspaceConfig(t, fs)
		appIdx := FindAppIndex(config, "alpha")
		if appIdx == -1 {
			t.Fatalf("alpha not registered")
		}
		if FindAppProjectIndex(config.Apps[appIdx], "cli") == -1 {
			t.Fatalf("expected cli registered, got %+v", config.Apps[appIdx].Projects)
		}
		if config.Apps[appIdx].Projects[0].Deps == nil {
			t.Fatalf("expected initialized Deps slice")
		}
	})

	t.Run("domain__unregistered_app__creates_app_entry", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		writeWorkspaceConfig(t, fs, WorkspaceConfig{})

		if err := AddAppProjectToWorkspace(fs, "beta", "cli"); err != nil {
			t.Fatalf("AddAppProjectToWorkspace: %v", err)
		}

		config := readWorkspaceConfig(t, fs)
		appIdx := FindAppIndex(config, "beta")
		if appIdx == -1 {
			t.Fatalf("expected the app entry to be created")
		}
		if FindAppProjectIndex(config.Apps[appIdx], "cli") == -1 {
			t.Fatalf("expected cli registered under the new app")
		}
	})

	t.Run("boundary__duplicate__is_no_op", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		writeWorkspaceConfig(t, fs, WorkspaceConfig{})
		if err := AddAppProjectToWorkspace(fs, "alpha", "cli"); err != nil {
			t.Fatalf("seed: %v", err)
		}
		if err := AddAppProjectToWorkspace(fs, "alpha", "cli"); err != nil {
			t.Fatalf("duplicate add should no-op: %v", err)
		}

		config := readWorkspaceConfig(t, fs)
		appIdx := FindAppIndex(config, "alpha")
		if len(config.Apps[appIdx].Projects) != 1 {
			t.Fatalf("expected a single entry, got %+v", config.Apps[appIdx].Projects)
		}
	})
}

func TestRemoveAppProjectFromWorkspace(t *testing.T) {
	t.Run("domain__registered__removes_entry", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		writeWorkspaceConfig(t, fs, WorkspaceConfig{})
		if err := AddAppProjectToWorkspace(fs, "alpha", "cli"); err != nil {
			t.Fatalf("seed: %v", err)
		}

		if err := RemoveAppProjectFromWorkspace(fs, "alpha", "cli"); err != nil {
			t.Fatalf("RemoveAppProjectFromWorkspace: %v", err)
		}

		config := readWorkspaceConfig(t, fs)
		appIdx := FindAppIndex(config, "alpha")
		if FindAppProjectIndex(config.Apps[appIdx], "cli") != -1 {
			t.Fatalf("expected cli removed")
		}
	})

	t.Run("boundary__unknown_app_or_project__is_no_op", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		writeWorkspaceConfig(t, fs, WorkspaceConfig{})
		if err := RemoveAppProjectFromWorkspace(fs, "ghost", "cli"); err != nil {
			t.Fatalf("unknown app should no-op: %v", err)
		}
		if err := addAppToWorkspace(fs, "alpha"); err != nil {
			t.Fatalf("seed: %v", err)
		}
		if err := RemoveAppProjectFromWorkspace(fs, "alpha", "ghost"); err != nil {
			t.Fatalf("unknown project should no-op: %v", err)
		}
	})
}

func TestMakeAppProjectNode(t *testing.T) {
	t.Run("domain__registered__builds_node", func(t *testing.T) {
		config := WorkspaceConfig{
			Apps: []WorkspaceApp{
				{
					Name:     "alpha",
					Projects: []WorkspaceProject{{Name: "cli", Deps: []WorkspaceDep{}}},
				},
			},
		}

		node, err := MakeAppProjectNode(config, "alpha", "cli")
		if err != nil {
			t.Fatalf("MakeAppProjectNode: %v", err)
		}
		if node.Kind != NodeAppProject || node.App != "alpha" || node.Name != "cli" {
			t.Fatalf("unexpected node: %+v", node)
		}
	})

	t.Run("complement__unregistered__errors", func(t *testing.T) {
		if _, err := MakeAppProjectNode(WorkspaceConfig{}, "alpha", "cli"); err == nil {
			t.Fatalf("expected an error for an unregistered app project")
		}
	})
}

func TestAppProjectKeyIsDistinctFromGlobalProject(t *testing.T) {
	if AppProjectKey("alpha", "cli") == ProjectKey("cli") {
		t.Fatalf("app-scoped and global project keys must not collide")
	}
}

func TestCollectWorkspaceNodes_IncludesAppProjects(t *testing.T) {
	config := WorkspaceConfig{
		Apps: []WorkspaceApp{
			{
				Name:     "alpha",
				Projects: []WorkspaceProject{{Name: "cli", Deps: []WorkspaceDep{}}},
			},
		},
	}

	nodes := CollectWorkspaceNodes(config)
	if len(nodes) != 1 {
		t.Fatalf("expected the app project in the graph, got %+v", nodes)
	}
	if nodes[0].Kind != NodeAppProject || nodes[0].App != "alpha" {
		t.Fatalf("unexpected node: %+v", nodes[0])
	}
}

func TestNodeBuildInfo_AppProject(t *testing.T) {
	label, path, hashPath, err := NodeBuildInfo("/root", Node{Kind: NodeAppProject, App: "alpha", Name: "cli"})
	if err != nil {
		t.Fatalf("NodeBuildInfo: %v", err)
	}
	if label != "project alpha/cli" {
		t.Errorf("unexpected label: %q", label)
	}
	if path != filepath.Join("/root", "repos", "apps", "alpha", "projects", "cli") {
		t.Errorf("unexpected path: %q", path)
	}
	if hashPath != MakeHashPath("/root", "projects", "alpha", "cli") {
		t.Errorf("unexpected hash path: %q", hashPath)
	}
}

func TestValidateTargetRegistration_AppProject(t *testing.T) {
	config := WorkspaceConfig{
		Apps: []WorkspaceApp{
			{Name: "alpha", Projects: []WorkspaceProject{{Name: "cli"}}},
		},
	}

	if err := ValidateTargetRegistration(Target{Kind: TargetAppProject, App: "alpha", Name: "cli"}, config); err != nil {
		t.Fatalf("expected registered app project to validate: %v", err)
	}
	if err := ValidateTargetRegistration(Target{Kind: TargetAppProject, App: "alpha", Name: "ghost"}, config); err == nil {
		t.Fatalf("expected an unregistered app project to fail")
	}
	if err := ValidateTargetRegistration(Target{Kind: TargetAppProject, App: "ghost", Name: "cli"}, config); err == nil {
		t.Fatalf("expected an unknown app to fail")
	}
}

func TestVcsTaskTarget_AppProjectRejected(t *testing.T) {
	if _, err := vcsTaskTarget("project", "cli", "alpha"); err == nil {
		t.Fatalf("app-scoped projects inherit the app's history; VCS wiring must be rejected")
	}
	if _, err := vcsTaskTarget("project", "tooling", ""); err != nil {
		t.Fatalf("global projects still wire VCS: %v", err)
	}
}

func TestValidateInstallFlow_AppProject(t *testing.T) {
	target := installTarget{Kind: TargetAppProject, App: "alpha", Name: "cli"}

	t.Run("domain__same_app_lib__accepted", func(t *testing.T) {
		dep := installDependency{kind: dependencyAppLib, app: "alpha", name: "contracts"}
		if err := validateInstallFlow(target, dep); err != nil {
			t.Fatalf("expected an app project to accept its own app's library: %v", err)
		}
	})

	t.Run("domain__global_lib__accepted", func(t *testing.T) {
		dep := installDependency{kind: dependencyGlobalLib, name: "shared"}
		if err := validateInstallFlow(target, dep); err != nil {
			t.Fatalf("expected an app project to accept a global library: %v", err)
		}
	})

	t.Run("complement__other_app_lib__rejected", func(t *testing.T) {
		dep := installDependency{kind: dependencyAppLib, app: "beta", name: "contracts"}
		if err := validateInstallFlow(target, dep); err == nil {
			t.Fatalf("expected a cross-app library to be rejected")
		}
	})

	t.Run("complement__project_dep__rejected", func(t *testing.T) {
		dep := installDependency{kind: dependencyProject, name: "tooling"}
		if err := validateInstallFlow(target, dep); err == nil {
			t.Fatalf("expected a project to be rejected as a dependency")
		}
	})
}
