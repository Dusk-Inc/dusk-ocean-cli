package functions

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
)

func TestWireNewRepoVcsAt(t *testing.T) {
	t.Run("domain__configured_tasks__runs_create_then_checkout", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		cfg := WorkspaceConfig{
			Workspace: "ws",
			Tasks: map[string]string{
				tokens.WorkspaceTaskCreateRemote: "gh repo create {{repo:name}}",
				tokens.WorkspaceTaskCheckoutNew:  "git init {{repo:path}}",
			},
			Ports: WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
			Infrastructure: []WorkspaceInfra{
				{Name: "terraform-core", Remote: tokens.RemoteNone},
			},
		}
		if err := WriteWorkspaceConfig(fs, cfg); err != nil {
			t.Fatalf("seed: %v", err)
		}

		var commands []string
		original := runShell
		runShell = func(workdir string, command string, stdout, stderr io.Writer) error {
			_ = workdir
			_ = stdout
			_ = stderr
			commands = append(commands, command)
			return nil
		}
		t.Cleanup(func() { runShell = original })

		var out bytes.Buffer
		if err := WireNewRepoVcsAt(fs, "/workspace", &out, &out, tokens.RepoKindInfra, "terraform-core", ""); err != nil {
			t.Fatalf("WireNewRepoVcsAt: %v", err)
		}
		if len(commands) != 2 {
			t.Fatalf("expected 2 commands, got %d: %v", len(commands), commands)
		}
		if !strings.Contains(commands[0], "terraform-core") {
			t.Errorf("create_remote did not reference repo: %q", commands[0])
		}
		if !strings.Contains(commands[1], "terraform-core") {
			t.Errorf("checkout_new did not reference repo path: %q", commands[1])
		}
	})

	t.Run("boundary__empty_task_commands__skipped_with_message", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		cfg := WorkspaceConfig{
			Workspace: "ws",
			Tasks: map[string]string{
				tokens.WorkspaceTaskCreateRemote: "",
				tokens.WorkspaceTaskCheckoutNew:  "",
			},
			Ports: WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
			Docs: []WorkspaceDocs{
				{Name: "handbook", Remote: tokens.RemoteNone},
			},
		}
		if err := WriteWorkspaceConfig(fs, cfg); err != nil {
			t.Fatalf("seed: %v", err)
		}
		rec := withStubRunShell(t, nil)

		var out bytes.Buffer
		if err := WireNewRepoVcsAt(fs, "/workspace", &out, &out, tokens.RepoKindDocs, "handbook", ""); err != nil {
			t.Fatalf("WireNewRepoVcsAt: %v", err)
		}
		if rec.command != "" {
			t.Errorf("expected no shell invocation, got %q", rec.command)
		}
		if !strings.Contains(out.String(), "skipped") {
			t.Errorf("expected skip message, got: %q", out.String())
		}
	})

	t.Run("complement__service_kind__errors_with_inheritance_note", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := WriteWorkspaceConfig(fs, WorkspaceConfig{
			Workspace: "ws",
			Ports:     WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}
		var out bytes.Buffer
		err := WireNewRepoVcsAt(fs, "/workspace", &out, &out, tokens.RepoKindService, "api", "my-app")
		if err == nil || !strings.Contains(err.Error(), "inherit") {
			t.Fatalf("expected inheritance error, got: %v", err)
		}
	})

	t.Run("chaos__create_remote_fails__checkout_new_not_run", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		cfg := WorkspaceConfig{
			Workspace: "ws",
			Tasks: map[string]string{
				tokens.WorkspaceTaskCreateRemote: "false",
				tokens.WorkspaceTaskCheckoutNew:  "true",
			},
			Ports: WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
			Projects: []WorkspaceProject{
				{Name: "tooling", Remote: tokens.RemoteNone},
			},
		}
		if err := WriteWorkspaceConfig(fs, cfg); err != nil {
			t.Fatalf("seed: %v", err)
		}
		callCount := 0
		original := runShell
		runShell = func(workdir string, command string, stdout, stderr io.Writer) error {
			_ = workdir
			_ = command
			_ = stdout
			_ = stderr
			callCount++
			return errors.New("boom")
		}
		t.Cleanup(func() { runShell = original })

		var out bytes.Buffer
		err := WireNewRepoVcsAt(fs, "/workspace", &out, &out, tokens.RepoKindProject, "tooling", "")
		if err == nil || !strings.Contains(err.Error(), "create_remote") {
			t.Fatalf("expected create_remote error, got: %v", err)
		}
		if callCount != 1 {
			t.Errorf("expected only create_remote to run, got %d invocations", callCount)
		}
	})
}
