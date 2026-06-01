package functions

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

func setupStopWorkspace(t *testing.T, appStopTask string, svcStopTask string) (afero.Fs, string, WorkspaceConfig) {
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
	appConfigJSON := `{"name":"app-a","type":"app","tasks":{"stop":"` + appStopTask + `"}}`
	if err := fs.MkdirAll(appPath, 0o755); err != nil {
		t.Fatalf("mkdir app: %v", err)
	}
	if err := afero.WriteFile(fs, filepath.Join(appPath, "ocean.config.json"), []byte(appConfigJSON), 0o644); err != nil {
		t.Fatalf("write app config: %v", err)
	}

	svcPath := filepath.Join(root, "repos", "apps", "app-a", "services", "svc-a")
	svcConfigJSON := `{"name":"svc-a","type":"service","tasks":{"stop":"` + svcStopTask + `"}}`
	if err := fs.MkdirAll(svcPath, 0o755); err != nil {
		t.Fatalf("mkdir svc: %v", err)
	}
	if err := afero.WriteFile(fs, filepath.Join(svcPath, "ocean.config.json"), []byte(svcConfigJSON), 0o644); err != nil {
		t.Fatalf("write svc config: %v", err)
	}

	return fs, root, config
}

func TestStopTaskRead(t *testing.T) {
	t.Run("domain__stop_app__readsStopTask", func(t *testing.T) {
		fs, root, _ := setupStopWorkspace(t, "echo stopping app", "")
		appPath := filepath.Join(root, "repos", "apps", "app-a")
		stopTask, err := ReadRepoCommand(fs, appPath, "stop")
		if err != nil {
			t.Fatalf("read stop command: %v", err)
		}
		if stopTask != "echo stopping app" {
			t.Fatalf("expected 'echo stopping app', got: %s", stopTask)
		}
	})

	t.Run("domain__stop_service__readsStopTask", func(t *testing.T) {
		fs, root, _ := setupStopWorkspace(t, "", "echo stopping svc")
		svcPath := filepath.Join(root, "repos", "apps", "app-a", "services", "svc-a")
		stopTask, err := ReadRepoCommand(fs, svcPath, "stop")
		if err != nil {
			t.Fatalf("read stop command: %v", err)
		}
		if stopTask != "echo stopping svc" {
			t.Fatalf("expected 'echo stopping svc', got: %s", stopTask)
		}
	})

	t.Run("complement__stop_service__noStopTask_skips", func(t *testing.T) {
		fs, root, _ := setupStopWorkspace(t, "", "")
		svcPath := filepath.Join(root, "repos", "apps", "app-a", "services", "svc-a")
		stopTask, err := ReadRepoCommand(fs, svcPath, "stop")
		if err != nil {
			t.Fatalf("read stop command: %v", err)
		}
		if strings.TrimSpace(stopTask) != "" {
			t.Fatalf("expected empty stop task, got: %s", stopTask)
		}
	})

	t.Run("complement__stop_app__missingApp_returnsError", func(t *testing.T) {
		config := MakeConfig(nil, nil, nil)
		if FindAppIndex(config, "nonexistent") != -1 {
			t.Fatalf("expected app not found")
		}
	})
}
