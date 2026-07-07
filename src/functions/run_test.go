package functions

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
)

func setupRunWorkspace(t *testing.T, appRunTask string, svcRunTask string) (afero.Fs, string, WorkspaceConfig) {
	t.Helper()
	fs := afero.NewMemMapFs()
	root := "/workspace"

	svcA := WorkspaceService{Name: "svc-a", Deps: []WorkspaceDep{}}
	config := MakeConfig(
		nil,
		[]WorkspaceApp{{
			Name:      "app-a",
			Services:  []WorkspaceService{svcA},
			Libraries: []WorkspaceLibrary{},
			Testing:   []WorkspaceTest{},
		}},
		nil,
	)
	if err := writeTestWorkspaceConfig(fs, root, config); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	appPath := filepath.Join(root, "repos", "apps", "app-a")
	appConfigJSON := `{"name":"app-a","type":"app","tasks":{"run":"` + appRunTask + `"}}`
	if err := fs.MkdirAll(appPath, 0o755); err != nil {
		t.Fatalf("mkdir app: %v", err)
	}
	if err := afero.WriteFile(fs, filepath.Join(appPath, "ocean.config.json"), []byte(appConfigJSON), 0o644); err != nil {
		t.Fatalf("write app config: %v", err)
	}

	svcPath := filepath.Join(root, "repos", "apps", "app-a", "services", "svc-a")
	svcConfigJSON := `{"name":"svc-a","type":"service","tasks":{"run":"` + svcRunTask + `","build":"","test":""}}`
	if err := fs.MkdirAll(svcPath, 0o755); err != nil {
		t.Fatalf("mkdir svc: %v", err)
	}
	if err := afero.WriteFile(fs, filepath.Join(svcPath, "ocean.config.json"), []byte(svcConfigJSON), 0o644); err != nil {
		t.Fatalf("write svc config: %v", err)
	}

	return fs, root, config
}

func TestRunServiceResolution(t *testing.T) {
	t.Run("complement__run__missing_service_returns_error", func(t *testing.T) {
		config := MakeConfig(
			nil,
			[]WorkspaceApp{{
				Name:      "app-a",
				Services:  []WorkspaceService{{Name: "svc-a", Deps: []WorkspaceDep{}}},
				Libraries: []WorkspaceLibrary{},
				Testing:   []WorkspaceTest{},
			}},
			nil,
		)
		_, _, err := ResolveContainTarget(config, "app-a", "nonexistent")
		if err == nil {
			t.Fatalf("expected error for missing service")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Fatalf("expected 'not found' error, got: %v", err)
		}
	})

	t.Run("domain__run__resolves_service_with_app", func(t *testing.T) {
		config := MakeConfig(
			nil,
			[]WorkspaceApp{{
				Name:      "app-a",
				Services:  []WorkspaceService{{Name: "svc-a", Deps: []WorkspaceDep{}}},
				Libraries: []WorkspaceLibrary{},
				Testing:   []WorkspaceTest{},
			}},
			nil,
		)
		app, svc, err := ResolveContainTarget(config, "app-a", "svc-a")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if app != "app-a" || svc != "svc-a" {
			t.Fatalf("expected app-a/svc-a, got %s/%s", app, svc)
		}
	})

	t.Run("domain__run__resolves_service_without_app", func(t *testing.T) {
		config := MakeConfig(
			nil,
			[]WorkspaceApp{{
				Name:      "app-a",
				Services:  []WorkspaceService{{Name: "svc-a", Deps: []WorkspaceDep{}}},
				Libraries: []WorkspaceLibrary{},
				Testing:   []WorkspaceTest{},
			}},
			nil,
		)
		app, svc, err := ResolveContainTarget(config, "", "svc-a")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if app != "app-a" || svc != "svc-a" {
			t.Fatalf("expected app-a/svc-a, got %s/%s", app, svc)
		}
	})

	t.Run("complement__run__ambiguous_service_without_app_returns_error", func(t *testing.T) {
		config := MakeConfig(
			nil,
			[]WorkspaceApp{
				{Name: "app-a", Services: []WorkspaceService{{Name: "svc-dup", Deps: []WorkspaceDep{}}}, Libraries: []WorkspaceLibrary{}, Testing: []WorkspaceTest{}},
				{Name: "app-b", Services: []WorkspaceService{{Name: "svc-dup", Deps: []WorkspaceDep{}}}, Libraries: []WorkspaceLibrary{}, Testing: []WorkspaceTest{}},
			},
			nil,
		)
		_, _, err := ResolveContainTarget(config, "", "svc-dup")
		if err == nil {
			t.Fatalf("expected error for ambiguous service")
		}
		if !strings.Contains(err.Error(), "ambiguous") {
			t.Fatalf("expected 'ambiguous' error, got: %v", err)
		}
	})
}

func TestRunTaskSkip(t *testing.T) {
	t.Run("complement__run_app__no_run_task_skips", func(t *testing.T) {
		fs, root, config := setupRunWorkspace(t, "", "")
		appPath := filepath.Join(root, "repos", "apps", "app-a")
		runTask, err := ReadRepoCommand(fs, appPath, "run")
		if err != nil {
			t.Fatalf("read run command: %v", err)
		}
		if strings.TrimSpace(runTask) != "" {
			t.Fatalf("expected empty run task, got: %s", runTask)
		}

		appIdx := FindAppIndex(config, "app-a")
		if appIdx == -1 {
			t.Fatalf("app not found")
		}
	})

	t.Run("complement__run_service__no_run_task_skips", func(t *testing.T) {
		fs, root, _ := setupRunWorkspace(t, "", "")
		svcPath := filepath.Join(root, "repos", "apps", "app-a", "services", "svc-a")
		runTask, err := ReadRepoCommand(fs, svcPath, "run")
		if err != nil {
			t.Fatalf("read run command: %v", err)
		}
		if strings.TrimSpace(runTask) != "" {
			t.Fatalf("expected empty run task, got: %s", runTask)
		}
	})

	t.Run("domain__run_app__reads_run_task", func(t *testing.T) {
		fs, root, _ := setupRunWorkspace(t, "echo hello", "")
		appPath := filepath.Join(root, "repos", "apps", "app-a")
		runTask, err := ReadRepoCommand(fs, appPath, "run")
		if err != nil {
			t.Fatalf("read run command: %v", err)
		}
		if runTask != "echo hello" {
			t.Fatalf("expected 'echo hello', got: %s", runTask)
		}
	})

	t.Run("domain__run_service__reads_run_task", func(t *testing.T) {
		fs, root, _ := setupRunWorkspace(t, "", "npm start")
		svcPath := filepath.Join(root, "repos", "apps", "app-a", "services", "svc-a")
		runTask, err := ReadRepoCommand(fs, svcPath, "run")
		if err != nil {
			t.Fatalf("read run command: %v", err)
		}
		if runTask != "npm start" {
			t.Fatalf("expected 'npm start', got: %s", runTask)
		}
	})
}

func TestServiceGroupResolution(t *testing.T) {
	fs := afero.NewMemMapFs()
	root := "/workspace"
	config := MakeConfig(
		nil,
		[]WorkspaceApp{{
			Name:      "app-a",
			Services:  []WorkspaceService{{Name: "svc-a", Deps: []WorkspaceDep{}}},
			Libraries: []WorkspaceLibrary{},
			Testing:   []WorkspaceTest{},
		}},
		nil,
	)
	if err := writeTestWorkspaceConfig(fs, root, config); err != nil {
		t.Fatalf("setup config: %v", err)
	}
	svcPath := filepath.Join(root, "repos", "apps", "app-a", "services", "svc-a")
	svcConfigJSON := `{"name":"svc-a","type":"service","tasks":{"run":"pnpm dev","stop":"echo stop","build":"","test":""},"overrides":[{"group":"test","tasks":{"run":"docker compose up -d --wait","stop":"docker compose down -v"}}]}`
	if err := fs.MkdirAll(svcPath, 0o755); err != nil {
		t.Fatalf("mkdir svc: %v", err)
	}
	if err := afero.WriteFile(fs, filepath.Join(svcPath, "ocean.config.json"), []byte(svcConfigJSON), 0o644); err != nil {
		t.Fatalf("write svc config: %v", err)
	}

	t.Run("domain__group__base_run_uses_plain_task", func(t *testing.T) {
		resolved, err := resolveRepoTask(fs, root, config, svcPath, tokens.RepoKindService, "app-a", "svc-a", "run", SelectGroup(""))
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if resolved.Command != "pnpm dev" {
			t.Fatalf("expected base 'pnpm dev', got %q", resolved.Command)
		}
	})

	t.Run("domain__group__test_group_overrides_run", func(t *testing.T) {
		resolved, err := resolveRepoTask(fs, root, config, svcPath, tokens.RepoKindService, "app-a", "svc-a", "run", SelectGroup("test"))
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if resolved.Command != "docker compose up -d --wait" {
			t.Fatalf("expected group run override, got %q", resolved.Command)
		}
		if resolved.Source != "group" {
			t.Fatalf("expected source 'group', got %q", resolved.Source)
		}
	})

	t.Run("domain__group__test_group_overrides_stop", func(t *testing.T) {
		resolved, err := resolveRepoTask(fs, root, config, svcPath, tokens.RepoKindService, "app-a", "svc-a", "stop", SelectGroup("test"))
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if resolved.Command != "docker compose down -v" {
			t.Fatalf("expected group stop override, got %q", resolved.Command)
		}
	})

	t.Run("complement__group__unknown_group_errors", func(t *testing.T) {
		_, err := resolveRepoTask(fs, root, config, svcPath, tokens.RepoKindService, "app-a", "svc-a", "run", SelectGroup("nope"))
		if err == nil {
			t.Fatalf("expected error for unknown group")
		}
	})
}

func TestRunAppValidation(t *testing.T) {
	t.Run("complement__run_app__missing_app_returns_error", func(t *testing.T) {
		config := MakeConfig(nil, nil, nil)
		appIdx := FindAppIndex(config, "nonexistent")
		if appIdx != -1 {
			t.Fatalf("expected app not found")
		}
	})
}

func TestPreflightService(t *testing.T) {
	t.Run("complement__preflight_service__missing_service_returns_error", func(t *testing.T) {
		fs, root, config := setupRunWorkspace(t, "", "")
		cmd := makeTestCmd(&bytes.Buffer{})
		err := PreflightService(cmd, fs, root, config, "app-a", "nonexistent", false)
		if err == nil {
			t.Fatalf("expected error for missing service")
		}
	})

}

func TestMergeEnvForExec(t *testing.T) {
	t.Setenv("DUSK_OCEAN_TEST_PROCESS_KEY", "from_process")
	t.Setenv("DUSK_OCEAN_TEST_SHARED_KEY", "from_process")

	t.Run("domain__merge_env__envFileKeys_appended", func(t *testing.T) {
		out := mergeEnvForExec(map[string]string{
			"DUSK_OCEAN_TEST_FILE_KEY": "from_file",
		})
		assertEnvContains(t, out, "DUSK_OCEAN_TEST_PROCESS_KEY=from_process")
		assertEnvContains(t, out, "DUSK_OCEAN_TEST_FILE_KEY=from_file")
	})

	t.Run("domain__merge_env__processEnv_winsOnConflict", func(t *testing.T) {
		out := mergeEnvForExec(map[string]string{
			"DUSK_OCEAN_TEST_SHARED_KEY": "from_file",
		})
		assertEnvContains(t, out, "DUSK_OCEAN_TEST_SHARED_KEY=from_process")
		for _, kv := range out {
			if kv == "DUSK_OCEAN_TEST_SHARED_KEY=from_file" {
				t.Fatalf(".env value leaked past process env on conflict")
			}
		}
	})

	t.Run("boundary__merge_env__emptyEnvFile_returnsProcessEnvUnchanged", func(t *testing.T) {
		out := mergeEnvForExec(map[string]string{})
		assertEnvContains(t, out, "DUSK_OCEAN_TEST_PROCESS_KEY=from_process")
	})
}

func assertEnvContains(t *testing.T, env []string, want string) {
	t.Helper()
	for _, kv := range env {
		if kv == want {
			return
		}
	}
	t.Fatalf("expected env to contain %q; not found", want)
}
