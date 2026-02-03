package libraries

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/deps"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"
	"github.com/spf13/afero"
)

func TestFindGlobalLib(t *testing.T) {
	t.Run("domain__single_match__returns_language_from_config", func(t *testing.T) {
		root := t.TempDir()
		fs := afero.NewMemMapFs()
		createGlobalLibDir(t, fs, root, "go", "core")

		ok, language, err := FindGlobalLib(fs, root, "core")
		if err != nil {
			t.Fatalf("FindGlobalLib: %v", err)
		}
		if !ok {
			t.Fatalf("expected match")
		}
		if language != "go" {
			t.Fatalf("expected language go, got %s", language)
		}
	})

	t.Run("boundary__no_matches__returns_false", func(t *testing.T) {
		root := t.TempDir()
		fs := afero.NewMemMapFs()
		createLibsRoot(t, fs, root)

		ok, language, err := FindGlobalLib(fs, root, "missing")
		if err != nil {
			t.Fatalf("FindGlobalLib: %v", err)
		}
		if ok {
			t.Fatalf("expected no match")
		}
		if language != "" {
			t.Fatalf("expected empty language, got %s", language)
		}
	})

	t.Run("chaos__target_is_file__returns_no_match", func(t *testing.T) {
		root := t.TempDir()
		fs := afero.NewMemMapFs()
		libsRoot := filepath.Join(root, "repos", "libs")
		if err := fs.MkdirAll(libsRoot, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := afero.WriteFile(fs, filepath.Join(libsRoot, "core"), []byte("not-a-dir"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		ok, language, err := FindGlobalLib(fs, root, "core")
		if err != nil {
			t.Fatalf("FindGlobalLib: %v", err)
		}
		if ok {
			t.Fatalf("expected no match")
		}
		if language != "" {
			t.Fatalf("expected empty language, got %s", language)
		}
	})
}

func TestMakeAppLibNode(t *testing.T) {
	t.Run("domain__registered_library__returns_node", func(t *testing.T) {
		config := workspace.MakeConfig(
			nil,
			[]workspace.WorkspaceApp{
				workspace.MakeApp("alpha", []workspace.WorkspaceLibrary{
					workspace.MakeLibrary("lib-a", "dep-1"),
				}),
			},
			nil,
		)

		node, err := MakeAppLibNode(config, "alpha", "lib-a")
		if err != nil {
			t.Fatalf("MakeAppLibNode: %v", err)
		}

		assertAppLibNode(t, node, "alpha", "lib-a", []workspace.WorkspaceDep{
			{Lib: "dep-1", From: "global"},
		})
	})

	t.Run("boundary__empty_deps__returns_node_with_empty_deps", func(t *testing.T) {
		config := workspace.MakeConfig(
			nil,
			[]workspace.WorkspaceApp{
				workspace.MakeApp("beta", []workspace.WorkspaceLibrary{
					workspace.MakeLibrary("lib-b"),
				}),
			},
			nil,
		)

		node, err := MakeAppLibNode(config, "beta", "lib-b")
		if err != nil {
			t.Fatalf("MakeAppLibNode: %v", err)
		}

		assertAppLibNode(t, node, "beta", "lib-b", []workspace.WorkspaceDep{})
		if len(node.Deps) != 0 {
			t.Fatalf("expected no deps, got %d", len(node.Deps))
		}
	})

	t.Run("complement__missing_library__returns_error", func(t *testing.T) {
		config := workspace.MakeConfig(
			nil,
			[]workspace.WorkspaceApp{
				workspace.MakeApp("gamma", []workspace.WorkspaceLibrary{
					workspace.MakeLibrary("lib-c"),
				}),
			},
			nil,
		)

		_, err := MakeAppLibNode(config, "gamma", "missing")
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("chaos__missing_app__returns_error", func(t *testing.T) {
		config := workspace.MakeConfig(
			nil,
			[]workspace.WorkspaceApp{
				workspace.MakeApp("echo", []workspace.WorkspaceLibrary{
					workspace.MakeLibrary("lib-e"),
				}),
			},
			nil,
		)

		_, err := MakeAppLibNode(config, "missing", "lib-e")
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestMakeGlobalLibNode(t *testing.T) {
	t.Run("domain__registered_library__returns_node", func(t *testing.T) {
		config := workspace.MakeConfig(
			[]workspace.WorkspaceLibrary{
				workspace.MakeLibrary("lib-a", "dep-1"),
			},
			nil,
			nil,
		)

		node, err := MakeGlobalLibNode(config, "lib-a")
		if err != nil {
			t.Fatalf("MakeGlobalLibNode: %v", err)
		}

		assertGlobalLibNode(t, node, "lib-a", []workspace.WorkspaceDep{
			{Lib: "dep-1", From: "global"},
		})
	})

	t.Run("boundary__empty_deps__returns_node_with_empty_deps", func(t *testing.T) {
		config := workspace.MakeConfig(
			[]workspace.WorkspaceLibrary{
				workspace.MakeLibrary("lib-b"),
			},
			nil,
			nil,
		)

		node, err := MakeGlobalLibNode(config, "lib-b")
		if err != nil {
			t.Fatalf("MakeGlobalLibNode: %v", err)
		}

		assertGlobalLibNode(t, node, "lib-b", []workspace.WorkspaceDep{})
		if len(node.Deps) != 0 {
			t.Fatalf("expected no deps, got %d", len(node.Deps))
		}
	})

	t.Run("complement__missing_library__returns_error", func(t *testing.T) {
		config := workspace.MakeConfig(
			[]workspace.WorkspaceLibrary{
				workspace.MakeLibrary("lib-c"),
			},
			nil,
			nil,
		)

		_, err := MakeGlobalLibNode(config, "missing")
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("chaos__ambiguous_library_name__returns_error", func(t *testing.T) {
		config := workspace.MakeConfig(
			[]workspace.WorkspaceLibrary{
				workspace.MakeLibrary("lib-e"),
				workspace.MakeLibrary("lib-e"),
			},
			nil,
			nil,
		)

		_, err := MakeGlobalLibNode(config, "lib-e")
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestAddAppLibraryToWorkspace(t *testing.T) {
	t.Run("domain__new_app_library__adds_entry", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := workspace.WriteWorkspaceConfig(fs, workspace.WorkspaceConfig{
			Apps: []workspace.WorkspaceApp{
				{
					Name:      "alpha",
					Services:  []workspace.WorkspaceService{},
					Libraries: []workspace.WorkspaceLibrary{},
				},
			},
		}); err != nil {
			t.Fatalf("WriteWorkspaceConfig: %v", err)
		}

		if err := AddAppLibraryToWorkspace(fs, "alpha", "lib-a"); err != nil {
			t.Fatalf("AddAppLibraryToWorkspace: %v", err)
		}

		config, err := workspace.ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("ReadWorkspaceConfig: %v", err)
		}
		if len(config.Apps) != 1 || len(config.Apps[0].Libraries) != 1 || config.Apps[0].Libraries[0].Name != "lib-a" {
			t.Fatalf("expected app library lib-a to be registered")
		}
	})

	t.Run("boundary__missing_app__creates_app_entry", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := workspace.WriteWorkspaceConfig(fs, workspace.WorkspaceConfig{}); err != nil {
			t.Fatalf("WriteWorkspaceConfig: %v", err)
		}

		if err := AddAppLibraryToWorkspace(fs, "beta", "lib-b"); err != nil {
			t.Fatalf("AddAppLibraryToWorkspace: %v", err)
		}

		config, err := workspace.ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("ReadWorkspaceConfig: %v", err)
		}
		if len(config.Apps) != 1 || config.Apps[0].Name != "beta" {
			t.Fatalf("expected app beta to be registered")
		}
		if len(config.Apps[0].Libraries) != 1 || config.Apps[0].Libraries[0].Name != "lib-b" {
			t.Fatalf("expected app library lib-b to be registered")
		}
	})

	t.Run("boundary__existing_app_library__no_duplicate", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := workspace.WriteWorkspaceConfig(fs, workspace.WorkspaceConfig{
			Apps: []workspace.WorkspaceApp{
				{
					Name: "gamma",
					Libraries: []workspace.WorkspaceLibrary{
						workspace.MakeLibrary("lib-c"),
					},
				},
			},
		}); err != nil {
			t.Fatalf("WriteWorkspaceConfig: %v", err)
		}

		if err := AddAppLibraryToWorkspace(fs, "gamma", "lib-c"); err != nil {
			t.Fatalf("AddAppLibraryToWorkspace: %v", err)
		}

		config, err := workspace.ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("ReadWorkspaceConfig: %v", err)
		}
		if len(config.Apps) != 1 || len(config.Apps[0].Libraries) != 1 {
			t.Fatalf("expected no duplicate app libraries")
		}
	})
}

func TestAddGlobalLibraryToWorkspace(t *testing.T) {
	t.Run("domain__new_global_library__adds_entry", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := workspace.WriteWorkspaceConfig(fs, workspace.WorkspaceConfig{
			Libraries: []workspace.WorkspaceLibrary{},
		}); err != nil {
			t.Fatalf("WriteWorkspaceConfig: %v", err)
		}

		if err := AddGlobalLibraryToWorkspace(fs, "lib-a"); err != nil {
			t.Fatalf("AddGlobalLibraryToWorkspace: %v", err)
		}

		config, err := workspace.ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("ReadWorkspaceConfig: %v", err)
		}
		if len(config.Libraries) != 1 || config.Libraries[0].Name != "lib-a" {
			t.Fatalf("expected global library lib-a to be registered")
		}
	})

	t.Run("boundary__existing_global_library__no_duplicate", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := workspace.WriteWorkspaceConfig(fs, workspace.WorkspaceConfig{
			Libraries: []workspace.WorkspaceLibrary{
				workspace.MakeLibrary("lib-a"),
				workspace.MakeLibrary("lib-b"),
			},
		}); err != nil {
			t.Fatalf("WriteWorkspaceConfig: %v", err)
		}

		if err := AddGlobalLibraryToWorkspace(fs, "lib-a"); err != nil {
			t.Fatalf("AddGlobalLibraryToWorkspace: %v", err)
		}

		config, err := workspace.ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("ReadWorkspaceConfig: %v", err)
		}
		if len(config.Libraries) != 2 {
			t.Fatalf("expected no duplicate global libraries")
		}
	})
}

func TestCollectLibraryDependents(t *testing.T) {
	t.Run("domain__global_library_dependents__returns_targets", func(t *testing.T) {
		config := workspace.WorkspaceConfig{
			Apps: []workspace.WorkspaceApp{
				{
					Name: "alpha",
					Services: []workspace.WorkspaceService{
						{
							Name: "api",
							Deps: []workspace.WorkspaceDep{
								{Lib: "lib-a", From: "global"},
							},
						},
					},
					Libraries: []workspace.WorkspaceLibrary{
						{
							Name: "lib-app",
							Deps: []workspace.WorkspaceDep{
								{Lib: "lib-a", From: "global"},
							},
						},
					},
				},
			},
			Libraries: []workspace.WorkspaceLibrary{
				{
					Name: "lib-consumer",
					Deps: []workspace.WorkspaceDep{
						{Lib: "lib-a", From: "global"},
					},
				},
			},
			Projects: []workspace.WorkspaceProject{
				{
					Name: "proj",
					Deps: []workspace.WorkspaceDep{
						{Lib: "lib-a", From: "global"},
					},
				},
			},
		}

		targets := CollectLibraryDependents(config, "/root", "lib-a", "global")
		if len(targets) != 4 {
			t.Fatalf("expected 4 dependents, got %d", len(targets))
		}

		expected := map[string]workspace.TargetKind{
			"/root/repos/apps/alpha/services/api": workspace.TargetService,
			"/root/repos/apps/alpha/libs/lib-app": workspace.TargetAppLib,
			"/root/repos/libs/lib-consumer":       workspace.TargetGlobalLib,
			"/root/repos/projects/proj":           workspace.TargetProject,
		}
		for _, target := range targets {
			if kind, ok := expected[target.Path]; !ok || kind != target.Kind {
				t.Fatalf("unexpected target %s (%s)", target.Path, target.Kind)
			}
		}
	})
}

func TestRemoveLibraryDeps(t *testing.T) {
	t.Run("domain__matching_deps__removed", func(t *testing.T) {
		config := workspace.WorkspaceConfig{
			Apps: []workspace.WorkspaceApp{
				{
					Name: "alpha",
					Services: []workspace.WorkspaceService{
						{
							Name: "api",
							Deps: []workspace.WorkspaceDep{
								{Lib: "lib-a", From: "global"},
								{Lib: "lib-b", From: "global"},
							},
						},
					},
					Libraries: []workspace.WorkspaceLibrary{
						{
							Name: "lib-app",
							Deps: []workspace.WorkspaceDep{
								{Lib: "lib-a", From: "global"},
								{Lib: "lib-c", From: "global"},
							},
						},
					},
				},
			},
			Libraries: []workspace.WorkspaceLibrary{
				{
					Name: "lib-consumer",
					Deps: []workspace.WorkspaceDep{
						{Lib: "lib-a", From: "global"},
					},
				},
			},
			Projects: []workspace.WorkspaceProject{
				{
					Name: "proj",
					Deps: []workspace.WorkspaceDep{
						{Lib: "lib-a", From: "global"},
						{Lib: "lib-d", From: "global"},
					},
				},
			},
		}

		updated := RemoveLibraryDeps(config, "lib-a", "global")
		if len(updated.Apps[0].Services[0].Deps) != 1 || updated.Apps[0].Services[0].Deps[0].Lib != "lib-b" {
			t.Fatalf("expected lib-a removed from service deps")
		}
		if len(updated.Apps[0].Libraries[0].Deps) != 1 || updated.Apps[0].Libraries[0].Deps[0].Lib != "lib-c" {
			t.Fatalf("expected lib-a removed from app library deps")
		}
		if len(updated.Libraries[0].Deps) != 0 {
			t.Fatalf("expected lib-a removed from global library deps")
		}
		if len(updated.Projects[0].Deps) != 1 || updated.Projects[0].Deps[0].Lib != "lib-d" {
			t.Fatalf("expected lib-a removed from project deps")
		}
	})
}

func TestRemoveGlobalLibraryFromWorkspace(t *testing.T) {
	t.Run("domain__existing_library__removes_entry", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := workspace.WriteWorkspaceConfig(fs, workspace.WorkspaceConfig{
			Libraries: []workspace.WorkspaceLibrary{
				workspace.MakeLibrary("lib-a"),
				workspace.MakeLibrary("lib-b"),
			},
		}); err != nil {
			t.Fatalf("WriteWorkspaceConfig: %v", err)
		}

		if err := RemoveGlobalLibraryFromWorkspace(fs, "lib-a"); err != nil {
			t.Fatalf("RemoveGlobalLibraryFromWorkspace: %v", err)
		}

		config, err := workspace.ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("ReadWorkspaceConfig: %v", err)
		}
		if len(config.Libraries) != 1 || config.Libraries[0].Name != "lib-b" {
			t.Fatalf("expected remaining library to be lib-b")
		}
	})
}

func TestRemoveAppLibraryFromWorkspace(t *testing.T) {
	t.Run("domain__existing_app_library__removes_entry", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := workspace.WriteWorkspaceConfig(fs, workspace.WorkspaceConfig{
			Apps: []workspace.WorkspaceApp{
				{
					Name: "alpha",
					Libraries: []workspace.WorkspaceLibrary{
						workspace.MakeLibrary("lib-a"),
						workspace.MakeLibrary("lib-b"),
					},
				},
			},
		}); err != nil {
			t.Fatalf("WriteWorkspaceConfig: %v", err)
		}

		if err := RemoveAppLibraryFromWorkspace(fs, "alpha", "lib-a"); err != nil {
			t.Fatalf("RemoveAppLibraryFromWorkspace: %v", err)
		}

		config, err := workspace.ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("ReadWorkspaceConfig: %v", err)
		}
		if len(config.Apps) != 1 || len(config.Apps[0].Libraries) != 1 || config.Apps[0].Libraries[0].Name != "lib-b" {
			t.Fatalf("expected remaining app library to be lib-b")
		}
	})
}

func assertAppLibNode(t *testing.T, node deps.Node, appName string, name string, depsList []workspace.WorkspaceDep) {
	t.Helper()
	if node.Kind != deps.NodeAppLib {
		t.Fatalf("expected kind %s, got %s", deps.NodeAppLib, node.Kind)
	}
	if node.App != appName {
		t.Fatalf("expected app %s, got %s", appName, node.App)
	}
	if node.Name != name {
		t.Fatalf("expected name %s, got %s", name, node.Name)
	}
	if !reflect.DeepEqual(node.Deps, depsList) {
		t.Fatalf("expected deps %v, got %v", depsList, node.Deps)
	}
}

func assertGlobalLibNode(t *testing.T, node deps.Node, name string, depsList []workspace.WorkspaceDep) {
	t.Helper()
	if node.Kind != deps.NodeGlobalLib {
		t.Fatalf("expected kind %s, got %s", deps.NodeGlobalLib, node.Kind)
	}
	if node.Name != name {
		t.Fatalf("expected name %s, got %s", name, node.Name)
	}
	if !reflect.DeepEqual(node.Deps, depsList) {
		t.Fatalf("expected deps %v, got %v", depsList, node.Deps)
	}
}

func createLibsRoot(t *testing.T, fs afero.Fs, root string) {
	t.Helper()
	if err := fs.MkdirAll(filepath.Join(root, "repos", "libs"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
}

func createGlobalLibDir(t *testing.T, fs afero.Fs, root string, language string, name string) {
	t.Helper()
	path := filepath.Join(root, "repos", "libs", name)
	if err := fs.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	payload := []byte("{\"language\": \"" + language + "\"}\n")
	if err := afero.WriteFile(fs, filepath.Join(path, "ocean.config.json"), payload, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
