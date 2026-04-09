package functions

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
)

func TestAddApp(t *testing.T) {
	t.Run("domain__valid_name__creates_subfolders_directly", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		appPath := filepath.Join("repos", "apps", "alpha")

		if err := WriteWorkspaceConfig(fs, WorkspaceConfig{
			Apps:      []WorkspaceApp{},
			Libraries: []WorkspaceLibrary{},
			Projects:  []WorkspaceProject{},
		}); err != nil {
			t.Fatalf("write workspace config: %v", err)
		}

		if err := AddApp(fs, "alpha"); err != nil {
			t.Fatalf("AddApp: %v", err)
		}

		// Apps are not template-able — AddApp must mkdir each subfolder
		// in code, not copy from repos/templates/apps/.
		for _, sub := range []string{"services", "libs", "jobs", "docs", "testing"} {
			subPath := filepath.Join(appPath, sub)
			info, err := fs.Stat(subPath)
			if err != nil {
				t.Fatalf("expected subfolder %s: %v", sub, err)
			}
			if !info.IsDir() {
				t.Fatalf("expected %s to be a directory", sub)
			}
			if _, err := fs.Stat(filepath.Join(subPath, ".gitkeep")); err != nil {
				t.Fatalf("expected .gitkeep in %s: %v", sub, err)
			}
		}

		if _, err := fs.Stat(filepath.Join(appPath, "ocean.config.json")); err != nil {
			t.Fatalf("expected ocean.config.json: %v", err)
		}
	})

	t.Run("domain__valid_name__registers_workspace_app", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := WriteWorkspaceConfig(fs, WorkspaceConfig{
			Apps:      []WorkspaceApp{},
			Libraries: []WorkspaceLibrary{},
			Projects:  []WorkspaceProject{},
		}); err != nil {
			t.Fatalf("write workspace config: %v", err)
		}

		if err := AddApp(fs, "alpha"); err != nil {
			t.Fatalf("AddApp: %v", err)
		}

		config, err := ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("read workspace config: %v", err)
		}
		if len(config.Apps) != 1 || config.Apps[0].Name != "alpha" {
			t.Fatalf("expected workspace app entry added")
		}
		if len(config.Apps[0].Services) != 0 || len(config.Apps[0].Libraries) != 0 {
			t.Fatalf("expected empty services and libraries")
		}
	})

	t.Run("boundary__empty_name__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := AddApp(fs, ""); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("complement__existing_app__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		appPath := filepath.Join("repos", "apps", "alpha")

		if err := fs.MkdirAll(appPath, 0o755); err != nil {
			t.Fatalf("mkdir app: %v", err)
		}

		if err := AddApp(fs, "alpha"); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("complement__no_app_template_tree_required", func(t *testing.T) {
		// AddApp must succeed on a freshly initialized workspace that has
		// NO repos/templates/apps/ tree. This locks in the "apps are not
		// template-able" decision.
		fs := afero.NewMemMapFs()
		if err := WriteWorkspaceConfig(fs, WorkspaceConfig{
			Apps:      []WorkspaceApp{},
			Libraries: []WorkspaceLibrary{},
			Projects:  []WorkspaceProject{},
		}); err != nil {
			t.Fatalf("write workspace config: %v", err)
		}
		if err := AddApp(fs, "alpha"); err != nil {
			t.Fatalf("AddApp must not require a template tree: %v", err)
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
