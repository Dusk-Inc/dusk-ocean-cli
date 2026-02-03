package deps

import (
	"bytes"
	"os/exec"
	"reflect"
	"testing"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

func TestCollectDependencyOrder(t *testing.T) {
	t.Run("domain__linear_deps__returns_topological_order", func(t *testing.T) {
		config := workspace.MakeConfig(
			[]workspace.WorkspaceLibrary{
				workspace.MakeLibrary("lib-a", "lib-b"),
				workspace.MakeLibrary("lib-b"),
			},
			nil,
			[]workspace.WorkspaceProject{
				workspace.MakeProject("project-x", "lib-a"),
			},
		)
		dep := workspace.WorkspaceDep{
			Lib:  "lib-a",
			From: "global",
		}
		target := makeNode(NodeProject, "", "project-x", dep)

		order, err := CollectDependencyOrder(config, target)
		if err != nil {
			t.Fatalf("CollectDependencyOrder: %v", err)
		}
		if len(order) != 2 {
			t.Fatalf("expected 2 dependencies, got %d", len(order))
		}
		assertOrderInvariant(t, target, order)

		keys := nodeKeys(order)
		expected := []string{
			GlobalLibKey("lib-b"),
			GlobalLibKey("lib-a"),
		}
		if keys[0] != expected[0] || keys[1] != expected[1] {
			t.Fatalf("unexpected order: %v", keys)
		}
	})

	t.Run("boundary__no_deps__returns_empty", func(t *testing.T) {
		config := workspace.MakeConfig(
			[]workspace.WorkspaceLibrary{
				workspace.MakeLibrary("lib-a"),
			},
			nil,
			nil,
		)
		target := makeNode(NodeGlobalLib, "", "lib-a")

		order, err := CollectDependencyOrder(config, target)
		if err != nil {
			t.Fatalf("CollectDependencyOrder: %v", err)
		}
		if len(order) != 0 {
			t.Fatalf("expected empty order, got %d", len(order))
		}
		assertOrderInvariant(t, target, order)
	})

	t.Run("complement__missing_dependency__returns_error", func(t *testing.T) {
		config := workspace.MakeConfig(nil, nil, nil)
		target := makeNode(NodeProject, "", "project-x", workspace.WorkspaceDep{
			Lib:  "missing-lib",
			From: "global",
		})

		if _, err := CollectDependencyOrder(config, target); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("chaos__whitespace_dependency__returns_error", func(t *testing.T) {
		config := workspace.MakeConfig(nil, nil, nil)
		target := makeNode(NodeProject, "", "project-x", workspace.WorkspaceDep{
			Lib:  "  ",
			From: "global",
		})

		if _, err := CollectDependencyOrder(config, target); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestHasPath(t *testing.T) {
	t.Run("domain__indirect_path__returns_true", func(t *testing.T) {
		graph := map[string][]string{
			"a": {"b"},
			"b": {"c"},
			"c": {},
		}

		if !HasPath(graph, "a", "c") {
			t.Fatalf("expected path to exist")
		}
	})

	t.Run("boundary__start_equals_target__returns_true", func(t *testing.T) {
		graph := map[string][]string{}

		if !HasPath(graph, "a", "a") {
			t.Fatalf("expected path to exist")
		}
	})

	t.Run("complement__no_path__returns_false", func(t *testing.T) {
		graph := map[string][]string{
			"a": {"b"},
			"b": {},
			"c": {},
		}

		if HasPath(graph, "a", "c") {
			t.Fatalf("expected no path to exist")
		}
	})

	t.Run("chaos__cycle_graph__returns_true_without_looping", func(t *testing.T) {
		graph := map[string][]string{
			"a": {"b"},
			"b": {"a", "c"},
			"c": {},
		}

		if !HasPath(graph, "a", "c") {
			t.Fatalf("expected path to exist")
		}
	})
}

func TestRunUninstallForTargets(t *testing.T) {
	t.Run("domain__targets__runs_uninstall_per_target", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		cmd := &cobra.Command{}
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		targets := []workspace.Target{
			{Path: "/repo/apps/app/services/a"},
			{Path: "/repo/apps/app/services/b"},
		}
		runDirs := []string{}
		readCount := 0

		err := RunUninstallForTargets(cmd, fs, "/repo/repos/libs/lib-a", "lib-a", targets, UninstallOptions{
			ReadRepoCommand: func(fs afero.Fs, root string, kind string) (string, error) {
				readCount++
				return "echo ok", nil
			},
			RunCommand: func(command *exec.Cmd) error {
				runDirs = append(runDirs, command.Dir)
				return nil
			},
		})
		if err != nil {
			t.Fatalf("RunUninstallForTargets: %v", err)
		}
		if readCount != 1 {
			t.Fatalf("expected read command once, got %d", readCount)
		}
		if !reflect.DeepEqual(runDirs, []string{targets[0].Path, targets[1].Path}) {
			t.Fatalf("unexpected run dirs: %v", runDirs)
		}
	})

	t.Run("complement__missing_uninstall_command__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		cmd := &cobra.Command{}
		runCount := 0

		err := RunUninstallForTargets(cmd, fs, "/repo/repos/libs/lib-a", "lib-a", []workspace.Target{{Path: "/repo/apps/app/services/a"}}, UninstallOptions{
			ReadRepoCommand: func(fs afero.Fs, root string, kind string) (string, error) {
				return " ", nil
			},
			RunCommand: func(command *exec.Cmd) error {
				runCount++
				return nil
			},
		})
		if err == nil {
			t.Fatalf("expected error")
		}
		if runCount != 0 {
			t.Fatalf("expected no command execution")
		}
	})
}

func TestResolveDependencyNode(t *testing.T) {
	t.Run("domain__app_lib_dependency__returns_app_lib", func(t *testing.T) {
		config := workspace.MakeConfig(
			[]workspace.WorkspaceLibrary{
				workspace.MakeLibrary("shared"),
			},
			[]workspace.WorkspaceApp{
				workspace.MakeApp("app", []workspace.WorkspaceLibrary{
					workspace.MakeLibrary("lib-a"),
				}),
			},
			nil,
		)
		dep := workspace.WorkspaceDep{
			Lib:  "lib-a",
			From: "app",
		}
		target := makeNode(NodeService, "app", "svc", dep)

		node, err := resolveDependencyNode(config, target, dep)
		if err != nil {
			t.Fatalf("resolveDependencyNode: %v", err)
		}
		if node.Kind != NodeAppLib || node.App != "app" || node.Name != "lib-a" {
			t.Fatalf("unexpected node: %#v", node)
		}
	})

	t.Run("boundary__trimmed_dependency_name__resolves", func(t *testing.T) {
		config := workspace.MakeConfig(
			[]workspace.WorkspaceLibrary{
				workspace.MakeLibrary("lib-a"),
			},
			nil,
			nil,
		)
		dep := workspace.WorkspaceDep{
			Lib:  " lib-a ",
			From: "global",
		}
		target := makeNode(NodeProject, "", "project-x", dep)

		node, err := resolveDependencyNode(config, target, dep)
		if err != nil {
			t.Fatalf("resolveDependencyNode: %v", err)
		}
		if node.Kind != NodeGlobalLib || node.Name != "lib-a" {
			t.Fatalf("unexpected node: %#v", node)
		}
	})

	t.Run("complement__missing_dependency__returns_error", func(t *testing.T) {
		config := workspace.MakeConfig(nil, nil, nil)
		dep := workspace.WorkspaceDep{
			Lib:  "missing",
			From: "global",
		}
		target := makeNode(NodeProject, "", "project-x", dep)

		if _, err := resolveDependencyNode(config, target, dep); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("chaos__ambiguous_dependency__resolves_with_source", func(t *testing.T) {
		config := workspace.MakeConfig(
			[]workspace.WorkspaceLibrary{
				workspace.MakeLibrary("lib-a"),
			},
			[]workspace.WorkspaceApp{
				workspace.MakeApp("app", []workspace.WorkspaceLibrary{
					workspace.MakeLibrary("lib-a"),
				}),
			},
			[]workspace.WorkspaceProject{
				workspace.MakeProject("lib-a"),
			},
		)
		dep := workspace.WorkspaceDep{
			Lib:  "lib-a",
			From: "app",
		}
		target := makeNode(NodeService, "app", "svc", dep)

		node, err := resolveDependencyNode(config, target, dep)
		if err != nil {
			t.Fatalf("resolveDependencyNode: %v", err)
		}
		if node.Kind != NodeAppLib || node.App != "app" || node.Name != "lib-a" {
			t.Fatalf("unexpected node: %#v", node)
		}
	})
}

func TestBuildDependencyGraph(t *testing.T) {
	t.Run("domain__mixed_dependencies__builds_graph", func(t *testing.T) {
		config := workspace.MakeConfig(
			[]workspace.WorkspaceLibrary{
				workspace.MakeLibrary("g-lib"),
				workspace.MakeLibrary("g-dep"),
			},
			[]workspace.WorkspaceApp{
				workspace.MakeApp("app", []workspace.WorkspaceLibrary{
					{
						Name: "a-lib",
						Deps: []workspace.WorkspaceDep{
							{Lib: "g-lib", From: "global"},
							{Lib: "project-x", From: "project"},
						},
					},
					workspace.MakeLibrary("a-dep"),
				}),
			},
			[]workspace.WorkspaceProject{
				workspace.MakeProject("project-x", "g-dep"),
			},
		)

		graph, err := BuildDependencyGraph(config)
		if err != nil {
			t.Fatalf("BuildDependencyGraph: %v", err)
		}

		assertGraphEntry(t, graph, GlobalLibKey("g-lib"), []string{})
		assertGraphEntry(t, graph, GlobalLibKey("g-dep"), []string{})
		assertGraphEntry(t, graph, AppLibKey("app", "a-lib"), []string{
			GlobalLibKey("g-lib"),
			ProjectKey("project-x"),
		})
		assertGraphEntry(t, graph, AppLibKey("app", "a-dep"), []string{})
		assertGraphEntry(t, graph, ProjectKey("project-x"), []string{
			GlobalLibKey("g-dep"),
		})
	})

	t.Run("boundary__empty_config__returns_empty", func(t *testing.T) {
		config := workspace.MakeConfig(nil, nil, nil)

		graph, err := BuildDependencyGraph(config)
		if err != nil {
			t.Fatalf("BuildDependencyGraph: %v", err)
		}
		if len(graph) != 0 {
			t.Fatalf("expected empty graph")
		}
	})

	t.Run("complement__global_name_collision__returns_error", func(t *testing.T) {
		config := workspace.MakeConfig(
			[]workspace.WorkspaceLibrary{
				workspace.MakeLibrary("shared"),
				workspace.MakeLibrary("shared"),
			},
			nil,
			nil,
		)

		if _, err := BuildDependencyGraph(config); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("chaos__unknown_deps__returns_error", func(t *testing.T) {
		config := workspace.MakeConfig(
			[]workspace.WorkspaceLibrary{
				workspace.MakeLibrary("g-lib", "missing"),
			},
			[]workspace.WorkspaceApp{
				workspace.MakeApp("app", []workspace.WorkspaceLibrary{
					{
						Name: "a-lib",
						Deps: []workspace.WorkspaceDep{
							{Lib: "missing", From: "global"},
							{Lib: "g-lib", From: "global"},
							{Lib: "project-x", From: "project"},
						},
					},
				}),
			},
			[]workspace.WorkspaceProject{
				workspace.MakeProject("project-x", "missing"),
			},
		)

		if _, err := BuildDependencyGraph(config); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestNodeBuildInfo(t *testing.T) {
	t.Run("domain__service_node__returns_paths", func(t *testing.T) {
		node := makeNode(NodeService, "app", "svc")

		label, path, hashPath, err := nodeBuildInfo("/root", node)
		if err != nil {
			t.Fatalf("nodeBuildInfo: %v", err)
		}
		if label != "service app/svc" {
			t.Fatalf("unexpected label: %s", label)
		}
		if path != "/root/repos/apps/app/services/svc" {
			t.Fatalf("unexpected path: %s", path)
		}
		if hashPath == "" {
			t.Fatalf("expected hash path")
		}
	})

	t.Run("boundary__global_lib_node__returns_paths", func(t *testing.T) {
		node := makeNode(NodeGlobalLib, "", "lib")

		label, path, hashPath, err := nodeBuildInfo("/root", node)
		if err != nil {
			t.Fatalf("nodeBuildInfo: %v", err)
		}
		if label != "library lib" {
			t.Fatalf("unexpected label: %s", label)
		}
		if path != "/root/repos/libs/lib" {
			t.Fatalf("unexpected path: %s", path)
		}
		if hashPath == "" {
			t.Fatalf("expected hash path")
		}
	})

	t.Run("complement__unsupported_node__returns_error", func(t *testing.T) {
		node := makeNode("invalid-kind", "", "name")

		if _, _, _, err := nodeBuildInfo("/root", node); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("chaos__empty_root__returns_paths", func(t *testing.T) {
		node := makeNode(NodeProject, "", "proj")

		label, path, hashPath, err := nodeBuildInfo("", node)
		if err != nil {
			t.Fatalf("nodeBuildInfo: %v", err)
		}
		if label != "project proj" {
			t.Fatalf("unexpected label: %s", label)
		}
		if path != "repos/projects/proj" {
			t.Fatalf("unexpected path: %s", path)
		}
		if hashPath == "" {
			t.Fatalf("expected hash path")
		}
	})
}

func TestNodeCheckInfo(t *testing.T) {
	t.Run("domain__service_node__returns_paths", func(t *testing.T) {
		node := makeNode(NodeService, "app", "svc")

		label, path, hashPath, err := nodeCheckInfo("/root", node)
		if err != nil {
			t.Fatalf("nodeCheckInfo: %v", err)
		}
		if label != "service app/svc" {
			t.Fatalf("unexpected label: %s", label)
		}
		if path != "/root/repos/apps/app/services/svc" {
			t.Fatalf("unexpected path: %s", path)
		}
		if hashPath == "" {
			t.Fatalf("expected hash path")
		}
	})

	t.Run("boundary__global_lib_node__returns_paths", func(t *testing.T) {
		node := makeNode(NodeGlobalLib, "", "lib")

		label, path, hashPath, err := nodeCheckInfo("/root", node)
		if err != nil {
			t.Fatalf("nodeCheckInfo: %v", err)
		}
		if label != "library lib" {
			t.Fatalf("unexpected label: %s", label)
		}
		if path != "/root/repos/libs/lib" {
			t.Fatalf("unexpected path: %s", path)
		}
		if hashPath == "" {
			t.Fatalf("expected hash path")
		}
	})

	t.Run("complement__unsupported_node__returns_error", func(t *testing.T) {
		node := makeNode("invalid-kind", "", "name")

		if _, _, _, err := nodeCheckInfo("/root", node); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("chaos__empty_root__returns_paths", func(t *testing.T) {
		node := makeNode(NodeProject, "", "proj")

		label, path, hashPath, err := nodeCheckInfo("", node)
		if err != nil {
			t.Fatalf("nodeCheckInfo: %v", err)
		}
		if label != "project proj" {
			t.Fatalf("unexpected label: %s", label)
		}
		if path != "repos/projects/proj" {
			t.Fatalf("unexpected path: %s", path)
		}
		if hashPath == "" {
			t.Fatalf("expected hash path")
		}
	})
}

func TestFindGlobalLibraryByName(t *testing.T) {
	t.Run("domain__single_match__returns_library", func(t *testing.T) {
		config := workspace.MakeConfig(
			[]workspace.WorkspaceLibrary{
				workspace.MakeLibrary("lib-a"),
			},
			nil,
			nil,
		)

		lib, ok, err := findGlobalLibraryByName(config, "lib-a")
		if err != nil {
			t.Fatalf("findGlobalLibraryByName: %v", err)
		}
		if !ok {
			t.Fatalf("expected match")
		}
		if lib.Name != "lib-a" {
			t.Fatalf("unexpected library: %#v", lib)
		}
		if len(lib.Deps) != 0 {
			t.Fatalf("unexpected library: %#v", lib)
		}
	})

	t.Run("boundary__empty_list__returns_not_found", func(t *testing.T) {
		config := workspace.MakeConfig(nil, nil, nil)

		if _, ok, err := findGlobalLibraryByName(config, "lib-a"); err != nil {
			t.Fatalf("findGlobalLibraryByName: %v", err)
		} else if ok {
			t.Fatalf("expected no match")
		}
	})

	t.Run("complement__missing_name__returns_not_found", func(t *testing.T) {
		config := workspace.MakeConfig(
			[]workspace.WorkspaceLibrary{
				workspace.MakeLibrary("lib-a"),
			},
			nil,
			nil,
		)

		if _, ok, err := findGlobalLibraryByName(config, "missing"); err != nil {
			t.Fatalf("findGlobalLibraryByName: %v", err)
		} else if ok {
			t.Fatalf("expected no match")
		}
	})

	t.Run("chaos__duplicate_name__returns_error", func(t *testing.T) {
		config := workspace.MakeConfig(
			[]workspace.WorkspaceLibrary{
				workspace.MakeLibrary("lib-a"),
				workspace.MakeLibrary("lib-a"),
			},
			nil,
			nil,
		)

		if _, _, err := findGlobalLibraryByName(config, "lib-a"); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestFindProjectByName(t *testing.T) {
	t.Run("domain__single_match__returns_project", func(t *testing.T) {
		config := workspace.MakeConfig(
			nil,
			nil,
			[]workspace.WorkspaceProject{
				workspace.MakeProject("proj-a"),
			},
		)

		project, ok, err := findProjectByName(config, "proj-a")
		if err != nil {
			t.Fatalf("findProjectByName: %v", err)
		}
		if !ok {
			t.Fatalf("expected match")
		}
		if project.Name != "proj-a" {
			t.Fatalf("unexpected project: %#v", project)
		}
		if len(project.Deps) != 0 {
			t.Fatalf("unexpected project: %#v", project)
		}
	})

	t.Run("boundary__empty_list__returns_not_found", func(t *testing.T) {
		config := workspace.MakeConfig(nil, nil, nil)

		if _, ok, err := findProjectByName(config, "proj-a"); err != nil {
			t.Fatalf("findProjectByName: %v", err)
		} else if ok {
			t.Fatalf("expected no match")
		}
	})

	t.Run("complement__missing_name__returns_not_found", func(t *testing.T) {
		config := workspace.MakeConfig(
			nil,
			nil,
			[]workspace.WorkspaceProject{
				workspace.MakeProject("proj-a"),
			},
		)

		if _, ok, err := findProjectByName(config, "missing"); err != nil {
			t.Fatalf("findProjectByName: %v", err)
		} else if ok {
			t.Fatalf("expected no match")
		}
	})

	t.Run("chaos__duplicate_name__returns_error", func(t *testing.T) {
		config := workspace.MakeConfig(
			nil,
			nil,
			[]workspace.WorkspaceProject{
				workspace.MakeProject("proj-a"),
				workspace.MakeProject("proj-a"),
			},
		)

		if _, _, err := findProjectByName(config, "proj-a"); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func assertOrderInvariant(t *testing.T, target Node, order []Node) {
	t.Helper()
	seen := map[string]struct{}{}
	targetKey := nodeKey(target)
	for _, node := range order {
		key := nodeKey(node)
		if key == targetKey {
			t.Fatalf("expected target to be excluded from order")
		}
		if _, ok := seen[key]; ok {
			t.Fatalf("expected unique dependencies")
		}
		seen[key] = struct{}{}
	}
}

func nodeKeys(nodes []Node) []string {
	keys := make([]string, 0, len(nodes))
	for _, node := range nodes {
		keys = append(keys, nodeKey(node))
	}
	return keys
}

func assertGraphEntry(t *testing.T, graph map[string][]string, key string, expected []string) {
	t.Helper()
	value, ok := graph[key]
	if !ok {
		t.Fatalf("expected graph entry for %s", key)
	}
	if len(value) != len(expected) {
		t.Fatalf("unexpected deps for %s: %v", key, value)
	}
	for i, dep := range expected {
		if value[i] != dep {
			t.Fatalf("unexpected deps for %s: %v", key, value)
		}
	}
}
