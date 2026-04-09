package functions

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
)

// seedScratchWorkspace lays down a minimal valid ocean.workspace.json so
// register/adopt tests can exercise the workspace-mutation paths without
// going through the full InitWorkspace ceremony.
func seedScratchWorkspace(t *testing.T, fs afero.Fs) {
	t.Helper()
	cfg := WorkspaceConfig{
		Workspace: "scratch",
		Ports:     WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
		Apps:      []WorkspaceApp{},
		Libraries: []WorkspaceLibrary{},
		Projects:  []WorkspaceProject{},
	}
	if err := WriteWorkspaceConfig(fs, cfg); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestRegisterRepo_Project(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	if err := fs.MkdirAll("repos/projects/tooling", 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	var out bytes.Buffer
	err := RegisterRepo(fs, &out, tokens.RepoKindProject, "tooling", "", "git@github.com:dusk-inc/tooling.git", "")
	if err != nil {
		t.Fatalf("RegisterRepo: %v", err)
	}
	cfg, err := ReadWorkspaceConfig(fs)
	if err != nil {
		t.Fatalf("ReadWorkspaceConfig: %v", err)
	}
	if len(cfg.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(cfg.Projects))
	}
	if cfg.Projects[0].Name != "tooling" {
		t.Errorf("name: got %q", cfg.Projects[0].Name)
	}
	if cfg.Projects[0].Remote != "git@github.com:dusk-inc/tooling.git" {
		t.Errorf("remote: got %q", cfg.Projects[0].Remote)
	}
	if _, err := fs.Stat("repos/projects/tooling/ocean.config.json"); err != nil {
		t.Errorf("expected ocean.config.json: %v", err)
	}
}

func TestRegisterRepo_GlobalLibrary(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	if err := fs.MkdirAll("repos/libs/lib-a", 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := RegisterRepo(fs, &bytes.Buffer{}, tokens.RepoKindLibrary, "lib-a", "", "git@github.com:dusk-inc/lib-a.git", ""); err != nil {
		t.Fatalf("RegisterRepo: %v", err)
	}
	cfg, _ := ReadWorkspaceConfig(fs)
	if len(cfg.Libraries) != 1 || cfg.Libraries[0].Name != "lib-a" {
		t.Fatalf("global library not registered: %+v", cfg.Libraries)
	}
	if cfg.Libraries[0].Remote != "git@github.com:dusk-inc/lib-a.git" {
		t.Errorf("remote: got %q", cfg.Libraries[0].Remote)
	}
}

func TestRegisterRepo_AppScopedLibrary(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	if err := fs.MkdirAll("repos/apps/app-a/libs/lib-a", 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := RegisterRepo(fs, &bytes.Buffer{}, tokens.RepoKindLibrary, "lib-a", "app-a", "", ""); err != nil {
		t.Fatalf("RegisterRepo: %v", err)
	}
	cfg, _ := ReadWorkspaceConfig(fs)
	if len(cfg.Apps) != 1 || cfg.Apps[0].Name != "app-a" {
		t.Fatalf("app entry missing: %+v", cfg.Apps)
	}
	if len(cfg.Apps[0].Libraries) != 1 || cfg.Apps[0].Libraries[0].Name != "lib-a" {
		t.Fatalf("app library not registered: %+v", cfg.Apps[0].Libraries)
	}
	if cfg.Apps[0].Libraries[0].Remote != tokens.RemoteNone {
		t.Errorf("expected RemoteNone sentinel when --remote omitted, got %q", cfg.Apps[0].Libraries[0].Remote)
	}
}

func TestRegisterRepo_App(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	if err := fs.MkdirAll("repos/apps/app-a", 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := RegisterRepo(fs, &bytes.Buffer{}, tokens.RepoKindApp, "app-a", "", "git@github.com:dusk-inc/app-a.git", ""); err != nil {
		t.Fatalf("RegisterRepo: %v", err)
	}
	cfg, _ := ReadWorkspaceConfig(fs)
	if len(cfg.Apps) != 1 || cfg.Apps[0].Name != "app-a" {
		t.Fatalf("app not registered: %+v", cfg.Apps)
	}
	if cfg.Apps[0].Remote != "git@github.com:dusk-inc/app-a.git" {
		t.Errorf("remote: got %q", cfg.Apps[0].Remote)
	}
}

func TestRegisterRepo_Service(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	// Service register requires the parent app directory + service dir.
	if err := fs.MkdirAll("repos/apps/app-a/services/svc-a", 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := RegisterRepo(fs, &bytes.Buffer{}, tokens.RepoKindService, "svc-a", "app-a", "", ""); err != nil {
		t.Fatalf("RegisterRepo: %v", err)
	}
	cfg, _ := ReadWorkspaceConfig(fs)
	if len(cfg.Apps) != 1 {
		t.Fatalf("expected app entry, got %d", len(cfg.Apps))
	}
	if len(cfg.Apps[0].Services) != 1 {
		t.Fatalf("expected service, got %d", len(cfg.Apps[0].Services))
	}
	svc := cfg.Apps[0].Services[0]
	if svc.Name != "svc-a" || svc.Remote != tokens.RemoteNone {
		t.Errorf("service: got %+v", svc)
	}
	if svc.Port == "" {
		t.Errorf("expected NextServicePort assignment, got empty")
	}
}

func TestRegisterRepo_MissingDirectoryErrors(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	err := RegisterRepo(fs, &bytes.Buffer{}, tokens.RepoKindProject, "ghost", "", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no directory at") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRegisterRepo_AlreadyRegisteredErrors(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	repoPath := "repos/projects/tooling"
	if err := fs.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := afero.WriteFile(fs, filepath.Join(repoPath, "ocean.config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	err := RegisterRepo(fs, &bytes.Buffer{}, tokens.RepoKindProject, "tooling", "", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRegisterRepo_InvalidFlagsRejected(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	if err := fs.MkdirAll("repos/projects/tooling", 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	err := RegisterRepo(fs, &bytes.Buffer{}, tokens.RepoKindProject, "tooling", "app-a", "", "")
	if err == nil {
		t.Fatal("expected validation error for project + --app")
	}
}
