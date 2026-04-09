package functions

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
)

// withStubGitClone replaces the package-level gitClone function for the
// duration of a test, restoring the original on cleanup. The stub creates
// the destination directory in the test's afero.Fs to mimic a successful
// clone without touching the network or executing git.
func withStubGitClone(t *testing.T, fs afero.Fs) {
	t.Helper()
	original := gitClone
	gitClone = func(url, dest string, stdout, stderr io.Writer) error {
		if err := fs.MkdirAll(dest, 0o755); err != nil {
			return err
		}
		// Drop a marker file so the test can verify the stub ran.
		return afero.WriteFile(fs, filepath.Join(dest, ".cloned"), []byte(url), 0o644)
	}
	t.Cleanup(func() { gitClone = original })
}

func withFailingGitClone(t *testing.T) {
	t.Helper()
	original := gitClone
	gitClone = func(url, dest string, stdout, stderr io.Writer) error {
		return fmt.Errorf("simulated clone failure")
	}
	t.Cleanup(func() { gitClone = original })
}

func TestAdoptRepo_ProjectSuccess(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	withStubGitClone(t, fs)

	err := AdoptRepo(fs, &bytes.Buffer{}, &bytes.Buffer{},
		"git@github.com:dusk-inc/tooling.git", tokens.RepoKindProject, "", "", "")
	if err != nil {
		t.Fatalf("AdoptRepo: %v", err)
	}
	if _, err := fs.Stat("repos/projects/tooling/.cloned"); err != nil {
		t.Errorf("expected .cloned marker: %v", err)
	}
	if _, err := fs.Stat("repos/projects/tooling/ocean.config.json"); err != nil {
		t.Errorf("expected ocean.config.json: %v", err)
	}
	cfg, _ := ReadWorkspaceConfig(fs)
	if len(cfg.Projects) != 1 || cfg.Projects[0].Name != "tooling" {
		t.Fatalf("project not registered: %+v", cfg.Projects)
	}
	if cfg.Projects[0].Remote != "git@github.com:dusk-inc/tooling.git" {
		t.Errorf("remote: got %q", cfg.Projects[0].Remote)
	}
}

func TestAdoptRepo_GlobalLibrarySuccess(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	withStubGitClone(t, fs)
	if err := AdoptRepo(fs, &bytes.Buffer{}, &bytes.Buffer{},
		"https://github.com/dusk-inc/lib-a.git", tokens.RepoKindLibrary, "lib-a", "", ""); err != nil {
		t.Fatalf("AdoptRepo: %v", err)
	}
	cfg, _ := ReadWorkspaceConfig(fs)
	if len(cfg.Libraries) != 1 || cfg.Libraries[0].Name != "lib-a" {
		t.Fatalf("library not registered: %+v", cfg.Libraries)
	}
}

func TestAdoptRepo_AppScopedLibrarySuccess(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	withStubGitClone(t, fs)
	if err := AdoptRepo(fs, &bytes.Buffer{}, &bytes.Buffer{},
		"git@github.com:dusk-inc/lib-a.git", tokens.RepoKindLibrary, "lib-a", "app-a", ""); err != nil {
		t.Fatalf("AdoptRepo: %v", err)
	}
	if _, err := fs.Stat("repos/apps/app-a/libs/lib-a/ocean.config.json"); err != nil {
		t.Errorf("expected app-scoped library config: %v", err)
	}
	cfg, _ := ReadWorkspaceConfig(fs)
	if len(cfg.Apps) != 1 || len(cfg.Apps[0].Libraries) != 1 {
		t.Fatalf("app library not registered: %+v", cfg.Apps)
	}
}

func TestAdoptRepo_AppSuccess(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	withStubGitClone(t, fs)
	if err := AdoptRepo(fs, &bytes.Buffer{}, &bytes.Buffer{},
		"git@github.com:dusk-inc/app-a.git", tokens.RepoKindApp, "app-a", "", ""); err != nil {
		t.Fatalf("AdoptRepo: %v", err)
	}
	cfg, _ := ReadWorkspaceConfig(fs)
	if len(cfg.Apps) != 1 || cfg.Apps[0].Name != "app-a" {
		t.Fatalf("app not registered: %+v", cfg.Apps)
	}
}

func TestAdoptRepo_ServiceSuccess(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	withStubGitClone(t, fs)
	if err := AdoptRepo(fs, &bytes.Buffer{}, &bytes.Buffer{},
		"git@github.com:dusk-inc/svc-a.git", tokens.RepoKindService, "svc-a", "app-a", ""); err != nil {
		t.Fatalf("AdoptRepo: %v", err)
	}
	cfg, _ := ReadWorkspaceConfig(fs)
	if len(cfg.Apps[0].Services) != 1 || cfg.Apps[0].Services[0].Name != "svc-a" {
		t.Fatalf("service not registered: %+v", cfg.Apps[0].Services)
	}
	if cfg.Apps[0].Services[0].Port == "" {
		t.Errorf("expected port assignment")
	}
}

func TestAdoptRepo_NameDefaultsFromRemote(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	withStubGitClone(t, fs)
	if err := AdoptRepo(fs, &bytes.Buffer{}, &bytes.Buffer{},
		"git@github.com:dusk-inc/derived-name.git", tokens.RepoKindProject, "", "", ""); err != nil {
		t.Fatalf("AdoptRepo: %v", err)
	}
	cfg, _ := ReadWorkspaceConfig(fs)
	if len(cfg.Projects) != 1 || cfg.Projects[0].Name != "derived-name" {
		t.Fatalf("expected derived-name, got %+v", cfg.Projects)
	}
}

func TestAdoptRepo_DirectoryWithoutConfigErrorsSuggestRegister(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	if err := fs.MkdirAll("repos/projects/tooling", 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	err := AdoptRepo(fs, &bytes.Buffer{}, &bytes.Buffer{},
		"git@github.com:dusk-inc/tooling.git", tokens.RepoKindProject, "tooling", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "register") {
		t.Errorf("expected register suggestion, got %v", err)
	}
}

func TestAdoptRepo_DirectoryWithConfigErrorsAlreadyRegistered(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	if err := fs.MkdirAll("repos/projects/tooling", 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := afero.WriteFile(fs, "repos/projects/tooling/ocean.config.json", []byte("{}"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	err := AdoptRepo(fs, &bytes.Buffer{}, &bytes.Buffer{},
		"git@github.com:dusk-inc/tooling.git", tokens.RepoKindProject, "tooling", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAdoptRepo_CloneFailureSurfacesError(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	withFailingGitClone(t)
	err := AdoptRepo(fs, &bytes.Buffer{}, &bytes.Buffer{},
		"git@github.com:dusk-inc/tooling.git", tokens.RepoKindProject, "tooling", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "git clone") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAdoptRepo_InvalidFlagsRejected(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	err := AdoptRepo(fs, &bytes.Buffer{}, &bytes.Buffer{},
		"git@github.com:dusk-inc/svc-a.git", tokens.RepoKindService, "svc-a", "", "")
	if err == nil {
		t.Fatal("expected error for service without --app")
	}
}

func TestDeriveNameFromRemote(t *testing.T) {
	cases := map[string]string{
		"git@github.com:dusk-inc/svc-a.git":  "svc-a",
		"https://github.com/foo/bar.git":    "bar",
		"https://github.com/foo/bar":        "bar",
		"git@gitlab.com:group/sub/proj.git": "proj",
		"":                                  "",
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			got := deriveNameFromRemote(input)
			if got != want {
				t.Errorf("got %q want %q", got, want)
			}
		})
	}
}
