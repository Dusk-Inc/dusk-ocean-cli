package functions

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
)

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

func TestRegisterRepo_Infra(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	if err := fs.MkdirAll("repos/infra/terraform-core", 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := RegisterRepo(fs, &bytes.Buffer{}, tokens.RepoKindInfra, "terraform-core", "", "git@github.com:dusk-inc/terraform-core.git", ""); err != nil {
		t.Fatalf("RegisterRepo: %v", err)
	}
	cfg, err := ReadWorkspaceConfig(fs)
	if err != nil {
		t.Fatalf("ReadWorkspaceConfig: %v", err)
	}
	if len(cfg.Infrastructure) != 1 || cfg.Infrastructure[0].Name != "terraform-core" {
		t.Fatalf("expected infra terraform-core registered: %+v", cfg.Infrastructure)
	}
	if cfg.Infrastructure[0].Remote != "git@github.com:dusk-inc/terraform-core.git" {
		t.Errorf("remote: got %q", cfg.Infrastructure[0].Remote)
	}
	repoConfig, err := ReadRepoConfig(fs, "repos/infra/terraform-core")
	if err != nil {
		t.Fatalf("expected starter ocean.config.json: %v", err)
	}
	if repoConfig.Type != "infra" {
		t.Errorf("expected starter type=infra, got %q", repoConfig.Type)
	}
}

func TestRegisterRepo_Docs(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	if err := fs.MkdirAll("repos/docs/handbook", 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := RegisterRepo(fs, &bytes.Buffer{}, tokens.RepoKindDocs, "handbook", "", "git@github.com:dusk-inc/handbook.git", ""); err != nil {
		t.Fatalf("RegisterRepo: %v", err)
	}
	cfg, err := ReadWorkspaceConfig(fs)
	if err != nil {
		t.Fatalf("ReadWorkspaceConfig: %v", err)
	}
	if len(cfg.Docs) != 1 || cfg.Docs[0].Name != "handbook" {
		t.Fatalf("expected docs handbook registered: %+v", cfg.Docs)
	}
	if cfg.Docs[0].Remote != "git@github.com:dusk-inc/handbook.git" {
		t.Errorf("remote: got %q", cfg.Docs[0].Remote)
	}
	repoConfig, err := ReadRepoConfig(fs, "repos/docs/handbook")
	if err != nil {
		t.Fatalf("expected starter ocean.config.json: %v", err)
	}
	if repoConfig.Type != "docs" {
		t.Errorf("expected starter type=docs, got %q", repoConfig.Type)
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

func TestRegisterRepo_Test(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	if err := fs.MkdirAll("repos/apps/app-a/testing/e2e-a", 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := RegisterRepo(fs, &bytes.Buffer{}, tokens.RepoKindTest, "e2e-a", "app-a", "git@github.com:dusk-inc/e2e-a.git", ""); err != nil {
		t.Fatalf("RegisterRepo: %v", err)
	}
	cfg, err := ReadWorkspaceConfig(fs)
	if err != nil {
		t.Fatalf("ReadWorkspaceConfig: %v", err)
	}
	if len(cfg.Apps) != 1 || cfg.Apps[0].Name != "app-a" {
		t.Fatalf("app entry missing: %+v", cfg.Apps)
	}
	if len(cfg.Apps[0].Testing) != 1 || cfg.Apps[0].Testing[0].Name != "e2e-a" {
		t.Fatalf("test not registered: %+v", cfg.Apps[0].Testing)
	}
	if cfg.Apps[0].Testing[0].Remote != "git@github.com:dusk-inc/e2e-a.git" {
		t.Errorf("remote: got %q", cfg.Apps[0].Testing[0].Remote)
	}
	repoConfig, err := ReadRepoConfig(fs, "repos/apps/app-a/testing/e2e-a")
	if err != nil {
		t.Fatalf("expected starter ocean.config.json: %v", err)
	}
	if repoConfig.Type != "test" {
		t.Errorf("expected starter type=test, got %q", repoConfig.Type)
	}
	if _, err := MakeTestNode(cfg, "app-a", "e2e-a"); err != nil {
		t.Errorf("registered test should resolve to a graph node: %v", err)
	}
}

func TestRegisterRepo_TestRejectsDuplicate(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	if err := fs.MkdirAll("repos/apps/app-a/testing/e2e-a", 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := RegisterRepo(fs, &bytes.Buffer{}, tokens.RepoKindTest, "e2e-a", "app-a", "", ""); err != nil {
		t.Fatalf("RegisterRepo: %v", err)
	}
	err := RegisterRepo(fs, &bytes.Buffer{}, tokens.RepoKindTest, "e2e-a", "app-a", "", "")
	if err == nil {
		t.Fatalf("expected duplicate registration to be rejected")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("got %v, want an already-registered error", err)
	}
}

func TestRegisterRepo_TestRequiresApp(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	err := RegisterRepo(fs, &bytes.Buffer{}, tokens.RepoKindTest, "e2e-a", "", "", "")
	if err == nil {
		t.Fatalf("expected an error when --app is omitted")
	}
	if !strings.Contains(err.Error(), "--app is required") {
		t.Errorf("got %v, want an --app is required error", err)
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

	if err := AddProjectToWorkspace(fs, "tooling"); err != nil {
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

func TestRegisterRepo_ExistingConfigNoWorkspaceEntry(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	repoPath := "repos/projects/dusk-ocean-cli"
	if err := fs.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	original := []byte(`{"name":"dusk-ocean-cli","language":"go","type":"project"}`)
	if err := afero.WriteFile(fs, filepath.Join(repoPath, "ocean.config.json"), original, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := RegisterRepo(fs, &bytes.Buffer{}, tokens.RepoKindProject, "dusk-ocean-cli", "", "https://github.com/D-U-S-K-D-E-V/dusk-ocean-cli", ""); err != nil {
		t.Fatalf("RegisterRepo: %v", err)
	}

	cfg := readWorkspaceConfig(t, fs)
	if len(cfg.Projects) != 1 || cfg.Projects[0].Name != "dusk-ocean-cli" {
		t.Fatalf("expected project registered, got %+v", cfg.Projects)
	}
	if cfg.Projects[0].Remote != "https://github.com/D-U-S-K-D-E-V/dusk-ocean-cli" {
		t.Errorf("unexpected remote: %q", cfg.Projects[0].Remote)
	}

	got, err := afero.ReadFile(fs, filepath.Join(repoPath, "ocean.config.json"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(got) != string(original) {
		t.Fatalf("ocean.config.json was modified:\nwant: %s\ngot:  %s", original, got)
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
