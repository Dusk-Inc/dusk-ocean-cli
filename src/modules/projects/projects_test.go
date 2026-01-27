package projects

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/deps"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"
	"github.com/spf13/afero"
)

func TestMakeProjectNode(t *testing.T) {
	t.Run("domain__registered_project__returns_project_node", func(t *testing.T) {
		config := workspace.MakeConfig(
			nil,
			nil,
			[]workspace.WorkspaceProject{
				workspace.MakeProject("alpha",  "lib-a", "lib-b"),
			},
		)

		node, err := MakeProjectNode(config, "alpha")
		if err != nil {
			t.Fatalf("MakeProjectNode: %v", err)
		}

		assertProjectNode(t, node, "alpha", []string{"lib-a", "lib-b"})
	})

	t.Run("boundary__empty_deps__returns_node_with_empty_deps", func(t *testing.T) {
		config := workspace.MakeConfig(
			nil,
			nil,
			[]workspace.WorkspaceProject{
				workspace.MakeProject("beta"),
			},
		)

		node, err := MakeProjectNode(config, "beta")
		if err != nil {
			t.Fatalf("MakeProjectNode: %v", err)
		}

		assertProjectNode(t, node, "beta", nil)
		if len(node.Deps) != 0 {
			t.Fatalf("expected no deps, got %d", len(node.Deps))
		}
	})

	t.Run("complement__missing_project__returns_error", func(t *testing.T) {
		config := workspace.MakeConfig(
			nil,
			nil,
			[]workspace.WorkspaceProject{
				workspace.MakeProject("gamma"),
			},
		)

		_, err := MakeProjectNode(config, "missing")
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("chaos__ambiguous_project_name__returns_error", func(t *testing.T) {
		config := workspace.MakeConfig(
			nil,
			nil,
			[]workspace.WorkspaceProject{
				workspace.MakeProject("echo"),
				workspace.MakeProject("echo"),
			},
		)

		_, err := MakeProjectNode(config, "echo")
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestFindProjectLanguage(t *testing.T) {
	t.Run("domain__single_project__returns_language", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/"
		createProjectDir(t, fs, root, "go", "ocean")

		ok, language, err := FindProjectLanguage(fs, root, "ocean")
		if err != nil {
			t.Fatalf("FindProjectLanguage: %v", err)
		}
		if !ok {
			t.Fatalf("expected match")
		}
		if language != "go" {
			t.Fatalf("expected language go, got %s", language)
		}
	})

	t.Run("boundary__no_project_matches__returns_false", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/"
		createProjectsRoot(t, fs, root)

		ok, language, err := FindProjectLanguage(fs, root, "missing")
		if err != nil {
			t.Fatalf("FindProjectLanguage: %v", err)
		}
		if ok {
			t.Fatalf("expected no match")
		}
		if language != "" {
			t.Fatalf("expected empty language, got %s", language)
		}
	})

	t.Run("chaos__project_path_is_file__returns_no_match", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/"
		path := filepath.Join(root, "repos", "projects", "file-project")
		if err := fs.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := afero.WriteFile(fs, path, []byte("not-a-dir"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		ok, language, err := FindProjectLanguage(fs, root, "file-project")
		if err != nil {
			t.Fatalf("FindProjectLanguage: %v", err)
		}
		if ok {
			t.Fatalf("expected no match")
		}
		if language != "" {
			t.Fatalf("expected empty language, got %s", language)
		}
	})
}

func TestRemoveProjectFromWorkspace(t *testing.T) {
	t.Run("domain__existing_project__removes_entry", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := workspace.WriteWorkspaceConfig(fs, workspace.WorkspaceConfig{
			Projects: []workspace.WorkspaceProject{
				workspace.MakeProject("alpha"),
				workspace.MakeProject("beta"),
			},
		}); err != nil {
			t.Fatalf("WriteWorkspaceConfig: %v", err)
		}

		if err := RemoveProjectFromWorkspace(fs, "alpha"); err != nil {
			t.Fatalf("RemoveProjectFromWorkspace: %v", err)
		}

		config, err := workspace.ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("ReadWorkspaceConfig: %v", err)
		}
		if len(config.Projects) != 1 || config.Projects[0].Name != "beta" {
			t.Fatalf("expected remaining project to be beta")
		}
	})
}

func assertProjectNode(t *testing.T, node deps.Node, name string, depsList []string) {
	t.Helper()
	if node.Kind != deps.NodeProject {
		t.Fatalf("expected kind %s, got %s", deps.NodeProject, node.Kind)
	}
	if node.Name != name {
		t.Fatalf("expected name %s, got %s", name, node.Name)
	}
	if !reflect.DeepEqual(node.Deps, depsList) {
		t.Fatalf("expected deps %v, got %v", depsList, node.Deps)
	}
}

func createProjectsRoot(t *testing.T, fs afero.Fs, root string) {
	t.Helper()
	if err := fs.MkdirAll(filepath.Join(root, "repos", "projects"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
}

func createProjectDir(t *testing.T, fs afero.Fs, root string, language string, name string) {
	t.Helper()
	path := filepath.Join(root, "repos", "projects", name)
	if err := fs.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	payload := []byte("{\"language\": \"" + language + "\"}\n")
	if err := afero.WriteFile(fs, filepath.Join(path, "ocean.config.json"), payload, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
