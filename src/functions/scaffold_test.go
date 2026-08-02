package functions

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

// seedAppTemplate writes a minimal composite app template carrying one nested service.
func seedAppTemplate(t *testing.T, fs afero.Fs, name string) {
	t.Helper()
	templatePath := filepath.Join("repos", "templates", name)
	servicePath := filepath.Join(templatePath, "services", "api")
	if err := fs.MkdirAll(servicePath, 0o755); err != nil {
		t.Fatalf("seed template: %v", err)
	}
	if err := afero.WriteFile(fs, filepath.Join(templatePath, "ocean.config.json"),
		[]byte(`{"name":"{{app_name}}","language":"","type":"app","tasks":{}}`+"\n"), 0o644); err != nil {
		t.Fatalf("seed template config: %v", err)
	}
	if err := afero.WriteFile(fs, filepath.Join(servicePath, "ocean.config.json"),
		[]byte(`{"name":"{{app_name}}-api","language":"typescript","type":"service","tasks":{}}`+"\n"), 0o644); err != nil {
		t.Fatalf("seed service config: %v", err)
	}
}

// seedEmptyWorkspace writes a workspace config with no registered units.
func seedEmptyWorkspace(t *testing.T, fs afero.Fs) {
	t.Helper()
	if err := WriteWorkspaceConfig(fs, WorkspaceConfig{
		Apps:      []WorkspaceApp{},
		Libraries: []WorkspaceLibrary{},
		Projects:  []WorkspaceProject{},
	}); err != nil {
		t.Fatalf("write workspace config: %v", err)
	}
}

func TestAddApp(t *testing.T) {
	t.Run("domain__template__copies_tree_and_registers", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		seedEmptyWorkspace(t, fs)
		seedAppTemplate(t, fs, "base-app")

		if err := AddApp(fs, "alpha", "base-app", map[string]string{"app_name": "alpha"}); err != nil {
			t.Fatalf("AddApp: %v", err)
		}

		appPath := filepath.Join("repos", "apps", "alpha")
		nested := filepath.Join(appPath, "services", "api", "ocean.config.json")
		payload, err := afero.ReadFile(fs, nested)
		if err != nil {
			t.Fatalf("expected the template's nested service to land: %v", err)
		}
		if !strings.Contains(string(payload), `"alpha-api"`) {
			t.Fatalf("expected app_name substituted in nested config, got %s", payload)
		}

		config, err := ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("read workspace config: %v", err)
		}
		if len(config.Apps) != 1 || config.Apps[0].Name != "alpha" {
			t.Fatalf("expected workspace app entry added, got %+v", config.Apps)
		}
	})

	t.Run("domain__template__backfills_canonical_subdirs", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		seedEmptyWorkspace(t, fs)
		seedAppTemplate(t, fs, "base-app")

		if err := AddApp(fs, "alpha", "base-app", map[string]string{"app_name": "alpha"}); err != nil {
			t.Fatalf("AddApp: %v", err)
		}

		appPath := filepath.Join("repos", "apps", "alpha")
		for _, sub := range []string{
			"services", "libs", "projects",
			filepath.Join("jobs", "docker"),
			filepath.Join("jobs", "migrations"),
			filepath.Join("jobs", "scripts"),
			"testing",
		} {
			info, err := fs.Stat(filepath.Join(appPath, sub))
			if err != nil {
				t.Fatalf("expected subfolder %s: %v", sub, err)
			}
			if !info.IsDir() {
				t.Fatalf("expected %s to be a directory", sub)
			}
		}

		if _, err := fs.Stat(filepath.Join(appPath, "docs")); err == nil {
			t.Fatalf("docs/ is no longer part of the canonical app layout")
		}
	})

	t.Run("boundary__populated_subdir__no_gitkeep", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		seedEmptyWorkspace(t, fs)
		seedAppTemplate(t, fs, "base-app")

		if err := AddApp(fs, "alpha", "base-app", map[string]string{"app_name": "alpha"}); err != nil {
			t.Fatalf("AddApp: %v", err)
		}

		appPath := filepath.Join("repos", "apps", "alpha")
		if _, err := fs.Stat(filepath.Join(appPath, "services", ".gitkeep")); err == nil {
			t.Fatalf("services/ came from the template and must not be marked with .gitkeep")
		}
		if _, err := fs.Stat(filepath.Join(appPath, "libs", ".gitkeep")); err != nil {
			t.Fatalf("expected .gitkeep in the empty libs/: %v", err)
		}
	})

	t.Run("boundary__template_without_config__writes_starter", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		seedEmptyWorkspace(t, fs)
		if err := fs.MkdirAll(filepath.Join("repos", "templates", "bare", "services"), 0o755); err != nil {
			t.Fatalf("seed: %v", err)
		}

		if err := AddApp(fs, "alpha", "bare", nil); err != nil {
			t.Fatalf("AddApp: %v", err)
		}

		payload, err := afero.ReadFile(fs, filepath.Join("repos", "apps", "alpha", "ocean.config.json"))
		if err != nil {
			t.Fatalf("expected a starter config to be written: %v", err)
		}
		if !strings.Contains(string(payload), `"type": "app"`) {
			t.Fatalf("expected starter type=app, got %s", payload)
		}
	})

	t.Run("boundary__empty_name__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := AddApp(fs, "", "base-app", nil); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("complement__empty_template__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		seedEmptyWorkspace(t, fs)
		if err := AddApp(fs, "alpha", "", nil); err == nil {
			t.Fatalf("expected a template to be required")
		}
	})

	t.Run("complement__missing_template__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		seedEmptyWorkspace(t, fs)
		if err := AddApp(fs, "alpha", "ghost", nil); err == nil {
			t.Fatalf("expected a missing template to be rejected")
		}
	})

	t.Run("complement__existing_app__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		seedEmptyWorkspace(t, fs)
		seedAppTemplate(t, fs, "base-app")
		if err := fs.MkdirAll(filepath.Join("repos", "apps", "alpha"), 0o755); err != nil {
			t.Fatalf("mkdir app: %v", err)
		}

		if err := AddApp(fs, "alpha", "base-app", nil); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestRemoveApp(t *testing.T) {
	t.Run("domain__existing_app__removes_folder", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		appPath := filepath.Join("repos", "apps", "alpha")
		appFile := filepath.Join(appPath, "README.md")

		if err := fs.MkdirAll(appPath, 0o755); err != nil {
			t.Fatalf("mkdir app: %v", err)
		}
		if err := afero.WriteFile(fs, appFile, []byte("app"), 0o644); err != nil {
			t.Fatalf("write app file: %v", err)
		}

		if err := RemoveApp(fs, "alpha"); err != nil {
			t.Fatalf("RemoveApp: %v", err)
		}

		if _, err := fs.Stat(appPath); err == nil {
			t.Fatalf("expected app folder removed")
		}
	})

	t.Run("domain__existing_app__removes_workspace_entry", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		appPath := filepath.Join("repos", "apps", "alpha")
		if err := fs.MkdirAll(appPath, 0o755); err != nil {
			t.Fatalf("mkdir app: %v", err)
		}
		if err := WriteWorkspaceConfig(fs, WorkspaceConfig{
			Apps: []WorkspaceApp{
				MakeApp("alpha", nil),
				MakeApp("beta", nil),
			},
		}); err != nil {
			t.Fatalf("write workspace config: %v", err)
		}

		if err := RemoveApp(fs, "alpha"); err != nil {
			t.Fatalf("RemoveApp: %v", err)
		}

		config, err := ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("read workspace config: %v", err)
		}
		if len(config.Apps) != 1 || config.Apps[0].Name != "beta" {
			t.Fatalf("expected workspace app entry removed")
		}
	})

	t.Run("boundary__empty_name__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := RemoveApp(fs, ""); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("complement__missing_app__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := RemoveApp(fs, "alpha"); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestScaffoldNextServicePort(t *testing.T) {
	t.Run("domain__existing_ports__returns_next_port", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := WriteWorkspaceConfig(fs, WorkspaceConfig{
			Ports: WorkspacePorts{
				Allowed: WorkspacePortRange{
					Min: 4000,
					Max: 4999,
				},
			},
			Apps: []WorkspaceApp{
				{
					Name: "alpha",
					Services: []WorkspaceService{
						{
							Name: "api",
							Port: "4001",
						},
						{
							Name: "jobs",
							Port: "4005",
						},
					},
				},
			},
		}); err != nil {
			t.Fatalf("write workspace config: %v", err)
		}

		port, err := NextServicePort(fs, "alpha")
		if err != nil {
			t.Fatalf("NextServicePort: %v", err)
		}
		if port != "4006" {
			t.Fatalf("expected next port")
		}
	})

	t.Run("boundary__no_allowed_range__defaults_to_3000", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := WriteWorkspaceConfig(fs, WorkspaceConfig{
			Apps: []WorkspaceApp{
				{
					Name:     "alpha",
					Services: []WorkspaceService{},
				},
			},
		}); err != nil {
			t.Fatalf("write workspace config: %v", err)
		}

		port, err := NextServicePort(fs, "alpha")
		if err != nil {
			t.Fatalf("NextServicePort: %v", err)
		}
		if port != "3000" {
			t.Fatalf("expected default port")
		}
	})

	t.Run("complement__invalid_ports__ignores_non_numeric", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := WriteWorkspaceConfig(fs, WorkspaceConfig{
			Ports: WorkspacePorts{
				Allowed: WorkspacePortRange{
					Min: 4500,
					Max: 4599,
				},
			},
			Apps: []WorkspaceApp{
				{
					Name: "alpha",
					Services: []WorkspaceService{
						{
							Name: "api",
							Port: "bad",
						},
					},
				},
			},
		}); err != nil {
			t.Fatalf("write workspace config: %v", err)
		}

		port, err := NextServicePort(fs, "alpha")
		if err != nil {
			t.Fatalf("NextServicePort: %v", err)
		}
		if port != "4500" {
			t.Fatalf("expected default port")
		}
	})

	t.Run("chaos__invalid_workspace_json__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := afero.WriteFile(fs, "ocean.workspace.json", []byte("{"), 0o644); err != nil {
			t.Fatalf("write workspace config: %v", err)
		}

		if _, err := NextServicePort(fs, "alpha"); err == nil {
			t.Fatalf("expected error")
		}
	})
}
