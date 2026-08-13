package functions

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

type recordedShellInvocation struct {
	workdir string
	command string
}

func withStubRunShell(t *testing.T, exitErr error) *recordedShellInvocation {
	t.Helper()
	original := runShell
	rec := &recordedShellInvocation{}
	runShell = func(workdir string, command string, stdout io.Writer, stderr io.Writer) error {
		rec.workdir = workdir
		rec.command = command
		return exitErr
	}
	t.Cleanup(func() { runShell = original })
	return rec
}

func TestRunWorkspaceTaskAt_HappyPath(t *testing.T) {
	fs := afero.NewMemMapFs()
	cfg := WorkspaceConfig{
		Workspace: "ws",
		Variables: map[string]string{"org": "dusk-inc"},
		Tasks:     map[string]string{"greet": "echo {{var:org}}/{{repo:name}}"},
		Ports:     WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
		Projects: []WorkspaceProject{
			{Name: "tooling", Remote: "git@github.com:dusk-inc/tooling.git"},
		},
	}
	if err := WriteWorkspaceConfig(fs, cfg); err != nil {
		t.Fatalf("seed: %v", err)
	}
	rec := withStubRunShell(t, nil)
	var stdout bytes.Buffer
	err := RunWorkspaceTaskAt(fs, "/workspace", &stdout, &bytes.Buffer{}, "greet", "tooling", "")
	if err != nil {
		t.Fatalf("RunWorkspaceTaskAt: %v", err)
	}
	if rec.command != "echo dusk-inc/tooling" {
		t.Errorf("command: got %q", rec.command)
	}
	if rec.workdir != "/workspace" {
		t.Errorf("workdir: got %q", rec.workdir)
	}
}

func TestRunWorkspaceTaskAt_ResolvesEnvFromDotEnv(t *testing.T) {
	fs := afero.NewMemMapFs()
	if err := afero.WriteFile(fs, "/workspace/.env", []byte("GITHUB_TOKEN=ghp_test\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	cfg := WorkspaceConfig{
		Workspace: "ws",
		Tasks:     map[string]string{"clone": "git clone https://x:{{env:GITHUB_TOKEN}}@host/{{repo:name}}.git"},
		Ports:     WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
		Projects:  []WorkspaceProject{{Name: "tooling"}},
	}
	if err := WriteWorkspaceConfig(fs, cfg); err != nil {
		t.Fatalf("seed: %v", err)
	}
	rec := withStubRunShell(t, nil)
	if err := RunWorkspaceTaskAt(fs, "/workspace", &bytes.Buffer{}, &bytes.Buffer{}, "clone", "tooling", ""); err != nil {
		t.Fatalf("RunWorkspaceTaskAt: %v", err)
	}
	if rec.command != "git clone https://x:ghp_test@host/tooling.git" {
		t.Errorf("command: got %q", rec.command)
	}
}

func TestRunWorkspaceTaskAt_TaskNotFoundErrors(t *testing.T) {
	fs := afero.NewMemMapFs()
	cfg := WorkspaceConfig{
		Workspace: "ws",
		Ports:     WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
		Projects:  []WorkspaceProject{{Name: "tooling"}},
	}
	if err := WriteWorkspaceConfig(fs, cfg); err != nil {
		t.Fatalf("seed: %v", err)
	}
	withStubRunShell(t, nil)
	err := RunWorkspaceTaskAt(fs, "/workspace", &bytes.Buffer{}, &bytes.Buffer{}, "missing", "tooling", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "workspace task not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunWorkspaceTaskAt_TargetNotFoundErrors(t *testing.T) {
	fs := afero.NewMemMapFs()
	cfg := WorkspaceConfig{
		Workspace: "ws",
		Tasks:     map[string]string{"greet": "echo hi"},
		Ports:     WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
	}
	if err := WriteWorkspaceConfig(fs, cfg); err != nil {
		t.Fatalf("seed: %v", err)
	}
	withStubRunShell(t, nil)
	err := RunWorkspaceTaskAt(fs, "/workspace", &bytes.Buffer{}, &bytes.Buffer{}, "greet", "ghost", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunWorkspaceTaskAt_PropagatesShellError(t *testing.T) {
	fs := afero.NewMemMapFs()
	cfg := WorkspaceConfig{
		Workspace: "ws",
		Tasks:     map[string]string{"greet": "echo {{repo:name}}"},
		Ports:     WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
		Projects:  []WorkspaceProject{{Name: "tooling"}},
	}
	if err := WriteWorkspaceConfig(fs, cfg); err != nil {
		t.Fatalf("seed: %v", err)
	}
	withStubRunShell(t, fmt.Errorf("boom"))
	err := RunWorkspaceTaskAt(fs, "/workspace", &bytes.Buffer{}, &bytes.Buffer{}, "greet", "tooling", "")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected boom error, got %v", err)
	}
}

func TestRunWorkspaceTaskAt_MissingTokenSurfaces(t *testing.T) {
	fs := afero.NewMemMapFs()
	cfg := WorkspaceConfig{
		Workspace: "ws",
		Tasks:     map[string]string{"greet": "echo {{repo:branch}}"},
		Ports:     WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
		Projects:  []WorkspaceProject{{Name: "tooling"}},
	}
	if err := WriteWorkspaceConfig(fs, cfg); err != nil {
		t.Fatalf("seed: %v", err)
	}
	withStubRunShell(t, nil)
	err := RunWorkspaceTaskAt(fs, "/workspace", &bytes.Buffer{}, &bytes.Buffer{}, "greet", "tooling", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "missing variable repo:branch") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveRepoKindByName_TopLevelProject(t *testing.T) {
	cfg := WorkspaceConfig{
		Projects: []WorkspaceProject{{Name: "tooling"}},
	}
	kind, err := ResolveRepoKindByName(cfg, "", "tooling")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != "project" {
		t.Errorf("kind: got %q", kind)
	}
}

func TestResolveRepoKindByName_TopLevelTemplate(t *testing.T) {
	cfg := WorkspaceConfig{
		Templates: []WorkspaceTemplate{{Name: "ts-lib"}},
	}
	kind, err := ResolveRepoKindByName(cfg, "", "ts-lib")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != "template" {
		t.Errorf("kind: got %q", kind)
	}
}

func TestResolveRepoKindByName_AppScopedLibrary(t *testing.T) {
	cfg := WorkspaceConfig{
		Apps: []WorkspaceApp{
			{Name: "app-a", Libraries: []WorkspaceLibrary{{Name: "lib-a"}}},
		},
	}
	kind, err := ResolveRepoKindByName(cfg, "app-a", "lib-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != "library" {
		t.Errorf("kind: got %q", kind)
	}
}

func TestResolveRepoKindByName_AmbiguousErrors(t *testing.T) {
	cfg := WorkspaceConfig{
		Projects:  []WorkspaceProject{{Name: "shared"}},
		Libraries: []WorkspaceLibrary{{Name: "shared"}},
	}
	_, err := ResolveRepoKindByName(cfg, "", "shared")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveRepoKindByName_AppNotFound(t *testing.T) {
	cfg := WorkspaceConfig{}
	_, err := ResolveRepoKindByName(cfg, "ghost-app", "x")
	if err == nil {
		t.Fatal("expected error")
	}
}
