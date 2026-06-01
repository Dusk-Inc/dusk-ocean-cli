package functions

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
)

func TestInitWorkspace(t *testing.T) {
	t.Run("domain__valid_options__creates_workspace_structure", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		var out bytes.Buffer

		err := InitWorkspace(fs, &out, InitOptions{
			Name: "dusk-ocean",
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
			filepath.Join("repos", "libs"),
			filepath.Join("repos", "projects"),
			filepath.Join("repos", "templates"),
			filepath.Join("repos", "containers"),
		}

		for _, path := range paths {
			if _, err := fs.Stat(path); err != nil {
				t.Fatalf("expected path: %s: %v", path, err)
			}
		}

		if _, err := fs.Stat(filepath.Join("repos", "templates", "apps")); err == nil {
			t.Fatalf("repos/templates/apps must not be created by init")
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

		if config.Variables == nil {
			t.Fatalf("expected initialized Variables map, got nil")
		}
		if len(config.Variables) != 0 {
			t.Fatalf("expected empty Variables map, got %v", config.Variables)
		}

		if config.Tasks == nil {
			t.Fatalf("expected initialized Tasks map, got nil")
		}
		for _, name := range tokens.DefaultWorkspaceTaskNames {
			cmd, ok := config.Tasks[name]
			if !ok {
				t.Errorf("expected default task %q to be present", name)
				continue
			}
			if cmd != "" {
				t.Errorf("expected default task %q to be empty, got %q", name, cmd)
			}
		}
		if len(config.Tasks) != len(tokens.DefaultWorkspaceTaskNames) {
			t.Errorf("expected %d default tasks, got %d", len(tokens.DefaultWorkspaceTaskNames), len(config.Tasks))
		}
	})

	t.Run("boundary__repeat_init__keeps_gitignore_single_entry", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		var out bytes.Buffer
		opts := InitOptions{
			Name: "dusk-ocean",
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
			Name: "",
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

func TestValidateWorkspaceConfig_RejectsRepoVariableCollision(t *testing.T) {
	cases := []struct {
		name   string
		config WorkspaceConfig
	}{
		{
			name: "project shadows reserved name",
			config: WorkspaceConfig{
				Workspace: "ws",
				Ports:     WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
				Projects: []WorkspaceProject{
					{Name: "tooling", Variables: map[string]string{"name": "shadowed"}},
				},
			},
		},
		{
			name: "global library shadows path",
			config: WorkspaceConfig{
				Workspace: "ws",
				Ports:     WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
				Libraries: []WorkspaceLibrary{
					{Name: "lib-a", Variables: map[string]string{"path": "elsewhere"}},
				},
			},
		},
		{
			name: "service shadows port",
			config: WorkspaceConfig{
				Workspace: "ws",
				Ports:     WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
				Apps: []WorkspaceApp{
					{
						Name: "app-a",
						Services: []WorkspaceService{
							{Name: "svc-a", Variables: map[string]string{"port": "9999"}},
						},
					},
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateWorkspaceConfig(tc.config)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), "collides with reserved repo field") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateWorkspaceConfig_AcceptsBenignUserVariables(t *testing.T) {
	config := WorkspaceConfig{
		Workspace: "ws",
		Ports:     WorkspacePorts{Allowed: WorkspacePortRange{Min: 3000, Max: 3999}},
		Projects: []WorkspaceProject{
			{Name: "tooling", Variables: map[string]string{"deploy_env": "staging", "branch": "main"}},
		},
	}
	if err := ValidateWorkspaceConfig(config); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
