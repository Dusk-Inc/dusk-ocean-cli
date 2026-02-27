package functions

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

func TestRunUninstallRemovesDependency(t *testing.T) {
	fs := afero.NewMemMapFs()
	config := WorkspaceConfig{
		Workspace: "test",
		Ports: WorkspacePorts{
			Allowed:  WorkspacePortRange{Min: 3000, Max: 3999},
			Reserved: []WorkspaceReservedPort{},
		},
		Apps: []WorkspaceApp{
			{
				Name: "app",
				Services: []WorkspaceService{
					{
						Name:       "svc",
						Port:       "3000",
						Image:      WorkspaceImage{Name: "app__svc", Tag: "dev"},
						Dockerfile: "ts.Dockerfile",
						Deps: []WorkspaceDep{
							{Lib: "lib-a", From: "global"},
						},
					},
				},
				Libraries: []WorkspaceLibrary{},
			},
		},
		Libraries: []WorkspaceLibrary{
			{Name: "lib-a", Deps: []WorkspaceDep{}},
		},
		Projects: []WorkspaceProject{},
	}
	if err := WriteWorkspaceConfig(fs, config); err != nil {
		t.Fatalf("write config: %v", err)
	}

	target := Target{
		Kind: TargetService,
		App:  "app",
		Name: "svc",
		Path: "/repo/apps/app/services/svc",
	}
	dep := uninstallDependency{
		kind: uninstallDependencyGlobalLib,
		name: "lib-a",
		from: "global",
		path: "/repo/repos/libs/lib-a",
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	runCount := 0
	options := UninstallOptions{
		ReadRepoCommand: func(fs afero.Fs, root string, kind string) (string, error) {
			return "echo ok", nil
		},
		RunCommand: func(command *exec.Cmd) error {
			runCount++
			return nil
		},
	}

	if err := runUninstall(cmd, fs, target, dep, options); err != nil {
		t.Fatalf("run uninstall: %v", err)
	}
	if runCount != 1 {
		t.Fatalf("expected uninstall command to run once, got %d", runCount)
	}
	updated, err := ReadWorkspaceConfig(fs)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if len(updated.Apps) != 1 || len(updated.Apps[0].Services) != 1 {
		t.Fatalf("unexpected config shape")
	}
	if len(updated.Apps[0].Services[0].Deps) != 0 {
		t.Fatalf("expected dependency to be removed")
	}
}

func TestRunUninstallMissingCommand(t *testing.T) {
	fs := afero.NewMemMapFs()
	config := WorkspaceConfig{
		Workspace: "test",
		Ports: WorkspacePorts{
			Allowed:  WorkspacePortRange{Min: 3000, Max: 3999},
			Reserved: []WorkspaceReservedPort{},
		},
		Apps: []WorkspaceApp{
			{
				Name: "app",
				Services: []WorkspaceService{
					{
						Name:       "svc",
						Port:       "3000",
						Image:      WorkspaceImage{Name: "app__svc", Tag: "dev"},
						Dockerfile: "ts.Dockerfile",
						Deps: []WorkspaceDep{
							{Lib: "lib-a", From: "global"},
						},
					},
				},
				Libraries: []WorkspaceLibrary{},
			},
		},
		Libraries: []WorkspaceLibrary{
			{Name: "lib-a", Deps: []WorkspaceDep{}},
		},
		Projects: []WorkspaceProject{},
	}
	if err := WriteWorkspaceConfig(fs, config); err != nil {
		t.Fatalf("write config: %v", err)
	}

	target := Target{
		Kind: TargetService,
		App:  "app",
		Name: "svc",
		Path: "/repo/apps/app/services/svc",
	}
	dep := uninstallDependency{
		kind: uninstallDependencyGlobalLib,
		name: "lib-a",
		from: "global",
		path: "/repo/repos/libs/lib-a",
	}

	runCount := 0
	options := UninstallOptions{
		ReadRepoCommand: func(fs afero.Fs, root string, kind string) (string, error) {
			return "", nil
		},
		RunCommand: func(command *exec.Cmd) error {
			runCount++
			return nil
		},
	}

	cmd := &cobra.Command{}
	if err := runUninstall(cmd, fs, target, dep, options); err == nil {
		t.Fatalf("expected error for missing uninstall command")
	}
	if runCount != 0 {
		t.Fatalf("expected uninstall command not to run")
	}
	updated, err := ReadWorkspaceConfig(fs)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if len(updated.Apps[0].Services[0].Deps) != 1 {
		t.Fatalf("expected dependency to remain")
	}
}
