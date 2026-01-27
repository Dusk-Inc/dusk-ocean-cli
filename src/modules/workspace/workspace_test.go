package workspace

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"github.com/spf13/afero"
)

func TestInitWorkspace(t *testing.T) {
	t.Run("domain__valid_options__creates_workspace_structure", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		var out bytes.Buffer

		err := InitWorkspace(fs, &out, InitOptions{
			Name:     "dusk-ocean",
			Registry: "ghcr.io/dusk-inc",
		})
		if err != nil {
			t.Fatalf("InitWorkspace: %v", err)
		}

		paths := []string{
			"ocean.workspace.json",
			".gitignore",
			".ocean",
			filepath.Join(".ocean", "results"),
			filepath.Join(".ocean", "hashes"),
			"repos",
			filepath.Join("repos", "apps"),
			filepath.Join("repos", "templates"),
			filepath.Join("repos", "libs"),
			filepath.Join("repos", "containers"),
			filepath.Join("repos", "templates", "apps"),
			filepath.Join("repos", "templates", "apps", "services"),
			filepath.Join("repos", "templates", "apps", "libs"),
			filepath.Join("repos", "templates", "apps", "jobs"),
			filepath.Join("repos", "templates", "apps", "testing"),
			filepath.Join("repos", "templates", "apps", "docker-compose.yml"),
			filepath.Join("repos", "templates", "apps", "docker-compose.dev.yml"),
		}

		for _, path := range paths {
			if _, err := fs.Stat(path); err != nil {
				t.Fatalf("expected path: %s: %v", path, err)
			}
		}

		if !gitignoreHasEntry(t, fs, ".ocean") {
			t.Fatalf("expected .gitignore to include .ocean")
		}

		config, err := ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("ReadWorkspaceConfig: %v", err)
		}
		if len(config.Apps) != 0 {
			t.Fatalf("expected no apps, got %d", len(config.Apps))
		}
		if len(config.Libraries) != 0 {
			t.Fatalf("expected no libraries, got %d", len(config.Libraries))
		}
		if len(config.Projects) != 0 {
			t.Fatalf("expected no projects, got %d", len(config.Projects))
		}
	})

	t.Run("boundary__repeat_init__keeps_gitignore_single_entry", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		var out bytes.Buffer
		opts := InitOptions{
			Name:     "dusk-ocean",
			Registry: "ghcr.io/dusk-inc",
		}

		if err := InitWorkspace(fs, &out, opts); err != nil {
			t.Fatalf("InitWorkspace: %v", err)
		}
		if err := InitWorkspace(fs, &out, opts); err != nil {
			t.Fatalf("InitWorkspace repeat: %v", err)
		}

		contents, err := afero.ReadFile(fs, ".gitignore")
		if err != nil {
			t.Fatalf("read .gitignore: %v", err)
		}

		normalized := "\n" + strings.ReplaceAll(string(contents), "\r\n", "\n") + "\n"
		if !strings.Contains(normalized, "\n.ocean\n") {
			t.Fatalf("expected .ocean entry present")
		}
		if strings.Index(normalized, "\n.ocean\n") != strings.LastIndex(normalized, "\n.ocean\n") {
			t.Fatalf("expected single .ocean entry")
		}
	})

	t.Run("complement__missing_name__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		var out bytes.Buffer

		err := InitWorkspace(fs, &out, InitOptions{
			Name:     "",
			Registry: "ghcr.io/dusk-inc",
		})
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("complement__missing_registry__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		var out bytes.Buffer

		err := InitWorkspace(fs, &out, InitOptions{
			Name:     "dusk-ocean",
			Registry: "",
		})
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestEnsureGitIgnore(t *testing.T) {
	t.Run("domain__missing_file__creates_gitignore", func(t *testing.T) {
		fs := afero.NewMemMapFs()

		if err := ensureGitIgnore(fs); err != nil {
			t.Fatalf("ensureGitIgnore: %v", err)
		}

		if !gitignoreHasEntry(t, fs, ".ocean") {
			t.Fatalf("expected .gitignore to include .ocean")
		}
	})

	t.Run("boundary__existing_file_no_newline__appends_entry", func(t *testing.T) {
		fs := afero.NewMemMapFs()

		if err := afero.WriteFile(fs, ".gitignore", []byte("node_modules"), 0o644); err != nil {
			t.Fatalf("write .gitignore: %v", err)
		}

		if err := ensureGitIgnore(fs); err != nil {
			t.Fatalf("ensureGitIgnore: %v", err)
		}

		contents, err := afero.ReadFile(fs, ".gitignore")
		if err != nil {
			t.Fatalf("read .gitignore: %v", err)
		}

		if !strings.HasSuffix(strings.ReplaceAll(string(contents), "\r\n", "\n"), "\n.ocean\n") {
			t.Fatalf("expected appended .ocean")
		}
	})

	t.Run("complement__existing_entry__no_duplicate", func(t *testing.T) {
		fs := afero.NewMemMapFs()

		if err := afero.WriteFile(fs, ".gitignore", []byte(".ocean\n"), 0o644); err != nil {
			t.Fatalf("write .gitignore: %v", err)
		}

		if err := ensureGitIgnore(fs); err != nil {
			t.Fatalf("ensureGitIgnore: %v", err)
		}

		contents, err := afero.ReadFile(fs, ".gitignore")
		if err != nil {
			t.Fatalf("read .gitignore: %v", err)
		}

		normalized := "\n" + strings.ReplaceAll(string(contents), "\r\n", "\n") + "\n"
		if !strings.Contains(normalized, "\n.ocean\n") {
			t.Fatalf("expected .ocean entry present")
		}
		if strings.Index(normalized, "\n.ocean\n") != strings.LastIndex(normalized, "\n.ocean\n") {
			t.Fatalf("expected single .ocean entry")
		}
	})
}

func gitignoreHasEntry(t *testing.T, fs afero.Fs, entry string) bool {
	t.Helper()
	contents, err := afero.ReadFile(fs, ".gitignore")
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	normalized := strings.ReplaceAll(string(contents), "\r\n", "\n")
	for _, line := range strings.Split(normalized, "\n") {
		if strings.TrimSpace(line) == entry {
			return true
		}
	}
	return false
}
