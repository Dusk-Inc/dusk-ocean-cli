package functions

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

func makeTestCmd(out *bytes.Buffer) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	return cmd
}

func writeOceanConfig(t *testing.T, fs afero.Fs, repoPath string, content string) {
	t.Helper()
	if err := fs.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", repoPath, err)
	}
	if err := afero.WriteFile(fs, repoPath+"/ocean.config.json", []byte(content), 0o644); err != nil {
		t.Fatalf("write ocean.config.json: %v", err)
	}
}

func makeRefreshConfig() WorkspaceConfig {
	return WorkspaceConfig{
		Workspace: "test",
		Tasks:     map[string]string{tokens.WorkspaceTaskClone: "git clone {{repo:remote}} {{repo:path}}"},
		Ports:     WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
		Apps: []WorkspaceApp{
			{
				Name:   "my-app",
				Remote: "https://github.com/example/my-app",
				Libraries: []WorkspaceLibrary{
					{Name: "my-lib"},
				},
				Services: []WorkspaceService{
					{Name: "api", Port: "3000"},
				},
			},
		},
		Libraries: []WorkspaceLibrary{
			{Name: "global-lib", Remote: "https://github.com/example/global-lib"},
		},
		Projects: []WorkspaceProject{
			{Name: "my-project", Remote: "https://github.com/example/my-project"},
		},
	}
}

func TestResolveNodeCloneTarget(t *testing.T) {
	root := "/workspace"
	config := makeRefreshConfig()

	t.Run("domain__app_lib_node__returns_app_path_and_app_name", func(t *testing.T) {
		node := Node{Kind: NodeAppLib, App: "my-app", Name: "my-lib"}
		dest, taskTarget, err := resolveNodeCloneTarget(root, config, node)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(root, tokens.RepoDirRoot, tokens.RepoDirApps, "my-app")
		if dest != want {
			t.Errorf("dest: got %q, want %q", dest, want)
		}
		if taskTarget != "my-app" {
			t.Errorf("taskTarget: got %q, want %q", taskTarget, "my-app")
		}
	})

	t.Run("domain__service_node__returns_app_path_and_app_name", func(t *testing.T) {
		node := Node{Kind: NodeService, App: "my-app", Name: "api"}
		dest, taskTarget, err := resolveNodeCloneTarget(root, config, node)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(root, tokens.RepoDirRoot, tokens.RepoDirApps, "my-app")
		if dest != want {
			t.Errorf("dest: got %q, want %q", dest, want)
		}
		if taskTarget != "my-app" {
			t.Errorf("taskTarget: got %q, want %q", taskTarget, "my-app")
		}
	})

	t.Run("domain__global_lib_node__returns_lib_path_and_lib_name", func(t *testing.T) {
		node := Node{Kind: NodeGlobalLib, Name: "global-lib"}
		dest, taskTarget, err := resolveNodeCloneTarget(root, config, node)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(root, tokens.RepoDirRoot, tokens.RepoDirLibs, "global-lib")
		if dest != want {
			t.Errorf("dest: got %q, want %q", dest, want)
		}
		if taskTarget != "global-lib" {
			t.Errorf("taskTarget: got %q, want %q", taskTarget, "global-lib")
		}
	})

	t.Run("domain__project_node__returns_project_path_and_project_name", func(t *testing.T) {
		node := Node{Kind: NodeProject, Name: "my-project"}
		dest, taskTarget, err := resolveNodeCloneTarget(root, config, node)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(root, tokens.RepoDirRoot, tokens.RepoDirProjects, "my-project")
		if dest != want {
			t.Errorf("dest: got %q, want %q", dest, want)
		}
		if taskTarget != "my-project" {
			t.Errorf("taskTarget: got %q, want %q", taskTarget, "my-project")
		}
	})

	t.Run("error__app_not_in_workspace__returns_error", func(t *testing.T) {
		node := Node{Kind: NodeAppLib, App: "nonexistent", Name: "some-lib"}
		_, _, err := resolveNodeCloneTarget(root, config, node)
		if err == nil {
			t.Fatal("expected error for unknown app")
		}
	})
}

func TestCloneNodeRepoIfMissing(t *testing.T) {
	root := "/workspace"

	t.Run("domain__directory_exists__no_clone_task_run", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		config := makeRefreshConfig()
		if err := WriteWorkspaceConfig(fs, config); err != nil {
			t.Fatalf("seed: %v", err)
		}
		rec := withStubRunShell(t, nil)
		appPath := filepath.Join(root, tokens.RepoDirRoot, tokens.RepoDirApps, "my-app")
		if err := fs.MkdirAll(appPath, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		node := Node{Kind: NodeAppLib, App: "my-app", Name: "my-lib"}
		var out bytes.Buffer
		cmd := makeTestCmd(&out)
		if err := cloneNodeRepoIfMissing(cmd, fs, root, config, node); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.command != "" {
			t.Errorf("expected no clone, but got command: %q", rec.command)
		}
	})

	t.Run("domain__missing_app__runs_clone_task_with_app_name", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		config := makeRefreshConfig()
		if err := WriteWorkspaceConfig(fs, config); err != nil {
			t.Fatalf("seed: %v", err)
		}
		rec := withStubRunShell(t, nil)
		node := Node{Kind: NodeAppLib, App: "my-app", Name: "my-lib"}
		var out bytes.Buffer
		cmd := makeTestCmd(&out)
		if err := cloneNodeRepoIfMissing(cmd, fs, root, config, node); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(rec.command, "my-app") {
			t.Errorf("expected clone command to reference my-app, got: %q", rec.command)
		}
	})

	t.Run("domain__missing_global_lib__runs_clone_task_with_lib_name", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		config := makeRefreshConfig()
		if err := WriteWorkspaceConfig(fs, config); err != nil {
			t.Fatalf("seed: %v", err)
		}
		rec := withStubRunShell(t, nil)
		node := Node{Kind: NodeGlobalLib, Name: "global-lib"}
		var out bytes.Buffer
		cmd := makeTestCmd(&out)
		if err := cloneNodeRepoIfMissing(cmd, fs, root, config, node); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(rec.command, "global-lib") {
			t.Errorf("expected clone command to reference global-lib, got: %q", rec.command)
		}
	})

	t.Run("domain__missing_project__runs_clone_task_with_project_name", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		config := makeRefreshConfig()
		if err := WriteWorkspaceConfig(fs, config); err != nil {
			t.Fatalf("seed: %v", err)
		}
		rec := withStubRunShell(t, nil)
		node := Node{Kind: NodeProject, Name: "my-project"}
		var out bytes.Buffer
		cmd := makeTestCmd(&out)
		if err := cloneNodeRepoIfMissing(cmd, fs, root, config, node); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(rec.command, "my-project") {
			t.Errorf("expected clone command to reference my-project, got: %q", rec.command)
		}
	})
}

func TestCloneNonCodeReposIfMissing(t *testing.T) {
	root := "/workspace"

	t.Run("domain__missing_infra__runs_clone_task", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		config := WorkspaceConfig{
			Workspace: "test",
			Tasks:     map[string]string{tokens.WorkspaceTaskClone: "git clone {{repo:remote}} {{repo:path}}"},
			Ports:     WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
			Infrastructure: []WorkspaceInfra{
				{Name: "terraform-core", Remote: "https://github.com/example/terraform-core"},
			},
		}
		if err := WriteWorkspaceConfig(fs, config); err != nil {
			t.Fatalf("seed: %v", err)
		}
		rec := withStubRunShell(t, nil)

		var out bytes.Buffer
		cmd := makeTestCmd(&out)
		if err := cloneNonCodeReposIfMissing(cmd, fs, root, config); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(rec.command, "terraform-core") {
			t.Errorf("expected clone command to reference terraform-core, got: %q", rec.command)
		}
	})

	t.Run("domain__missing_docs__runs_clone_task", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		config := WorkspaceConfig{
			Workspace: "test",
			Tasks:     map[string]string{tokens.WorkspaceTaskClone: "git clone {{repo:remote}} {{repo:path}}"},
			Ports:     WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
			Docs: []WorkspaceDocs{
				{Name: "handbook", Remote: "https://github.com/example/handbook"},
			},
		}
		if err := WriteWorkspaceConfig(fs, config); err != nil {
			t.Fatalf("seed: %v", err)
		}
		rec := withStubRunShell(t, nil)

		var out bytes.Buffer
		cmd := makeTestCmd(&out)
		if err := cloneNonCodeReposIfMissing(cmd, fs, root, config); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(rec.command, "handbook") {
			t.Errorf("expected clone command to reference handbook, got: %q", rec.command)
		}
	})

	t.Run("domain__directory_exists__no_clone", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		config := WorkspaceConfig{
			Workspace: "test",
			Tasks:     map[string]string{tokens.WorkspaceTaskClone: "git clone {{repo:remote}} {{repo:path}}"},
			Ports:     WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
			Infrastructure: []WorkspaceInfra{
				{Name: "terraform-core", Remote: "https://github.com/example/terraform-core"},
			},
		}
		if err := WriteWorkspaceConfig(fs, config); err != nil {
			t.Fatalf("seed: %v", err)
		}
		dest := filepath.Join(root, tokens.RepoDirRoot, tokens.RepoDirInfra, "terraform-core")
		if err := fs.MkdirAll(dest, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		rec := withStubRunShell(t, nil)

		var out bytes.Buffer
		cmd := makeTestCmd(&out)
		if err := cloneNonCodeReposIfMissing(cmd, fs, root, config); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.command != "" {
			t.Errorf("expected no clone, got: %q", rec.command)
		}
	})

	t.Run("boundary__no_remote__skipped_with_message", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		config := WorkspaceConfig{
			Workspace: "test",
			Tasks:     map[string]string{tokens.WorkspaceTaskClone: "git clone {{repo:remote}} {{repo:path}}"},
			Ports:     WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
			Infrastructure: []WorkspaceInfra{
				{Name: "no-remote-here", Remote: tokens.RemoteNone},
			},
		}
		if err := WriteWorkspaceConfig(fs, config); err != nil {
			t.Fatalf("seed: %v", err)
		}
		rec := withStubRunShell(t, nil)

		var out bytes.Buffer
		cmd := makeTestCmd(&out)
		if err := cloneNonCodeReposIfMissing(cmd, fs, root, config); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.command != "" {
			t.Errorf("expected no clone, got: %q", rec.command)
		}
		if !strings.Contains(out.String(), "no remote configured") {
			t.Errorf("expected skip message, got: %q", out.String())
		}
	})
}

func TestRunInstall(t *testing.T) {
	t.Run("domain__no_install_task__skips_with_message", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		repoPath := "/repo/my-lib"
		writeOceanConfig(t, fs, repoPath, `{"name":"my-lib","language":"go","type":"library","tasks":{}}`)

		var out bytes.Buffer
		cmd := makeTestCmd(&out)

		err := runInstall(cmd, fs, "global library my-lib", repoPath)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(out.String(), "install skipped") {
			t.Fatalf("expected skip message, got: %q", out.String())
		}
	})

	t.Run("boundary__empty_install_command__skips_with_message", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		repoPath := "/repo/my-lib"
		writeOceanConfig(t, fs, repoPath, `{"name":"my-lib","language":"go","type":"library","tasks":{"install":""}}`)

		var out bytes.Buffer
		cmd := makeTestCmd(&out)

		err := runInstall(cmd, fs, "global library my-lib", repoPath)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(out.String(), "install skipped") {
			t.Fatalf("expected skip message, got: %q", out.String())
		}
	})
}

// emptyTasksConfig is a starter ocean.config.json that declares every task
// as empty. Refresh's install/build/check phases treat empty commands as
// no-ops, so an end-to-end refresh test can run on the OS filesystem
// without invoking real shell commands.
const emptyTasksConfig = `{
    "name": "%s",
    "language": "go",
    "type": "%s",
    "tasks": {
        "build": "",
        "test": "",
        "install": "",
        "add": "",
        "uninstall": "",
        "contain": "",
        "publish": "",
        "run": ""
    }
}
`

func seedRepo(t *testing.T, root string, relPath string, name string, kind string) {
	t.Helper()
	full := filepath.Join(root, relPath)
	if err := os.MkdirAll(full, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", full, err)
	}
	configPath := filepath.Join(full, "ocean.config.json")
	payload := []byte(fmt.Sprintf(emptyTasksConfig, name, kind))
	if err := os.WriteFile(configPath, payload, 0o644); err != nil {
		t.Fatalf("write %s: %v", configPath, err)
	}
}

// makeRefreshWorkspace seeds a workspace with a global library that a
// project depends on. Caller decides which directories actually exist on
// disk via the seed flags so each test can simulate access loss
// independently.
type refreshSeeds struct {
	cloneAppDir bool
	cloneLibDir bool
	cloneProjDir bool
}

func makeRefreshWorkspace(t *testing.T, seeds refreshSeeds) (string, WorkspaceConfig) {
	t.Helper()
	root := t.TempDir()

	if seeds.cloneAppDir {
		seedRepo(t, root, filepath.Join("repos", "apps", "shop", "services", "api"), "api", "service")
	}
	if seeds.cloneLibDir {
		seedRepo(t, root, filepath.Join("repos", "libs", "shared"), "shared", "library")
	}
	if seeds.cloneProjDir {
		seedRepo(t, root, filepath.Join("repos", "projects", "tooling"), "tooling", "project")
	}

	config := WorkspaceConfig{
		Workspace: "test",
		Tasks:     map[string]string{tokens.WorkspaceTaskClone: "git clone {{repo:remote}} {{repo:path}}"},
		Ports:     WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
		Apps: []WorkspaceApp{
			{
				Name:   "shop",
				Remote: "git@github.com:dusk-inc/shop.git",
				Services: []WorkspaceService{
					{Name: "api", Port: "3000", Deps: []WorkspaceDep{{Lib: "shared", From: "global"}}},
				},
			},
		},
		Libraries: []WorkspaceLibrary{
			{Name: "shared", Remote: "git@github.com:dusk-inc/shared.git"},
		},
		Projects: []WorkspaceProject{
			{Name: "tooling", Remote: "git@github.com:dusk-inc/tooling.git"},
		},
	}
	chdirForTest(t, root)
	if err := WriteWorkspaceConfig(afero.NewOsFs(), config); err != nil {
		t.Fatalf("write workspace config: %v", err)
	}
	return root, config
}

func chdirForTest(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

func TestRunRefresh_ReportsAllInstalledWhenWorkspaceComplete(t *testing.T) {
	root, config := makeRefreshWorkspace(t, refreshSeeds{cloneAppDir: true, cloneLibDir: true, cloneProjDir: true})
	withStubRunShell(t, nil)

	var out bytes.Buffer
	cmd := makeTestCmd(&out)
	if err := RunRefresh(cmd, afero.NewOsFs(), root, config); err != nil {
		t.Fatalf("RunRefresh: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "Refresh report") {
		t.Fatalf("expected report header, got: %q", output)
	}
	if !strings.Contains(output, "installed: 3") {
		t.Errorf("expected installed: 3 in summary, got: %q", output)
	}
	if strings.Contains(output, "No access") {
		t.Errorf("expected no no-access section, got: %q", output)
	}
	if strings.Contains(output, "missing dependencies:") && !strings.Contains(output, "missing dependencies: 0") {
		t.Errorf("expected zero missing-deps, got: %q", output)
	}
}

func TestRunRefresh_MarksRepoNoAccessWhenCloneFails(t *testing.T) {
	root, config := makeRefreshWorkspace(t, refreshSeeds{cloneAppDir: true, cloneLibDir: true, cloneProjDir: false})
	// runShell errors, simulating `git clone` failing because the user
	// has no access to the upstream repo.
	withStubRunShell(t, fmt.Errorf("Permission denied (publickey)"))

	var stdout, stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	if err := RunRefresh(cmd, afero.NewOsFs(), root, config); err != nil {
		t.Fatalf("RunRefresh should not return error for inaccessible repo: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "No access") {
		t.Errorf("expected no-access section in report, got: %q", output)
	}
	if !strings.Contains(output, "tooling") {
		t.Errorf("expected tooling listed in report, got: %q", output)
	}
	if !strings.Contains(output, "installed: 2") {
		t.Errorf("expected 2 installed (api+shared), got: %q", output)
	}
	if !strings.Contains(stderr.String(), "clone failed") {
		t.Errorf("expected stderr to log clone failure, got: %q", stderr.String())
	}
}

func TestRunRefresh_SkipsDependentWhenLocalDepMissing(t *testing.T) {
	// Service `api` depends on global lib `shared`. The lib was never
	// cloned (no access) — refresh should mark `api` as missing-deps and
	// list `shared` as the unavailable dep.
	root, config := makeRefreshWorkspace(t, refreshSeeds{cloneAppDir: true, cloneLibDir: false, cloneProjDir: true})
	withStubRunShell(t, fmt.Errorf("Permission denied (publickey)"))

	var stdout bytes.Buffer
	cmd := makeTestCmd(&stdout)

	if err := RunRefresh(cmd, afero.NewOsFs(), root, config); err != nil {
		t.Fatalf("RunRefresh should not return error: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "Skipped due to missing dependencies") {
		t.Errorf("expected missing-deps section, got: %q", output)
	}
	if !strings.Contains(output, "service shop/api") {
		t.Errorf("expected api listed under missing-deps, got: %q", output)
	}
	if !strings.Contains(output, "shared") {
		t.Errorf("expected shared listed as missing dep for api, got: %q", output)
	}
	if !strings.Contains(output, "global library shared") {
		t.Errorf("expected shared lib in no-access, got: %q", output)
	}
}

func TestRunRefresh_EmptyWorkspacePrintsSkipMessage(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	config := WorkspaceConfig{
		Workspace: "empty",
		Ports:     WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
	}
	if err := WriteWorkspaceConfig(afero.NewOsFs(), config); err != nil {
		t.Fatalf("write workspace: %v", err)
	}

	var out bytes.Buffer
	cmd := makeTestCmd(&out)
	if err := RunRefresh(cmd, afero.NewOsFs(), root, config); err != nil {
		t.Fatalf("RunRefresh: %v", err)
	}
	if !strings.Contains(out.String(), "no workspace repositories") {
		t.Errorf("expected empty-workspace skip message, got: %q", out.String())
	}
	if strings.Contains(out.String(), "Refresh report") {
		t.Errorf("expected no report on empty workspace, got: %q", out.String())
	}
}
