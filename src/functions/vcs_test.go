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
	t.Run("domain__configured_tasks__runs_init_commit_then_remote", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		cfg := WorkspaceConfig{
			Workspace: "ws",
			Tasks: map[string]string{
				tokens.WorkspaceTaskInit:          "git init -b main {{repo:path}}",
				tokens.WorkspaceTaskInitialCommit: "git -C {{repo:path}} commit --allow-empty -m init {{repo:name}}",
				tokens.WorkspaceTaskCreateRemote:  "gh repo create {{repo:name}}",
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
		if len(commands) != 3 {
			t.Fatalf("expected 3 commands, got %d: %v", len(commands), commands)
		}
		if !strings.Contains(commands[0], "git init") {
			t.Errorf("first command should be init, got: %q", commands[0])
		}
		if !strings.Contains(commands[1], "commit") {
			t.Errorf("second command should be initial_commit, got: %q", commands[1])
		}
		if !strings.Contains(commands[2], "gh repo create") {
			t.Errorf("third command should be create_remote, got: %q", commands[2])
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

	t.Run("chaos__create_remote_fails__local_init_still_succeeds", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		cfg := WorkspaceConfig{
			Workspace: "ws",
			Tasks: map[string]string{
				tokens.WorkspaceTaskInit:          "git init -b main {{repo:path}}",
				tokens.WorkspaceTaskInitialCommit: "git -C {{repo:path}} commit --allow-empty -m init {{repo:name}}",
				tokens.WorkspaceTaskCreateRemote:  "gh repo create {{repo:name}}",
			},
			Ports: WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
			Projects: []WorkspaceProject{
				{Name: "tooling", Remote: tokens.RemoteNone},
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
			if strings.Contains(command, "gh repo create") {
				return errors.New("boom")
			}
			return nil
		}
		t.Cleanup(func() { runShell = original })

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		err := WireNewRepoVcsAt(fs, "/workspace", &stdout, &stderr, tokens.RepoKindProject, "tooling", "")
		if err != nil {
			t.Fatalf("expected nil error (remote failure is non-fatal), got: %v", err)
		}
		if len(commands) != 3 {
			t.Fatalf("expected init, initial_commit, and create_remote to all run, got %d: %v", len(commands), commands)
		}
		if !strings.Contains(stderr.String(), "create_remote") || !strings.Contains(stderr.String(), "warning") {
			t.Errorf("expected a create_remote warning on stderr, got: %q", stderr.String())
		}
	})
}

func TestInitRepoVcsAt(t *testing.T) {
	baseTasks := func() map[string]string {
		return map[string]string{
			tokens.WorkspaceTaskInit:          "git init -b main {{repo:path}}",
			tokens.WorkspaceTaskInitialCommit: "git -C {{repo:path}} commit --allow-empty -m init {{repo:name}}",
			tokens.WorkspaceTaskCreateRemote:  "gh repo create {{repo:name}}",
		}
	}
	seed := func(t *testing.T, cfg WorkspaceConfig, makeDir bool) afero.Fs {
		t.Helper()
		fs := afero.NewMemMapFs()
		if err := WriteWorkspaceConfig(fs, cfg); err != nil {
			t.Fatalf("seed: %v", err)
		}
		if makeDir {
			if err := fs.MkdirAll("repos/projects/tooling", 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
		}
		return fs
	}
	registeredCfg := func(tasks map[string]string, vars map[string]string) WorkspaceConfig {
		return WorkspaceConfig{
			Workspace: "ws",
			Tasks:     tasks,
			Variables: vars,
			Ports:     WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
			Projects:  []WorkspaceProject{{Name: "tooling", Remote: tokens.RemoteNone}},
		}
	}

	t.Run("domain__no_remote__runs_init_and_commit_only", func(t *testing.T) {
		fs := seed(t, registeredCfg(baseTasks(), nil), true)
		var commands []string
		original := runShell
		runShell = func(workdir, command string, stdout, stderr io.Writer) error {
			commands = append(commands, command)
			return nil
		}
		t.Cleanup(func() { runShell = original })

		var out bytes.Buffer
		if err := InitRepoVcsAt(fs, "/workspace", &out, &out, tokens.RepoKindProject, "tooling", "", "", true); err != nil {
			t.Fatalf("InitRepoVcsAt: %v", err)
		}
		if len(commands) != 2 {
			t.Fatalf("expected 2 commands (init, initial_commit), got %d: %v", len(commands), commands)
		}
		cfg, _ := ReadWorkspaceConfig(fs)
		if got := cfg.Projects[0].Remote; got != tokens.RemoteNone {
			t.Errorf("expected recorded remote %q, got %q", tokens.RemoteNone, got)
		}
	})

	t.Run("domain__with_remote__derives_org_name", func(t *testing.T) {
		fs := seed(t, registeredCfg(baseTasks(), map[string]string{"org": "Dusk-Inc"}), true)
		var commands []string
		original := runShell
		runShell = func(workdir, command string, stdout, stderr io.Writer) error {
			commands = append(commands, command)
			return nil
		}
		t.Cleanup(func() { runShell = original })

		var out bytes.Buffer
		if err := InitRepoVcsAt(fs, "/workspace", &out, &out, tokens.RepoKindProject, "tooling", "", "", false); err != nil {
			t.Fatalf("InitRepoVcsAt: %v", err)
		}
		if len(commands) != 3 {
			t.Fatalf("expected 3 commands, got %d: %v", len(commands), commands)
		}
		cfg, _ := ReadWorkspaceConfig(fs)
		if got := cfg.Projects[0].Remote; got != "Dusk-Inc/tooling" {
			t.Errorf("expected recorded remote %q, got %q", "Dusk-Inc/tooling", got)
		}
	})

	t.Run("boundary__unregistered__errors", func(t *testing.T) {
		fs := seed(t, WorkspaceConfig{
			Workspace: "ws",
			Tasks:     baseTasks(),
			Ports:     WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
		}, true)
		rec := withStubRunShell(t, nil)
		var out bytes.Buffer
		err := InitRepoVcsAt(fs, "/workspace", &out, &out, tokens.RepoKindProject, "tooling", "", "", true)
		if err == nil || !strings.Contains(err.Error(), "not registered") {
			t.Fatalf("expected not-registered error, got: %v", err)
		}
		if rec.command != "" {
			t.Errorf("expected no shell invocation, got %q", rec.command)
		}
	})

	t.Run("boundary__existing_git__errors", func(t *testing.T) {
		fs := seed(t, registeredCfg(baseTasks(), nil), true)
		if err := fs.MkdirAll("repos/projects/tooling/.git", 0o755); err != nil {
			t.Fatalf("mkdir .git: %v", err)
		}
		rec := withStubRunShell(t, nil)
		var out bytes.Buffer
		err := InitRepoVcsAt(fs, "/workspace", &out, &out, tokens.RepoKindProject, "tooling", "", "", true)
		if err == nil || !strings.Contains(err.Error(), "already a git repository") {
			t.Fatalf("expected already-a-git-repo error, got: %v", err)
		}
		if rec.command != "" {
			t.Errorf("expected no shell invocation, got %q", rec.command)
		}
	})

	t.Run("boundary__org_absent__records_none_with_warning", func(t *testing.T) {
		fs := seed(t, registeredCfg(baseTasks(), nil), true)
		original := runShell
		runShell = func(workdir, command string, stdout, stderr io.Writer) error { return nil }
		t.Cleanup(func() { runShell = original })

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		if err := InitRepoVcsAt(fs, "/workspace", &stdout, &stderr, tokens.RepoKindProject, "tooling", "", "", false); err != nil {
			t.Fatalf("InitRepoVcsAt: %v", err)
		}
		if !strings.Contains(stderr.String(), "org") {
			t.Errorf("expected an 'org' warning on stderr, got: %q", stderr.String())
		}
		cfg, _ := ReadWorkspaceConfig(fs)
		if got := cfg.Projects[0].Remote; got != tokens.RemoteNone {
			t.Errorf("expected recorded remote %q, got %q", tokens.RemoteNone, got)
		}
	})
}
